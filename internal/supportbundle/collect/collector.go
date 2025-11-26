// Copyright 2025 Upbound Inc.
// All rights reserved

// Package collect provides support bundle collection functionality.
package collect

import (
	"context"
	"fmt"
	"sync"

	"github.com/replicatedhq/troubleshoot/pkg/supportbundle"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/supportbundle/processor"
)

// Collector performs support bundle collection operations.
type Collector struct{}

// NewCollector returns a new Collector.
func NewCollector() *Collector {
	return &Collector{}
}

// Collect performs support bundle collection with the given options.
func (c *Collector) Collect(ctx context.Context, opts Options) (*supportbundle.SupportBundleResponse, error) {
	if opts.Spec == nil {
		return nil, errors.New("spec is required")
	}
	if opts.RestConfig == nil {
		return nil, errors.New("rest config is required")
	}
	if opts.OutputPath == "" {
		return nil, errors.New("output path is required")
	}

	// Create a progress channel and consume it to prevent blocking
	progressChan := make(chan any)
	var wg sync.WaitGroup

	defer func() {
		close(progressChan)
		wg.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for msg := range progressChan {
			if opts.ProgressCallback == nil {
				continue
			}

			var message string
			switch m := msg.(type) {
			case string:
				message = m
			case error:
				message = fmt.Sprintf("Error: %v", m)
			default:
				message = fmt.Sprintf("%v", m)
			}

			opts.ProgressCallback(message)
		}
	}()

	collectorCB := func(c chan any, msg string) { c <- msg }

	supportBundleOpts := supportbundle.SupportBundleCreateOpts{
		CollectorProgressCallback: collectorCB,
		KubernetesRestConfig:      opts.RestConfig,
		Redact:                    true,
		OutputPath:                opts.OutputPath,
		FromCLI:                   false,
		ProgressChan:              progressChan,
		CollectWithoutPermissions: true,
	}

	response, err := supportbundle.CollectSupportBundleFromSpec(opts.Spec, opts.AdditionalRedactors, supportBundleOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to collect support bundle")
	}

	if len(opts.Processors) > 0 {
		if err := processor.Apply(ctx, response.ArchivePath, opts.Processors...); err != nil {
			return nil, errors.Wrap(err, "failed to post-process bundle")
		}
	}

	return response, nil
}
