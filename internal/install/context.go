// Copyright 2025 Upbound Inc.
// All rights reserved

package install

import (
	"os"

	"k8s.io/client-go/rest"
)

// Context includes common data that installer consumers may utilize.
type Context struct {
	Kubeconfig *rest.Config
	Namespace  string
}

// CommonParams are common parameters for installing and upgrading.
type CommonParams struct {
	Set    map[string]string `help:"Set parameters."`
	File   *os.File          `help:"Parameters file."   short:"f"`
	Bundle *os.File          `help:"Local bundle path."`
}
