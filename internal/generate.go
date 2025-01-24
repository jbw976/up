// Copyright 2025 Upbound Inc.
// All rights reserved

//go:build generate
// +build generate

// NOTE(negz): See the below link for details on what is happening here.
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

// Add license headers to all files.
//go:generate go run -tags generate github.com/google/addlicense -v -ignore **/testdata/** -ignore **/vendor/** -ignore ../cmd/up/function/templates/** -ignore ../cmd/up/test/templates/** -f ../hack/boilerplate.txt . ../cmd ../pkg

// Generate deepcopy methodsets and CRD manifests
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=../hack/boilerplate.go.txt paths=../internal/upbound/...
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=../hack/boilerplate.go.txt paths=../pkg/apis/...

package internal

import (
	_ "github.com/google/addlicense" //nolint:typecheck
)
