// Copyright 2025 Upbound Inc.
// All rights reserved

package report

import (
	"context"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/usage/aggregate"
	"github.com/upbound/up/internal/usage/event"
	usagetime "github.com/upbound/up/internal/usage/time"
)

const (
	errReadEvents  = "error reading events"
	errWriteEvents = "error writing events"
)

// Meta contains metadata for a usage report.
type Meta struct {
	UpboundAccount string          `json:"account"`
	TimeRange      usagetime.Range `json:"time_range"`
	CollectedAt    time.Time       `json:"collected_at"`
}

// MaxResourceCountPerGVKPerMXP reads events from i and writes aggregated events
// to w. Events are aggregated across each window of time returned by i. An
// aggregated event records the largest observed count of instances of a GVK on
// an MXP during a window. The order of written events is not stable.
func MaxResourceCountPerGVKPerMXP(ctx context.Context, i event.WindowIterator, w event.Writer) error {
	for i.More() {
		r, window, err := i.Next()
		if err != nil {
			return errors.Wrap(err, errReadEvents)
		}

		ag := &aggregate.MaxResourceCountPerGVKPerMXP{}
		for {
			e, err := r.Read(ctx)
			if errors.Is(err, event.ErrEOF) {
				break
			}
			if err != nil {
				return err
			}
			if err := ag.Add(e); err != nil {
				return err
			}
		}
		if err := r.Close(); err != nil {
			return errors.Wrap(err, errReadEvents)
		}

		for _, e := range ag.UpboundEvents() {
			e.Timestamp = window.Start
			e.TimestampEnd = window.End
			if err := w.Write(e); err != nil {
				return errors.Wrap(err, errWriteEvents)
			}
		}
	}
	return nil
}
