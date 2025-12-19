// Copyright 2025 Upbound Inc.
// All rights reserved

package migration

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/pkg/migration"
	"github.com/upbound/up/pkg/migration/category"

	_ "embed"
)

const (
	alreadyPausedAnnotation = "migration.upbound.io/already-paused"
	pausedAnnotation        = "crossplane.io/paused"
)

type pauseToggleCmd struct {
	prompter input.Prompter

	Pause bool `default:"false" help:"Set to 'true' to pause all resources in the target control plane after a faulty migration, or 'false' to remove the paused annotation in the source control plane after a failed migration."`
	Yes   bool `default:"false" help:"When set to true, automatically accepts any confirmation prompts that may appear during the process."`
}

//go:embed help/pause-toggle.md
var pauseToggleHelp string

func (c *pauseToggleCmd) Help() string {
	return pauseToggleHelp
}

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *pauseToggleCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

func (c *pauseToggleCmd) Run(ctx context.Context, migCtx *migration.Context, p upterm.Printer) error {
	// Determine action
	action, operationFunc := c.getActionAndFunc()

	p.Printfln("%s resources...", action)

	if !c.Yes {
		result, _ := upterm.Confirm("Do you still want to proceed?", false)
		if !result {
			return nil
		}
	}

	// Start scanning spinner
	scanMsg := "Scanning control plane for types... "
	s := upterm.NewSuccessSpinner(scanMsg)
	s.Start()
	cfg := migCtx.Kubeconfig

	// Create Kubernetes clients
	dynamicClient, discoveryClient, err := createKubeClients(cfg)
	if err != nil {
		s.UpdateText("Failed to initialize Kubernetes clients")
		s.Fail()
		return err
	}
	s.UpdateText("Control plane scan completed 👀")
	s.Success()

	// Define categories to be modified
	categories := []string{"composite", "claim", "managed"}
	cm := category.NewAPICategoryModifier(dynamicClient, discoveryClient)

	// Process each category separately
	for _, category := range categories {
		categoryMsg := fmt.Sprintf("%s %s resources...", action, category)
		sp := upterm.NewSuccessSpinner(categoryMsg)
		sp.Start()

		count, err := operationFunc(ctx, category, cm)
		if err != nil {
			sp.UpdateText(fmt.Sprintf("Failed to %s %s resources ❌", strings.ToLower(action), category))
			sp.Fail()
			return fmt.Errorf("failed to %s %s resources: %w", strings.ToLower(action), category, err)
		}

		sp.UpdateText(fmt.Sprintf("%d %s resources %sd! ✅", count, category, strings.ToLower(action)))
		sp.Success()
	}

	p.Println()
	p.Printfln("All relevant resources successfully %sd!", strings.ToLower(action))

	return nil
}

// createKubeClients initializes Kubernetes clients.
func createKubeClients(cfg *rest.Config) (dynamic.Interface, *discovery.DiscoveryClient, error) {
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	return dynamicClient, discoveryClient, nil
}

// getActionAndFunc determines the action and corresponding function.
func (c *pauseToggleCmd) getActionAndFunc() (string, func(ctx context.Context, resourceType string, cm *category.APICategoryModifier) (int, error)) {
	if c.Pause {
		return "Pause", pauseResources
	}
	return "Unpause", unpauseResources
}

// unpauseResources removes pause and alreadyPause annotations if they exist.
func unpauseResources(ctx context.Context, resourceType string, cm *category.APICategoryModifier) (int, error) {
	count, err := cm.ModifyResources(ctx, resourceType, func(u *unstructured.Unstructured) error {
		annotations := u.GetAnnotations()
		if annotations == nil {
			return nil
		}

		if alreadyPaused, exists := annotations[alreadyPausedAnnotation]; !exists || alreadyPaused == "false" {
			meta.RemoveAnnotations(u, pausedAnnotation)
		}
		return nil
	})
	return count, err
}

// pauseResources adds pause annotations.
func pauseResources(ctx context.Context, resourceType string, cm *category.APICategoryModifier) (int, error) {
	count, err := cm.ModifyResources(ctx, resourceType, func(u *unstructured.Unstructured) error {
		annotations := u.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}

		if _, exists := annotations[pausedAnnotation]; exists {
			annotations[alreadyPausedAnnotation] = "true"
		} else {
			annotations[pausedAnnotation] = "true"
		}

		u.SetAnnotations(annotations)
		return nil
	})
	return count, err
}
