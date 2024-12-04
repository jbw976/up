// Copyright 2023 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package upbound

import (
	"net/url"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/upbound/up/internal/version"
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

	// set by AfterApply
	config    *rest.Config
	context   string
	namespace string
}

// AfterApply applies defaults to KubeFlags.
func (f *KubeFlags) AfterApply() error {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = f.Kubeconfig
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules,
		&clientcmd.ConfigOverrides{CurrentContext: f.Context},
	)

	f.context = f.Context
	if f.context == "" {
		// Get the name of the default context so we can set it explicitly.
		rawConfig, err := loader.RawConfig()
		if err != nil {
			return err
		}
		f.context = rawConfig.CurrentContext
	}

	restConfig, err := loader.ClientConfig()
	if err != nil {
		return err
	}
	restConfig.UserAgent = version.UserAgent()
	f.config = restConfig

	ns, _, err := loader.Namespace()
	if err != nil {
		return err
	}
	f.namespace = ns

	return nil
}

// GetConfig returns the *rest.Config from KubeFlags. Returns nil unless
// AfterApply has been called.
func (f *KubeFlags) GetConfig() *rest.Config {
	return f.config
}

// GetContext returns the kubeconfig context from KubeFlags. Returns empty
// string unless AfterApply has been called. Returns KubeFlags.Context if it's
// defined, otherwise the name of the default context in the config resolved
// from KubeFlags.Kubeconfig.
// NOTE(branden): This ensures that a profile created from this context will
// continue to work with the same cluster if the kubeconfig's default context
// is changed.
func (f *KubeFlags) GetContext() string {
	return f.context
}

// Namespace gets the namespace flag.
func (f *KubeFlags) Namespace() string {
	return f.namespace
}
