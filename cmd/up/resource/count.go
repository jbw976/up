// Copyright 2025 Upbound Inc.
// All rights reserved

package resource

import (
	"context"
	"strconv"

	"k8s.io/client-go/rest"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/resource/counter"
	"github.com/upbound/up/internal/upterm"

	_ "embed"
)

type countCmd struct {
	Kubeconfig string `env:"KUBECONFIG"                  help:"Path to kubeconfig file." type:"existingfile"`
	Context    string `help:"Kubeconfig context to use."`
}

//go:embed help/count.md
var countHelp string

// Help returns the help text for the count command.
func (c *countCmd) Help() string {
	return countHelp
}

type resourceRow struct {
	ResourceType string `json:"resourceType"`
	Count        int    `json:"count"`
}

func (c *countCmd) Run(ctx context.Context, printer upterm.Printer) error {
	var restCfg *rest.Config
	var err error

	if c.Context != "" {
		restCfg, err = kube.GetKubeConfigWithContext(c.Kubeconfig, c.Context)
	} else {
		restCfg, err = kube.GetKubeConfig(c.Kubeconfig)
	}
	if err != nil {
		return errors.Wrap(err, "cannot get kubeconfig")
	}

	cnt, err := counter.New(restCfg)
	if err != nil {
		return errors.Wrap(err, "cannot create resource counter")
	}

	counts, err := cnt.Count(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot count resources")
	}

	return c.printCounts(printer, counts)
}

func (c *countCmd) printCounts(printer upterm.Printer, counts *counter.ResourceCounts) error {
	fieldNames := []string{"RESOURCE TYPE", "COUNT"}
	extractFields := func(obj any) []string {
		row, ok := obj.(resourceRow)
		if !ok {
			return []string{"", ""}
		}
		return []string{row.ResourceType, strconv.Itoa(row.Count)}
	}

	rows := []resourceRow{
		{ResourceType: "Managed Resources", Count: counts.ManagedResources},
		{ResourceType: "Composite Resources", Count: counts.CompositeResources},
		{ResourceType: "Composite Resource Claims", Count: counts.CompositeResourceClaims},
		{ResourceType: "Composed Resources", Count: counts.ComposedResources},
		{ResourceType: "Total Resources", Count: counts.TotalResources},
	}
	return printer.PrintObject(rows, fieldNames, extractFields)
}
