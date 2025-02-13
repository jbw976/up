// Copyright 2025 Upbound Inc.
// All rights reserved

package migration

import (
	"k8s.io/client-go/rest"
)

// Context includes common data that migration commands may utilize.
type Context struct {
	Kubeconfig *rest.Config
}
