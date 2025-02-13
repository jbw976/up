// Copyright 2025 Upbound Inc.
// All rights reserved

//go:build integration
// +build integration

package functions

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/afero"
	"github.com/spf13/afero/tarfs"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/upbound/up/internal/filesystem"

	_ "embed"
)

// The go function contains a go.mod, so we can't embed it as an embed.FS. We
// have to include it as a tar file and then extract it into a temporary
// directory for the test.
//
//go:embed testdata/go-function.tar
var goFunction []byte

func TestGoBuild(t *testing.T) {
	t.Parallel()

	// Start a test registry to serve the base image.
	regSrv, err := registry.TLS("localhost")
	assert.NilError(t, err)
	t.Cleanup(regSrv.Close)
	testRegistry, err := name.NewRegistry(strings.TrimPrefix(regSrv.URL, "https://"))
	assert.NilError(t, err)

	// Put a base image in the registry with an index.
	baseImageRef := testRegistry.Repo("unittest-base-image").Tag("amd64")
	baseImage, err := mutate.ConfigFile(empty.Image, &v1.ConfigFile{
		Architecture: "amd64",
	})
	assert.NilError(t, err)

	baseImageIndexRef := testRegistry.Repo("unittest-base-image").Tag("latest")
	mt, err := baseImage.MediaType()
	assert.NilError(t, err)
	baseImageIndex := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{
		Add: baseImage,
		Descriptor: v1.Descriptor{
			MediaType: mt,
			Platform: &v1.Platform{
				Architecture: "amd64",
				OS:           "linux",
			},
		},
	})
	err = remote.Push(baseImageRef, baseImage, remote.WithTransport(regSrv.Client().Transport))
	assert.NilError(t, err)
	err = remote.Push(baseImageIndexRef, baseImageIndex, remote.WithTransport(regSrv.Client().Transport))
	assert.NilError(t, err)

	// Set up the function directory.
	functionFS, functionDir := extractTestdata(t)

	// Build a Go function on top of the base image.
	b := &goBuilder{
		baseImage: baseImageIndexRef.String(),
		transport: regSrv.Client().Transport,
	}
	fnImgs, err := b.Build(context.Background(), functionFS, []string{"amd64"}, functionDir)
	assert.NilError(t, err)

	// Treat the resulting image as opaque and trust that ko has built something
	// reasonable. Just make sure we get an image back and it has the right
	// architecture.
	assert.Assert(t, cmp.Len(fnImgs, 1))
	fnImg := fnImgs[0]
	cfg, err := fnImg.ConfigFile()
	assert.NilError(t, err)
	assert.Equal(t, cfg.Architecture, "amd64")
}

func extractTestdata(t *testing.T) (afero.Fs, string) {
	dir, err := os.MkdirTemp("", "test-go-function-")
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	functionFS := afero.NewBasePathFs(afero.NewOsFs(), dir)
	tr := tar.NewReader(bytes.NewReader(goFunction))
	tfs := tarfs.New(tr)
	err = filesystem.CopyFilesBetweenFs(tfs, functionFS)
	assert.NilError(t, err)

	return afero.NewBasePathFs(functionFS, "/go-function"), filepath.Join(dir, "go-function")
}
