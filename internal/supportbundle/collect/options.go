// Copyright 2025 Upbound Inc.
// All rights reserved

package collect

import (
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/rest"

	"github.com/upbound/up/internal/supportbundle/processor"
)

// ProgressCallback is called with progress messages during collection.
// If nil, progress messages are discarded.
type ProgressCallback func(message string)

// Options configures support bundle collection behavior.
type Options struct {
	// Support bundle spec
	Spec *troubleshootv1beta2.SupportBundleSpec
	// Additional redactors to apply to the support bundle
	AdditionalRedactors *troubleshootv1beta2.Redactor
	// Kubernetes rest config
	RestConfig *rest.Config
	// Output path for the support bundle archive
	OutputPath string
	// Post-processing functions to apply to the support bundle
	Processors []processor.Func
	// Progress callback for reporting collection progress
	ProgressCallback ProgressCallback
}
