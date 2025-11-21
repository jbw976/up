// Copyright 2025 Upbound Inc.
// All rights reserved

package supportbundle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/docrewrite"
	"github.com/replicatedhq/troubleshoot/pkg/supportbundle"
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

	Config                  string `help:"Path to a SupportBundle YAML configuration file. If provided, this will be used instead of the default configuration." short:"c"`
	Output                  string `help:"Output file path for the support bundle archive. If not specified, a timestamped filename will be used."               short:"o"`
	CrossplaneResourcesOnly bool   `help:"Collect only Crossplane CRDs and custom resources (resources with composites, crossplane, or managed categories)."     name:"crossplane-resources-only" short:"x"`
}

//go:embed help/collect.md
var collectHelp string

// Help prints help.
func (c *collectCmd) Help() string {
	return collectHelp
}

// Run executes the support bundle collection.
func (c *collectCmd) Run(ctx context.Context) error {
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
		namespaces, err := determineNamespaces(ctx, restConfig, c.IncludeNamespaces, c.ExcludeNamespaces)
		if err != nil {
			return errors.Wrap(err, "failed to determine namespaces for collection")
		}

		// Skip collecting logs if we're only collecting Crossplane resources
		includeLogs := !c.CrossplaneResourcesOnly
		spec = defaults.SupportBundleSpec(namespaces, includeLogs)
		additionalRedactors = defaults.Redactors()
	}

	spinner, err := upterm.CheckmarkSuccessSpinner.Start("Collecting support bundle...")
	if err != nil {
		return errors.Wrap(err, "failed to start spinner")
	}

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
			spinner.UpdateText(fmt.Sprintf("Collecting: %s", message))
		},
	}

	response, err := collector.Collect(ctx, opts)
	if err != nil {
		spinner.Fail("Failed to collect support bundle")
		return errors.Wrap(err, "failed to collect support bundle")
	}

	spinner.Success(fmt.Sprintf("Support bundle collected successfully: %s", response.ArchivePath))
	return nil
}

// loadConfigFromFile loads a SupportBundle config from a YAML file.
func (c *collectCmd) loadConfigFromFile(configPath string) (*troubleshootv1beta2.SupportBundleSpec, *troubleshootv1beta2.Redactor, error) {
	fileBytes, err := os.ReadFile(filepath.Clean(configPath))
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
		if len(parts) > 1 {
			for i := 1; i < len(parts); i++ {
				redactorObj, ok, err := parseRedactorFromDoc([]byte(parts[i]))
				if err != nil {
					return nil, nil, errors.Wrapf(err, "failed to parse redactor from document %d in file %q", i, configPath)
				}
				if ok {
					redactor = redactorObj
					break
				}
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
// If includeNamespaces is provided, those are used. Otherwise, it collects from crossplane-system,
// upbound-system, and namespaces labeled with internal.spaces.upbound.io/controlplane-name.
func determineNamespaces(ctx context.Context, restConfig *rest.Config, includeNamespaces, excludeNamespaces []string) ([]string, error) {
	var candidateNamespaces []string

	if len(includeNamespaces) > 0 {
		candidateNamespaces = includeNamespaces
	} else {
		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create kubernetes client")
		}

		candidateNamespaces = []string{"crossplane-system", "upbound-system"}

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

	excludeMap := make(map[string]bool, len(excludeNamespaces))
	for _, ns := range excludeNamespaces {
		excludeMap[ns] = true
	}

	filtered := []string{}
	for _, ns := range candidateNamespaces {
		if !excludeMap[ns] {
			filtered = append(filtered, ns)
		}
	}

	return filtered, nil
}
