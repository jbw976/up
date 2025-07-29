// Copyright 2025 Upbound Inc.
// All rights reserved

package project

import (
	"context"
	"fmt"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	opsv1alpha1 "github.com/crossplane/crossplane/apis/ops/v1alpha1"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/workspace"
	"github.com/upbound/up/internal/xpkg/workspace/meta"
	"github.com/upbound/up/internal/yaml"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

// Move updates a project to use a new repository. The project metadata and any
// compositions that reference embedded functions will be updated. The passed
// project and filesystem will be updated in place.
func Move(ctx context.Context, project *v2alpha1.Project, projectFS afero.Fs, newRepository string) error {
	oldRepository := project.Spec.Repository
	fnMap, err := buildFunctionMap(project, projectFS, oldRepository, newRepository)
	if err != nil {
		return err
	}

	project.Spec.Repository = newRepository

	ws, err := workspace.New("/",
		workspace.WithFS(projectFS),
		workspace.WithPrinter(&pterm.BasicTextPrinter{Writer: io.Discard}),
		workspace.WithPermissiveParser(),
	)
	if err != nil {
		return errors.Wrap(err, "failed to construct project workspace")
	}
	if err := ws.Parse(ctx); err != nil {
		return errors.Wrap(err, "failed to parse project workspace")
	}

	// Update the repository in the project metadata. We do this instead of
	// writing out the parsed project because we don't want to write out
	// defaults we've applied during parsing.
	metaProj := ws.View().Meta().Object()
	switch proj := metaProj.(type) {
	case *v1alpha1.Project:
		proj.Spec.Repository = newRepository
	case *v2alpha1.Project:
		proj.Spec.Repository = newRepository
	default:
		return errors.Errorf("project has unexpected metadata type: %T", metaProj)
	}
	if err := ws.Write(meta.New(metaProj)); err != nil {
		return errors.Wrap(err, "failed to write project metadata")
	}

	if err := updatePipelines(ws, fnMap); err != nil {
		return err
	}

	return nil
}

func buildFunctionMap(project *v2alpha1.Project, projectFS afero.Fs, oldRepository, newRepository string) (map[string]string, error) {
	infos, err := afero.ReadDir(projectFS, project.Spec.Paths.Functions)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list functions")
	}
	fnMap := make(map[string]string)
	for _, info := range infos {
		if info.IsDir() {
			oldRepoStr := fmt.Sprintf("%s_%s", oldRepository, info.Name())
			oldRepo, err := name.NewRepository(oldRepoStr, name.StrictValidation)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse old function repo")
			}
			oldName := xpkg.ToDNSLabel(oldRepo.RepositoryStr())
			newRepoStr := fmt.Sprintf("%s_%s", newRepository, info.Name())
			newRepo, err := name.NewRepository(newRepoStr, name.StrictValidation)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse new function repo")
			}
			newName := xpkg.ToDNSLabel(newRepo.RepositoryStr())

			fnMap[oldName] = newName
		}
	}

	return fnMap, nil
}

func updatePipelines(ws *workspace.Workspace, fnMap map[string]string) error {
	projFS := ws.Filesystem()
	for _, node := range ws.View().Nodes() {
		fname := node.GetFileName()

		unst, ok := node.GetObject().(*unstructured.Unstructured)
		if !ok {
			return errors.Errorf("unexpected node type %T in file %q", node.GetObject(), fname)
		}

		newPipeline, updated, err := updatePipeline(unst, fnMap)
		if err != nil {
			return errors.Wrapf(err, "failed to convert pipeline %q", fname)
		}
		if !updated {
			continue
		}

		newYAML, err := yaml.Marshal(newPipeline,
			yaml.RemoveField("status"),
			yaml.RemoveField("metadata.creationTimestamp"),
			yaml.RemoveField("spec.operationTemplate.metadata"),
		)
		if err != nil {
			return errors.Wrapf(err, "failed to marshal updated pipeline %q", fname)
		}
		if err := afero.WriteFile(projFS, fname, newYAML, 0o644); err != nil {
			return errors.Wrapf(err, "failed to write updated pipeline %q", fname)
		}
	}

	return nil
}

func updatePipeline(u *unstructured.Unstructured, fnMap map[string]string) (runtime.Object, bool, error) {
	switch u.GroupVersionKind().String() {
	case xpextv1.CompositionGroupVersionKind.String():
		c := new(xpextv1.Composition)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, c); err != nil {
			return nil, false, err
		}

		newPipeline := make([]xpextv1.PipelineStep, len(c.Spec.Pipeline))
		updated := false
		for i, step := range c.Spec.Pipeline {
			newRef, update := fnMap[step.FunctionRef.Name]
			if update {
				step.FunctionRef.Name = newRef
				updated = true
			}
			newPipeline[i] = step
		}
		c.Spec.Pipeline = newPipeline

		return c, updated, nil

	case opsv1alpha1.OperationGroupVersionKind.String():
		o := new(opsv1alpha1.Operation)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, o); err != nil {
			return nil, false, err
		}

		newPipeline, updated := updateOperationPipeline(o.Spec.Pipeline, fnMap)
		o.Spec.Pipeline = newPipeline

		return o, updated, nil

	case opsv1alpha1.CronOperationGroupVersionKind.String():
		o := new(opsv1alpha1.CronOperation)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, o); err != nil {
			return nil, false, err
		}

		newPipeline, updated := updateOperationPipeline(o.Spec.OperationTemplate.Spec.Pipeline, fnMap)
		o.Spec.OperationTemplate.Spec.Pipeline = newPipeline

		return o, updated, nil

	case opsv1alpha1.WatchOperationGroupVersionKind.String():
		o := new(opsv1alpha1.WatchOperation)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, o); err != nil {
			return nil, false, err
		}

		newPipeline, updated := updateOperationPipeline(o.Spec.OperationTemplate.Spec.Pipeline, fnMap)
		o.Spec.OperationTemplate.Spec.Pipeline = newPipeline

		return o, updated, nil

	default:
		// Not a pipeline type, this is fine.
		return nil, false, nil
	}
}

func updateOperationPipeline(p []opsv1alpha1.PipelineStep, fnMap map[string]string) ([]opsv1alpha1.PipelineStep, bool) {
	newPipeline := make([]opsv1alpha1.PipelineStep, len(p))
	updated := false
	for i, step := range p {
		newRef, update := fnMap[step.FunctionRef.Name]
		if update {
			step.FunctionRef.Name = newRef
			updated = true
		}
		newPipeline[i] = step
	}

	return newPipeline, updated
}
