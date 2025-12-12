// Copyright 2025 Upbound Inc.
// All rights reserved

package supportbundle

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/docrewrite"
	"github.com/replicatedhq/troubleshoot/pkg/supportbundle"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/supportbundle/collect"
	"github.com/upbound/up/internal/supportbundle/defaults"
	"github.com/upbound/up/internal/supportbundle/processor"
	"github.com/upbound/up/internal/upterm"

	_ "embed"
)

// collectCmd collects support bundles from the current kube context.
type collectCmd struct {
	commonFlags `embed:""`

	Config                  string `help:"Path to a SupportBundle YAML configuration file. If provided, this will be used instead of the default configuration. Redactors can be included in the same file as a separate YAML document (multi-document YAML)." short:"c"`
	Output                  string `help:"Output file path for the support bundle archive. If not specified, a timestamped filename will be used (e.g., upbound-support-bundle-20250105-163905.tar.gz)."                                                       short:"o"`
	CrossplaneResourcesOnly bool   `help:"Collect only Crossplane CRDs and custom resources. When this flag is set, log collectors are excluded and only Crossplane-related resources are included in the bundle."                                             name:"crossplane-resources-only" short:"x"`

	fs afero.Fs
}

//go:embed help/collect.md
var collectHelp string

// Help prints help.
func (c *collectCmd) Help() string {
	return collectHelp
}

// AfterApply initializes the filesystem and validates flags.
func (c *collectCmd) AfterApply() error {
	if c.fs == nil {
		c.fs = afero.NewOsFs()
	}

	if err := validatePatterns(c.IncludeNamespaces); err != nil {
		return errors.Wrap(err, "invalid include-namespaces pattern")
	}

	if err := validatePatterns(c.ExcludeNamespaces); err != nil {
		return errors.Wrap(err, "invalid exclude-namespaces pattern")
	}

	return nil
}

// Run executes the support bundle collection.
func (c *collectCmd) Run(ctx context.Context, p upterm.Printer) error {
	restConfig, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return errors.Wrap(err, "failed to get kubeconfig")
	}

	// suppress deprecation warnings
	restConfig.WarningHandler = rest.NoWarnings{}

	if c.Output == "" {
		timestamp := time.Now().Format("20060102-150405")
		c.Output = fmt.Sprintf("upbound-support-bundle-%s.tar.gz", timestamp)
	}

	var spec *troubleshootv1beta2.SupportBundleSpec
	var additionalRedactors *troubleshootv1beta2.Redactor

	if c.Config != "" {
		spec, additionalRedactors, err = c.loadConfigFromFile(c.Config)
		if err != nil {
			return errors.Wrap(err, "failed to load support bundle config")
		}
		if additionalRedactors == nil {
			additionalRedactors = defaults.Redactors()
		}
	} else {
		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return errors.Wrap(err, "failed to create kubernetes client")
		}

		namespaces := determineNamespaces(ctx, clientset, c.IncludeNamespaces, c.ExcludeNamespaces)

		// Skip collecting logs if we're only collecting Crossplane resources
		includeLogs := !c.CrossplaneResourcesOnly
		spec = defaults.SupportBundleSpec(namespaces, includeLogs)
		additionalRedactors = defaults.Redactors()
	}

	spinner := upterm.NewSuccessSpinner("Collecting support bundle...")
	spinner.Start()

	processors := []processor.Func{
		processor.RedactConfigMaps,
		processor.RedactEnvironmentConfigs,
		processor.RedactProviderKubernetesObjects,
	}

	if c.CrossplaneResourcesOnly {
		processors = append(processors, processor.FilterCrossplaneResources)
	}

	collector := collect.NewCollector()
	opts := collect.Options{
		Spec:                spec,
		AdditionalRedactors: additionalRedactors,
		RestConfig:          restConfig,
		OutputPath:          c.Output,
		Processors:          processors,
		ProgressCallback: func(message string) {
			spinner.Logf("Collecting: %s", message)
		},
	}

	response, err := collector.Collect(ctx, opts)
	if err != nil {
		spinner.Fail()
		return errors.Wrap(err, "failed to collect support bundle")
	}
	spinner.Success()

	p.Println()
	p.Printfln("Support bundle collected successfully: %s", response.ArchivePath)

	return nil
}

// loadConfigFromFile loads a SupportBundle config from a YAML file.
func (c *collectCmd) loadConfigFromFile(configPath string) (*troubleshootv1beta2.SupportBundleSpec, *troubleshootv1beta2.Redactor, error) {
	fileBytes, err := afero.ReadFile(c.fs, filepath.Clean(configPath))
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to read config file %q", configPath)
	}

	supportBundle, err := supportbundle.ParseSupportBundle(fileBytes, false)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to parse support bundle config")
	}

	// try to parse redactors from the same file (multi-document YAML)
	var redactor *troubleshootv1beta2.Redactor
	fileStr := string(fileBytes)
	if strings.Contains(fileStr, "---") {
		parts := strings.Split(fileStr, "\n---\n")
		for i := 1; i < len(parts); i++ {
			redactorObj, ok, err := parseRedactorFromDoc([]byte(parts[i]))
			if err != nil {
				return nil, nil, errors.Wrapf(err, "failed to parse redactor from document %d in file %q", i, configPath)
			}
			if ok {
				if redactor != nil {
					return nil, nil, errors.Errorf("multiple redactor documents found in file %q, only one redactor is supported", configPath)
				}
				redactor = redactorObj
			}
		}
	}

	return &supportBundle.Spec, redactor, nil
}

// parseRedactorFromDoc parses a redactor from a YAML document.
func parseRedactorFromDoc(doc []byte) (*troubleshootv1beta2.Redactor, bool, error) {
	doc, err := docrewrite.ConvertToV1Beta2(doc)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to convert to v1beta2")
	}

	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(doc, nil, nil)
	if err != nil {
		return nil, false, err
	}

	redactor, ok := obj.(*troubleshootv1beta2.Redactor)
	return redactor, ok, nil
}

// determineNamespaces determines the final list of namespaces based on include/exclude flags.
// If includeNamespaces is provided, those are used (supports glob patterns like "upbound-*").
// Otherwise, it collects from crossplane-system, upbound-system, and namespaces labeled with
// internal.spaces.upbound.io/controlplane-name.
// Exclude patterns support glob matching (e.g., "upbound-*" to exclude all namespaces starting with "upbound-").
// This function always returns a valid list of namespaces, falling back to defaults when necessary.
func determineNamespaces(ctx context.Context, clientset kubernetes.Interface, includeNamespaces, excludeNamespaces []string) []string {
	var candidateNamespaces []string

	if len(includeNamespaces) > 0 {
		if clientset == nil {
			// if no client is provided, use includeNamespaces directly
			candidateNamespaces = includeNamespaces
		} else {
			allNamespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
			if err != nil {
				candidateNamespaces = includeNamespaces
			} else {
				candidateNamespaces = matchNamespaces(allNamespaces.Items, includeNamespaces)
			}
		}
	} else {
		candidateNamespaces = []string{"crossplane-system", "upbound-system"}
		if clientset != nil {
			nsList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
				LabelSelector: "internal.spaces.upbound.io/controlplane-name",
			})
			if err == nil {
				// If listing namespaces fails we will continue with the default namespaces only
				for _, ns := range nsList.Items {
					candidateNamespaces = append(candidateNamespaces, ns.Name)
				}
			}
		}
	}

	filtered := []string{}
	for _, ns := range candidateNamespaces {
		if !shouldExcludeNamespace(ns, excludeNamespaces) {
			filtered = append(filtered, ns)
		}
	}

	return filtered
}

// validatePatterns checks if all patterns are valid by attempting to match them.
func validatePatterns(patterns []string) error {
	for _, pattern := range patterns {
		// we use a test namespace name to validate the pattern
		_, err := filepath.Match(pattern, "test")
		if err != nil {
			return errors.Wrapf(err, "pattern %q", pattern)
		}
	}
	return nil
}

// matchNamespaces returns namespaces that match any of the provided patterns.
// Supports both exact matches and glob patterns (e.g., "upbound-*").
func matchNamespaces(namespaces []corev1.Namespace, patterns []string) []string {
	result := []string{}
	for _, ns := range namespaces {
		nsName := ns.Name
		for _, pattern := range patterns {
			if matchesPattern(nsName, pattern) {
				result = append(result, nsName)
				break
			}
		}
	}
	return result
}

// shouldExcludeNamespace checks if a namespace should be excluded based on the exclude patterns.
// Supports both exact matches and glob patterns (e.g., "upbound-*").
func shouldExcludeNamespace(namespace string, excludePatterns []string) bool {
	for _, pattern := range excludePatterns {
		if matchesPattern(namespace, pattern) {
			return true
		}
	}
	return false
}

// matchesPattern checks if a namespace matches a pattern (exact match or glob).
// The pattern is assumed to be valid (validated by validatePatterns).
func matchesPattern(namespace, pattern string) bool {
	if namespace == pattern {
		return true
	}
	matched, err := filepath.Match(pattern, namespace)
	if err != nil {
		return false
	}
	return matched
}
