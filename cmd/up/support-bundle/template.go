// Copyright 2025 Upbound Inc.
// All rights reserved

package supportbundle

import (
	"context"
	"fmt"
	"io"
	"os"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/supportbundle/defaults"

	_ "embed"
)

// templateCmd outputs the default SupportBundle configuration.
type templateCmd struct {
	commonFlags `embed:""`

	// out is the writer to output the template to. Defaults to os.Stdout.
	out io.Writer

	// kClient is an Kubernetes client interface
	// If nil, a client will be created from the kubeconfig.
	kClient kubernetes.Interface
}

//go:embed help/template.md
var templateHelp string

// Help prints help.
func (c *templateCmd) Help() string {
	return templateHelp
}

// AfterApply initializes the output + validates the include and exclude namespaces patterns.
// It also attempts to create a Kubernetes client from the kubeconfig if available.
func (c *templateCmd) AfterApply() error {
	if c.out == nil {
		c.out = os.Stdout
	}
	if err := validatePatterns(c.IncludeNamespaces); err != nil {
		return errors.Wrap(err, "invalid include-namespaces pattern")
	}

	if err := validatePatterns(c.ExcludeNamespaces); err != nil {
		return errors.Wrap(err, "invalid exclude-namespaces pattern")
	}

	// Try to create a Kubernetes client from kubeconfig if available.
	if c.kClient == nil {
		restConfig, err := kube.GetKubeConfig(c.Kubeconfig)
		if err == nil {
			// ignore error if creation fails
			c.kClient, _ = kubernetes.NewForConfig(restConfig)
		}
	}

	return nil
}

// Run outputs the default SupportBundle YAML configuration.
func (c *templateCmd) Run(ctx context.Context) error {
	namespaces := determineNamespaces(ctx, c.kClient, c.IncludeNamespaces, c.ExcludeNamespaces)
	return writeTemplate(c.out, namespaces)
}

// writeTemplate writes the SupportBundle and Redactor templates to the given writer.
func writeTemplate(w io.Writer, namespaces []string) error {
	spec := defaults.SupportBundleSpec(namespaces, true)

	supportBundle := troubleshootv1beta2.SupportBundle{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "troubleshoot.sh/v1beta2",
			Kind:       "SupportBundle",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "support-bundle",
		},
		Spec: *spec,
	}

	yamlBytes, err := yaml.Marshal(supportBundle)
	if err != nil {
		return errors.Wrap(err, "failed to marshal support bundle to YAML")
	}

	if _, err := w.Write(yamlBytes); err != nil {
		return errors.Wrap(err, "failed to write support bundle to output")
	}

	if _, err := fmt.Fprintln(w, "---"); err != nil {
		return errors.Wrap(err, "failed to write separator to output")
	}

	defaultRedactor := defaults.Redactors()
	redactor := troubleshootv1beta2.Redactor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "troubleshoot.sh/v1beta2",
			Kind:       "Redactor",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-redactors",
		},
		Spec: defaultRedactor.Spec,
	}

	redactorYAML, err := yaml.Marshal(redactor)
	if err != nil {
		return errors.Wrap(err, "failed to marshal redactor to YAML")
	}

	if _, err := w.Write(redactorYAML); err != nil {
		return errors.Wrap(err, "failed to write redactor to output")
	}

	return nil
}
