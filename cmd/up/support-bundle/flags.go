// Copyright 2025 Upbound Inc.
// All rights reserved

package supportbundle

// commonFlags contains flags shared by multiple support-bundle commands.
type commonFlags struct {
	Kubeconfig        string   `help:"Path to the kubeconfig file. If not provided, the default kubeconfig resolution will be used." short:"k"`
	IncludeNamespaces []string `help:"Namespaces to include."                                                                        name:"include-namespaces"`
	ExcludeNamespaces []string `help:"Namespaces to exclude."                                                                        name:"exclude-namespaces"`
}
