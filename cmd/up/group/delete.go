// Copyright 2025 Upbound Inc.
// All rights reserved

package group

import (
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upterm"
)

// deleteCmd creates a group in a space.
type deleteCmd struct {
	Name  string `arg:""          help:"Name of group."                   required:""`
	Force bool   `default:"false" help:"Force the deletion of the group." name:"force" optional:""`
}

// Run executes the create command.
func (c *deleteCmd) Run(ctx context.Context, printer upterm.Printer, client client.Client) error {
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
				return errors.New("deletion protection is enabled on the specified group; use '--force' to delete anyway")
			}
		}
	}

	if err := client.Delete(ctx, &group); err != nil {
		return err
	}

	printer.Printfln("%s deleted", c.Name)
	return nil
}
