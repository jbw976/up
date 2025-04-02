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

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/workspace"
	"github.com/upbound/up/internal/xpkg/workspace/meta"
	"github.com/upbound/up/internal/yaml"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// Move updates a project to use a new repository. The project metadata and any
// compositions that reference embedded functions will be updated. The passed
// project and filesystem will be updated in place.
func Move(ctx context.Context, project *v1alpha1.Project, projectFS afero.Fs, newRepository string) error {
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
	metaProj, ok := ws.View().Meta().Object().(*v1alpha1.Project)
	if !ok {
		return errors.New("project has unexpected metadata type")
	}
	metaProj.Spec.Repository = newRepository
	if err := ws.Write(meta.New(metaProj)); err != nil {
		return errors.Wrap(err, "failed to write project metadata")
	}

	if err := updateCompositions(ws, fnMap); err != nil {
		return err
	}

	return nil
}

func buildFunctionMap(project *v1alpha1.Project, projectFS afero.Fs, oldRepository, newRepository string) (map[string]string, error) {
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

func updateCompositions(ws *workspace.Workspace, fnMap map[string]string) error {
	projFS := ws.Filesystem()
	for _, node := range ws.View().Nodes() {
		var comp xpextv1.Composition
		unst, ok := node.GetObject().(*unstructured.Unstructured)
		if !ok {
			return errors.Errorf("unexpected node type %T in workspace", node.GetObject())
		}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(unst.UnstructuredContent(), &comp)
		if err != nil {
			continue
		}

		if comp.Spec.Mode == nil || *comp.Spec.Mode != xpextv1.CompositionModePipeline {
			continue
		}

		newPipeline := make([]xpextv1.PipelineStep, len(comp.Spec.Pipeline))
		rewritten := false
		for i, step := range comp.Spec.Pipeline {
			newRef, update := fnMap[step.FunctionRef.Name]
			if update {
				step.FunctionRef.Name = newRef
				rewritten = true
			}
			newPipeline[i] = step
		}
		comp.Spec.Pipeline = newPipeline

		if !rewritten {
			continue
		}

		fname := node.GetFileName()
		compYAML, err := yaml.Marshal(comp)
		if err != nil {
			return errors.Wrapf(err, "failed to marshal updated composition %q", comp.Name)
		}
		if err := afero.WriteFile(projFS, fname, compYAML, 0o644); err != nil {
			return errors.Wrapf(err, "failed to write updated composition %q", comp.Name)
		}
	}

	return nil
}
