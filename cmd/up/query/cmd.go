// Copyright 2025 Upbound Inc.
// All rights reserved

// Please note: As of March 2023, the `upbound` commands have been disabled.
// We're keeping the code here for now, so they're easily resurrected.
// The upbound commands were meant to support the Upbound self-hosted option.

package query

import (
	"k8s.io/kubectl/pkg/cmd/get"

	"github.com/upbound/up/internal/upbound"
)

// QueryCmd contains commands for querying control plane objects.
type cmd struct {
	// general printer flags
	OutputFormat string   `help:"Output format. One of: json,yaml,kyaml,name,go-template,go-template-file,template,templatefile,jsonpath,jsonpath-as-json,jsonpath-file,custom-columns,custom-columns-file,wide"                                                                              name:"output"        short:"o"`
	NoHeaders    bool     `help:"When using the default or custom-column output format, don't print headers."`
	ShowLabels   bool     `help:"When printing, show all labels as the last column (default hide labels column)"                                                                                                                                                                              name:"show-labels"`
	SortBy       string   `help:"If non-empty, sort list types using this field specification.  The field specification is expressed as a JSONPath expression (e.g. '{.metadata.name}'). The field in the API resource specified by this JSONPath expression must be an integer or a string." name:"sort-by"`
	ColumnLabels []string `help:"Accepts a comma separated list of labels that are going to be presented as columns. Names are case-sensitive. You can also use multiple flag options like -L label1 -L label2..."                                                                            name:"label-columns"`
	ShowKind     bool     `help:"If present, list the resource type for the requested object(s)."                                                                                                                                                                                             name:"show-kind"`

	// template printer flags
	Template         string `help:"Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview]." name:"template"                    short:"t"`
	AllowMissingKeys bool   `help:"If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats."                                        name:"allow-missing-template-keys"`

	// json/yaml flags
	ShowManagedFields bool `help:"If true, keep the managedFields when printing objects in JSON or YAML format." name:"show-managed-fields"`

	// positional arguments
	Resources []string `arg:"" help:"Type(s) (resource, singular or plural, category, short-name) and names: TYPE[.GROUP][,TYPE[.GROUP]...] [NAME ...] | TYPE[.GROUP]/NAME .... If no resource is specified, all resources are queried, but --all-resources must be specified."`

	Flags upbound.Flags `embed:""`

	printFlags *get.PrintFlags
	namespace  string // inside the control plane
}

// NotFound print Message NotFound.
type NotFound interface {
	PrintMessage() error
}

// NotFoundFunc is a function type that implements the NotFound interface.
type NotFoundFunc func() error

// PrintMessage print a message.
func (f NotFoundFunc) PrintMessage() error {
	return f()
}
