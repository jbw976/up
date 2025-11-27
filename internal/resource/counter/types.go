// Copyright 2025 Upbound Inc.
// All rights reserved

package counter

// ResourceCounts holds the counts of different Crossplane resource types.
type ResourceCounts struct {
	TotalResources          int `json:"totalResources"`
	ManagedResources        int `json:"managedResources"`
	CompositeResources      int `json:"compositeResources"`
	CompositeResourceClaims int `json:"compositeResourceClaims"`
	ComposedResources       int `json:"composedResources"`
}

// resourceType represents the classification of a CRD.
type resourceType int

const (
	resourceTypeExcluded resourceType = iota
	resourceTypeManagedResource
	resourceTypeComposite
	resourceTypeClaim
)
