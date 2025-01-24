// Copyright 2025 Upbound Inc.
// All rights reserved

// Package pullsecret contains functions for create or update pull secrets
package pullsecret

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/upbound"
)

const (
	defaultUsername        = "_token"
	errMissingProfileCreds = "current profile does not contain credentials"
	errCreatePullSecret    = "failed to create package pull secret"
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *createCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	if _, ctp, isSpace := upCtx.GetCurrentSpaceContextScope(); isSpace && ctp.Name == "" {
		return errors.New("no control plane context is defined. Use 'up ctx' to set an control plane inside a group context")
	}

	cl, err := upCtx.BuildCurrentContextClient()
	if err != nil {
		return errors.Wrap(err, "unable to get kube client")
	}
	kongCtx.BindTo(cl, (*client.Client)(nil))

	if c.File != "" {
		tf, err := upbound.TokenFromPath(c.File)
		if err != nil {
			return err
		}
		c.user, c.pass = tf.AccessID, tf.Token
	}
	if c.user == "" || c.pass == "" {
		if upCtx.Profile.Session == "" {
			return errors.New(errMissingProfileCreds)
		}
		c.user, c.pass = defaultUsername, upCtx.Profile.Session
		pterm.Warning.WithWriter(kongCtx.Stdout).Printfln("Using temporary user credentials that will expire within 30 days.")
	}
	return nil
}

// createCmd creates a package pull secret.
type createCmd struct {
	user string
	pass string

	Name string `arg:"" help:"Name of the pull secret."`

	// NOTE(hasheddan): kong automatically cleans paths tagged with existingfile.
	File      string `help:"Path to credentials file. Credentials from profile are used if not specified." short:"f"               type:"existingfile"`
	Namespace string `default:"crossplane-system"                                                          env:"UPBOUND_NAMESPACE" help:"Kubernetes namespace for pull secret." short:"n"`
}

// Run executes the pull secret command.
func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context, cl client.Client) error {
	// Construct the pull-secret secret object
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Namespace,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": []byte(fmt.Sprintf(`{
				"auths": {
					"%s": {
						"username": "%s",
						"password": "%s",
						"auth": "%s"
					}
				}
			}`, upCtx.RegistryEndpoint.Hostname(), c.user, c.pass,
				base64.StdEncoding.EncodeToString([]byte(c.user+":"+c.pass)))),
		},
	}

	// Apply the Secret using the client
	if err := cl.Patch(ctx, secret, client.Apply, client.ForceOwnership, client.FieldOwner("up-ctp-pull-secret")); err != nil {
		return errors.Wrap(err, "failed to create or update pull secret")
	}

	p.Printfln("Secret %s/%s created or updated", c.Namespace, c.Name)
	return nil
}
