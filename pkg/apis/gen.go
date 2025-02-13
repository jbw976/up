// Copyright 2025 Upbound Inc.
// All rights reserved

// Remove existing CRDs
//go:generate rm -rf crds

// Generate deepcopy methodsets and CRD manifests
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen paths=./... crd:allowDangerousTypes=true,crdVersions=v1 output:artifacts:config=crds

// Package apis contains meta apis
package apis
