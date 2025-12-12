// Copyright 2025 Upbound Inc.
// All rights reserved

package ctx

import (
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upterm"
)

type printWriter struct {
	printer upterm.Printer
}

var _ kube.ContextWriter = &printWriter{}

// Write implements kubeContextWriter.Write.
func (p *printWriter) Write(config *clientcmdapi.Config) error {
	b, err := clientcmd.Write(*config)
	if err != nil {
		return err
	}

	p.printer.PrintResult(string(b))
	return nil
}
