// Copyright 2025 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package migration

import (
	"context"
	"fmt"
	"strings"

	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/crossplane/crossplane-runtime/pkg/meta"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/pkg/migration"
	"github.com/upbound/up/pkg/migration/category"
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

func (c *pauseToggleCmd) Help() string {
	return `
The 'pause-toggle' command allows you to manage the paused state of resources after a migration attempt.

- When --pause=true, all resources in the **target control plane** will be paused due to a faulty migration. This is useful after running 'migration import --unpause-after-import=true' and discovering issues in the target.
- When --pause=false, only resources paused during the migration will be **unpaused in the source control plane**, ensuring that pre-existing paused resources remain unchanged.

Use Cases:
    migration pause-toggle --pause=true
        Pauses all resources in the **target control plane** after a migration if the import caused issues.
        Useful for stopping resources in a faulty target environment.

    migration pause-toggle --pause=false
        Unpauses only the resources that were paused in the **source control plane** due to migration.
        This is helpful when reverting migration-induced pauses in the source after a failed import to the target.
`
}

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *pauseToggleCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

func (c *pauseToggleCmd) Run(ctx context.Context, migCtx *migration.Context) error {
	// Determine action
	action, operationFunc := c.getActionAndFunc()

	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	pterm.Printfln("%s resources...", action)
	migration.DefaultSpinner = &spinner{upterm.CheckmarkSuccessSpinner}

	if !c.Yes {
		pterm.Println() // Blank line
		confirm := pterm.DefaultInteractiveConfirm
		confirm.DefaultText = "Do you still want to proceed?"
		confirm.DefaultValue = false
		result, _ := confirm.Show()
		pterm.Println() // Blank line
		if !result {
			return nil
		}
	}

	// Start scanning spinner
	scanMsg := "Scanning control plane for types... "
	s, _ := migration.DefaultSpinner.Start(scanMsg)
	cfg := migCtx.Kubeconfig

	// Create Kubernetes clients
	dynamicClient, discoveryClient, err := createKubeClients(cfg)
	if err != nil {
		s.Fail("Failed to initialize Kubernetes clients")
		return err
	}
	s.Success("Control plane scan completed 👀")

	// Define categories to be modified
	categories := []string{"composite", "claim", "managed"}
	cm := category.NewAPICategoryModifier(dynamicClient, discoveryClient)

	// Process each category separately
	for _, category := range categories {
		categoryMsg := fmt.Sprintf("%s %s resources...", action, category)
		categorySpinner, _ := migration.DefaultSpinner.Start(categoryMsg)

		count, err := operationFunc(ctx, category, cm)
		if err != nil {
			categorySpinner.Fail(fmt.Sprintf("Failed to %s %s resources ❌", strings.ToLower(action), category))
			return fmt.Errorf("failed to %s %s resources: %w", strings.ToLower(action), category, err)
		}

		categorySpinner.Success(fmt.Sprintf("%d %s resources %sd! ✅", count, category, strings.ToLower(action)))
	}

	pterm.Println() // Blank line
	pterm.Printfln("All relevant resources successfully %sd!", strings.ToLower(action))
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
