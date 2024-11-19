// Copyright 2024 Upbound Inc
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

package group

import (
	"context"
	"strconv"

	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

var errDeletionProtectionEnabled = errors.New("Deletion protection is enabled on the specified group. Use '--force' to delete anyway.")

// deleteCmd creates a group in a space.
type deleteCmd struct {
	Name  string `arg:""          help:"Name of group."                   required:""`
	Force bool   `default:"false" help:"Force the deletion of the group." name:"force" optional:""`
}

// Run executes the create command.
func (c *deleteCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, upCtx *upbound.Context, client client.Client, p pterm.TextPrinter) error { //nolint:gocyclo
	// delete group
	group := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.Name,
		},
	}

	// ensure deletion protection is disabled, if not forcing
	if !c.Force {
		if err := client.Get(ctx, types.NamespacedName{Name: c.Name}, &group); err != nil {
			return err
		}

		key, ok := group.Labels[spacesv1beta1.ControlPlaneGroupProtectionKey]
		if ok {
			if protected, err := strconv.ParseBool(key); err != nil {
				return err
			} else if protected {
				return errDeletionProtectionEnabled
			}
		}
	}

	if err := client.Delete(ctx, &group); err != nil {
		return err
	}

	p.Printfln("%s deleted", c.Name)
	return nil
}
