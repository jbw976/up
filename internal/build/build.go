// Copyright 2025 Upbound Inc.
// All rights reserved

//go:build packaging
// +build packaging

package build

// NOTE(hasheddan): See the below link for details on what is happening here.
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

//go:generate go run github.com/goreleaser/nfpm/v2/cmd/nfpm pkg --config $CACHE_DIR/nfpm_up.yaml --packager $PACKAGER --target $OUTPUT_DIR/$PACKAGER/$PLATFORM/up.$PACKAGER

//go:generate go run github.com/goreleaser/nfpm/v2/cmd/nfpm pkg --config $CACHE_DIR/nfpm_docker-credential-up.yaml --packager $PACKAGER --target $OUTPUT_DIR/$PACKAGER/$PLATFORM/docker-credential-up.$PACKAGER

import (
	_ "github.com/goreleaser/nfpm/v2/cmd/nfpm" //nolint:typecheck
	_ "github.com/spf13/cobra/doc"             //nolint:typecheck
)
