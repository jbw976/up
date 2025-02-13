// Copyright 2025 Upbound Inc.
// All rights reserved

package model

import (
	"time"
)

// MXPGVKEvent records an event associated with an MXP and k8s GVK.
type MXPGVKEvent struct {
	Name         string          `json:"name"`
	Tags         MXPGVKEventTags `json:"tags"`
	Timestamp    time.Time       `json:"timestamp"`
	TimestampEnd time.Time       `json:"timestamp_end"`
	Value        float64         `json:"value"`
}

type MXPGVKEventTags struct {
	Group          string `json:"customresource_group"`
	Version        string `json:"customresource_version"`
	Kind           string `json:"customresource_kind"`
	UpboundAccount string `json:"upbound_account"`
	MXPID          string `json:"mxp_id"`
}
