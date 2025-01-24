// Copyright 2025 Upbound Inc.
// All rights reserved

package upbound

import (
	"net/url"
)

// Flags are common flags used by commands that interact with Upbound.
type Flags struct {
	// Optional
	Domain  *url.URL `env:"UP_DOMAIN"  help:"Root Upbound domain. Overrides the current profile's domain." json:"domain,omitempty"`
	Profile string   `env:"UP_PROFILE" help:"Profile used to execute command."                             json:"profile,omitempty" predictor:"profiles"`
	// Deprecated: Prefer Organization and fall back to Account if necessary.
	Account      string `env:"UP_ACCOUNT" help:"Deprecated. Use organization instead." json:"account,omitempty"                                                                   short:"a"`
	Organization string `alias:"org"      env:"UP_ORGANIZATION"                        help:"Organization used to execute command. Overrides the current profile's organization." json:"organization,omitempty"`

	// Insecure
	InsecureSkipTLSVerify bool `env:"UP_INSECURE_SKIP_TLS_VERIFY" help:"[INSECURE] Skip verifying TLS certificates."                                                                          json:"insecureSkipTLSVerify,omitempty"`
	Debug                 int  `env:"UP_DEBUG"                    help:"[INSECURE] Run with debug logging. Repeat to increase verbosity. Output might contain confidential data like tokens." json:"debug,omitempty"                 name:"debug" short:"d" type:"counter"`

	// Hidden
	APIEndpoint      *url.URL `env:"OVERRIDE_API_ENDPOINT"      help:"Overrides the default API endpoint."      hidden:"" json:"apiEndpoint,omitempty"      name:"override-api-endpoint"`
	AuthEndpoint     *url.URL `env:"OVERRIDE_AUTH_ENDPOINT"     help:"Overrides the default auth endpoint."     hidden:"" json:"authEndpoint,omitempty"     name:"override-auth-endpoint"`
	ProxyEndpoint    *url.URL `env:"OVERRIDE_PROXY_ENDPOINT"    help:"Overrides the default proxy endpoint."    hidden:"" json:"proxyEndpoint,omitempty"    name:"override-proxy-endpoint"`
	RegistryEndpoint *url.URL `env:"OVERRIDE_REGISTRY_ENDPOINT" help:"Overrides the default registry endpoint." hidden:"" json:"registryEndpoint,omitempty" name:"override-registry-endpoint"`
	AccountsEndpoint *url.URL `env:"OVERRIDE_ACCOUNTS_ENDPOINT" help:"Overrides the default accounts endpoint." hidden:"" json:"accountsEndpoint,omitempty" name:"override-accounts-endpoint"`

	Kube KubeFlags `embed:""`
}

// KubeFlags are common flags used by commands that interact with
// Kubernetes-like APIs.
type KubeFlags struct {
	// Kubeconfig is the kubeconfig file path to read. If empty, it refers to
	// client-go's default kubeconfig location.
	Kubeconfig string `help:"Override default kubeconfig path." type:"existingfile"`
	// Context is the context within Kubeconfig to read. If empty, it refers
	// to the default context.
	Context string `help:"Override default kubeconfig context." name:"kubecontext"`
}
