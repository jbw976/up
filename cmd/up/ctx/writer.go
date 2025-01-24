// Copyright 2025 Upbound Inc.
// All rights reserved

package ctx

import (
	"fmt"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/upbound/up/internal/kube"
)

type printWriter struct{}

var _ kube.ContextWriter = &printWriter{}

// Write implements kubeContextWriter.Write.
func (p *printWriter) Write(config *clientcmdapi.Config) error {
	b, err := clientcmd.Write(*config)
	if err != nil {
		return err
	}

	fmt.Print(string(b)) //nolint:forbidigo // The printWriter is allowed to print.
	return nil
}
