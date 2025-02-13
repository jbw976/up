// Copyright 2025 Upbound Inc.
// All rights reserved

package importer

import (
	"io/fs"
	"os"
	"regexp"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/upbound/up/pkg/migration/meta/v1alpha1"
)

const yamlPathPattern = `^(cluster|namespaces\/[a-z0-9]([-a-z0-9]*[a-z0-9])?)\/[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\.yaml$`

var (
	yamlPathRegex = regexp.MustCompile(yamlPathPattern)
)

type ResourceReader interface {
	ReadResources(groupResource string) (resources []unstructured.Unstructured, meta *v1alpha1.TypeMeta, err error)
}

type FileSystemReader struct {
	fs afero.Afero
}

// Directory structure:
// <groupResource>/<cluster or namespace>/<?namespace>/<name>.yaml
// <groupResource>/metadata.yaml

func NewFileSystemReader(fs afero.Afero) *FileSystemReader {
	return &FileSystemReader{
		fs: fs,
	}
}

func (g *FileSystemReader) ReadResources(groupResource string) (resources []unstructured.Unstructured, meta *v1alpha1.TypeMeta, rErr error) {
	rErr = g.fs.Walk(groupResource, func(path string, info fs.FileInfo, _ error) error {
		if info == nil || info.IsDir() {
			return nil
		}

		groupPath := strings.TrimPrefix(path, groupResource+string(os.PathSeparator))
		if groupPath == "metadata.yaml" {
			b, err := g.fs.ReadFile(path)
			if err != nil {
				return errors.Wrapf(err, "cannot read file %q", path)
			}
			meta = &v1alpha1.TypeMeta{}
			if err := yaml.Unmarshal(b, meta); err != nil {
				return errors.Wrapf(err, "cannot unmarshal metadata file %q", path)
			}
			return nil
		}

		if !yamlPathRegex.MatchString(groupPath) {
			return errors.Errorf("invalid path %q for YAML file, should match regexp %q", groupPath, yamlPathPattern)
		}

		b, err := g.fs.ReadFile(path)
		if err != nil {
			return errors.Wrapf(err, "cannot read file %q", path)
		}

		var r unstructured.Unstructured
		if err := yaml.Unmarshal(b, &r); err != nil {
			return errors.Wrapf(err, "cannot unmarshal file %q", path)
		}

		resources = append(resources, r)
		return nil
	})
	if rErr != nil {
		return nil, nil, errors.Wrapf(rErr, "cannot walk directory for resource group %q", groupResource)
	}

	return resources, meta, nil
}
