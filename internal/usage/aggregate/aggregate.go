// Copyright 2025 Upbound Inc.
// All rights reserved

package aggregate

import (
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/usage/model"
)

const (
	mrCountUpboundEventName    = "kube_managedresource_uid"
	mrCountMaxUpboundEventName = "max_resource_count_per_gvk_per_mxp"
)

type mxpGVK struct {
	MXPID   string
	Group   string
	Version string
	Kind    string
}

// MaxResourceCountPerGVKPerMXP aggregates the maximum recorded GVK counts per MXP from
// Upbound usage events.
type MaxResourceCountPerGVKPerMXP struct {
	counts map[mxpGVK]int
}

// Add adds a usage event to the aggregate.
func (ag *MaxResourceCountPerGVKPerMXP) Add(e model.MXPGVKEvent) error {
	if err := ag.validateEvent(e); err != nil {
		return err
	}

	value := int(e.Value)
	key := mxpGVK{
		MXPID:   e.Tags.MXPID,
		Group:   e.Tags.Group,
		Version: e.Tags.Version,
		Kind:    e.Tags.Kind,
	}

	if ag.counts == nil {
		ag.counts = make(map[mxpGVK]int)
	}
	if value > ag.counts[key] {
		ag.counts[key] = value
	}

	return nil
}

// UpboundEvents returns an Upbound usage event for each combination of MXP and
// GVK.
func (ag *MaxResourceCountPerGVKPerMXP) UpboundEvents() []model.MXPGVKEvent {
	events := []model.MXPGVKEvent{}
	for key, count := range ag.counts {
		events = append(events, model.MXPGVKEvent{
			Name:  mrCountMaxUpboundEventName,
			Value: float64(count),
			Tags: model.MXPGVKEventTags{
				MXPID:   key.MXPID,
				Group:   key.Group,
				Version: key.Version,
				Kind:    key.Kind,
			},
		})
	}
	return events
}

func (ag *MaxResourceCountPerGVKPerMXP) validateEvent(e model.MXPGVKEvent) error {
	if e.Name != mrCountUpboundEventName {
		return fmt.Errorf("expected event name %s, got %s", mrCountUpboundEventName, e.Name)
	}
	if e.Tags.MXPID == "" {
		return errors.New("MXPID tag is empty")
	}
	if e.Tags.Group == "" {
		return errors.New("Group tag is empty")
	}
	if e.Tags.Version == "" {
		return errors.New("Version tag is empty")
	}
	if e.Tags.Kind == "" {
		return errors.New("Kind tag is empty")
	}
	return nil
}
