// Copyright 2025 Upbound Inc.
// All rights reserved

package exporter

import (
	"context"
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/upbound/up/pkg/migration/meta/v1alpha1"
)

type ResourcePersister interface {
	PersistResources(ctx context.Context, groupResource string, resources []unstructured.Unstructured) error
}

type FileSystemPersister struct {
	fs   afero.Afero
	root string

	meta *v1alpha1.TypeMeta
}

func NewFileSystemPersister(fs afero.Afero, root string, m *v1alpha1.TypeMeta) *FileSystemPersister {
	return &FileSystemPersister{
		fs:   fs,
		root: root,
		meta: m,
	}
}

func (p *FileSystemPersister) pathFor(dirs ...string) string {
	dirs = append([]string{p.root}, dirs...)
	return filepath.Join(dirs...)
}

func (p *FileSystemPersister) PersistResources(_ context.Context, groupResource string, resources []unstructured.Unstructured) error { // nolint:gocyclo // This is slightly over the limit, but it's not too bad.
	if len(resources) == 0 {
		return nil
	}

	if err := p.fs.MkdirAll(p.pathFor(groupResource), 0700); err != nil {
		return errors.Wrapf(err, "cannot create directory resource group %q", groupResource)
	}

	if p.meta != nil {
		b, err := yaml.Marshal(&p.meta)
		if err != nil {
			return errors.Wrap(err, "cannot marshal type meta to yaml")
		}

		mf := p.pathFor(groupResource, "metadata.yaml")
		err = p.fs.WriteFile(mf, b, 0600)
		if err != nil {
			return errors.Wrapf(err, "cannot write type metadata to %q", mf)
		}
	}

	for i := range resources {
		fileDirPath := p.pathFor(groupResource, "cluster")
		if resources[i].GetNamespace() != "" {
			fileDirPath = p.pathFor(groupResource, "namespaces", resources[i].GetNamespace())
		}

		if err := p.fs.MkdirAll(fileDirPath, 0700); err != nil {
			return errors.Wrapf(err, "cannot create directory %q for resource %q", groupResource, resources[i].GetName())
		}

		b, err := yaml.Marshal(&resources[i])
		if err != nil {
			return errors.Wrap(err, "cannot marshal resource to yaml")
		}

		f := filepath.Join(fileDirPath, resources[i].GetName()+".yaml")
		err = p.fs.WriteFile(f, b, 0600)
		if err != nil {
			return errors.Wrapf(err, "cannot write resource to %q", f)
		}
	}

	return nil
}
