// Copyright 2025 Upbound Inc.
// All rights reserved

package supportbundle

// commonFlags contains flags shared by multiple support-bundle commands.
type commonFlags struct {
	Kubeconfig        string   `help:"Path to the kubeconfig file. If not provided, the default kubeconfig resolution will be used."                                                                                                                                                                                                   short:"k"`
	IncludeNamespaces []string `help:"Namespaces to include in the support bundle. When not specified, collects crossplane-system, upbound-system, and namespaces labeled with internal.spaces.upbound.io/controlplane-name or spaces.upbound.io/group. Supports glob patterns (e.g., upbound-*). Multiple patterns can be specified." name:"include-namespaces"`
	ExcludeNamespaces []string `help:"Namespaces to exclude from the support bundle. Supports glob patterns (e.g., upbound-* to exclude all namespaces starting with \"upbound-\"). Multiple patterns can be specified."                                                                                                               name:"exclude-namespaces"`
}
