// Copyright 2025 Upbound Inc.
// All rights reserved

package xpkg

import (
	"slices"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Image wraps a v1.Image and extends it with ImageMeta.
type Image struct {
	Meta  ImageMeta `json:"meta"`
	Image v1.Image
}

// ImageMeta contains metadata information about the Package Image.
type ImageMeta struct {
	Repo     string `json:"repo"`
	Registry string `json:"registry"`
	Version  string `json:"version"`
	Digest   string `json:"digest"`
}

// AnnotateImage reads in the layers of the given v1.Image and annotates the
// xpkg layers with their corresponding annotations, returning a new v1.Image
// containing the annotation details.
func AnnotateImage(i v1.Image) (v1.Image, error) { //nolint:gocyclo
	cfgFile, err := i.ConfigFile()
	if err != nil {
		return nil, err
	}

	layers, err := i.Layers()
	if err != nil {
		return nil, err
	}

	addendums := make([]mutate.Addendum, 0)

	for _, l := range layers {
		d, err := l.Digest()
		if err != nil {
			return nil, err
		}
		if annotation, ok := cfgFile.Config.Labels[Label(d.String())]; ok {
			addendums = append(addendums, mutate.Addendum{
				Layer: l,
				Annotations: map[string]string{
					AnnotationKey: annotation,
				},
			})
			continue
		}
		addendums = append(addendums, mutate.Addendum{
			Layer: l,
		})
	}

	// we didn't find any annotations, return original image
	if len(addendums) == 0 {
		return i, nil
	}

	img := empty.Image
	for _, a := range addendums {
		img, err = mutate.Append(img, a)
		if err != nil {
			return nil, errors.Wrap(err, "failed to build annotated image")
		}
	}

	return mutate.ConfigFile(img, cfgFile)
}

// BuildIndex applies annotations to each of the given images and then generates
// an index for them. The annotated images are returned so that a caller can
// push them before pushing the index, since the passed images may not match the
// annotated images.
func BuildIndex(imgs ...v1.Image) (v1.ImageIndex, []v1.Image, error) {
	adds := make([]mutate.IndexAddendum, 0, len(imgs))
	images := make([]v1.Image, 0, len(imgs))
	for _, img := range imgs {
		aimg, err := AnnotateImage(img)
		if err != nil {
			return nil, nil, err
		}
		images = append(images, aimg)
		mt, err := aimg.MediaType()
		if err != nil {
			return nil, nil, err
		}

		conf, err := aimg.ConfigFile()
		if err != nil {
			return nil, nil, err
		}

		adds = append(adds, mutate.IndexAddendum{
			Add: aimg,
			Descriptor: v1.Descriptor{
				MediaType: mt,
				Platform: &v1.Platform{
					Architecture: conf.Architecture,
					OS:           conf.OS,
					OSVersion:    conf.OSVersion,
				},
			},
		})
	}

	// Sort the addendums so that the resulting index will always be the same
	// when the same images are passed in, regardless of their order.
	var sortErr error
	slices.SortFunc(adds, func(a, b mutate.IndexAddendum) int {
		dgstA, errA := a.Add.Digest()
		dgstB, errB := b.Add.Digest()
		sortErr = errors.Join(errA, errB)
		return strings.Compare(dgstA.String(), dgstB.String())
	})
	if sortErr != nil {
		return nil, nil, sortErr
	}

	return mutate.AppendManifests(empty.Index, adds...), images, nil
}
