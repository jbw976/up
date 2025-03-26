// Copyright 2025 Upbound Inc.
// All rights reserved

// Package upbound contains common CLI configuration for working with Upbound
// services.
package upbound

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/alecthomas/kong"
	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	"go.uber.org/zap/zapcore"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xplogging "github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/upbound/up-sdk-go"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/logging"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/version"
)

const (
	// CookieName is the default cookie name used to identify a session token.
	CookieName = "SID"

	// Default API subdomain.
	apiSubdomain = "api."
	// Default auth subdomain.
	authSubdomain = "auth."
	// Default proxy subdomain.
	proxySubdomain = "proxy."
	// Default registry subdomain.
	xpkgSubdomain = "xpkg."
	// Default accounts subdomain.
	accountsSubdomain = "accounts."

	// Base path for proxy.
	proxyPath = "/v1/controlPlanes"
	// Base path for all controller client requests.
	controllerClientPath = "/apis"
)

const (
	errProfileNotFoundFmt = "profile not found with identifier: %s"
)

// Context includes common data that Upbound consumers may utilize.
type Context struct {
	// Profile fields
	ProfileName  string
	Profile      profile.Profile
	Token        string
	Cfg          *config.Config
	CfgSrc       config.Source
	Organization string

	// Kubeconfig fields. Direct access to this should be a last resort, used
	// when the actual kubeconfig file needs to be accessed or modified. Prefer
	// the helper methods attached to the Context, which will set appropriate
	// defaults such as the UserAgent when constructing clients.
	Kubecfg clientcmd.ClientConfig

	// Upbound API connection URLs
	Domain                *url.URL
	APIEndpoint           *url.URL
	AuthEndpoint          *url.URL
	ProxyEndpoint         *url.URL
	RegistryEndpoint      *url.URL
	AccountsEndpoint      *url.URL
	InsecureSkipTLSVerify bool

	// Logging
	Log        xplogging.Logger
	DebugLevel int

	// Miscellaneous
	allowMissingProfile bool
	cfgPath             string
	fs                  afero.Fs
	zl                  logr.Logger
}

// Option modifies a Context.
type Option func(*Context)

// AllowMissingProfile indicates that Context should still be returned even if a
// profile name is supplied and it does not exist in config.
func AllowMissingProfile() Option {
	return func(ctx *Context) {
		ctx.allowMissingProfile = true
	}
}

// HideLogging disables logging for the context (after calling SetupLogging).
func HideLogging() Option {
	return func(ctx *Context) {
		ctx.zl = zap.New(zap.Level(zapcore.FatalLevel))
		ctx.Log = xplogging.NewLogrLogger(ctx.zl)
	}
}

// NewFromFlags constructs a new context from flags.
func NewFromFlags(f Flags, opts ...Option) (*Context, error) {
	p, err := config.GetDefaultPath()
	if err != nil {
		return nil, err
	}

	c := &Context{
		fs:      afero.NewOsFs(),
		cfgPath: p,
	}

	for _, o := range opts {
		o(c)
	}

	src := config.NewFSSource(
		config.WithFS(c.fs),
		config.WithPath(c.cfgPath),
	)
	if err := src.Initialize(); err != nil {
		return nil, err
	}
	conf, err := config.Extract(src)
	if err != nil {
		return nil, err
	}

	c.Cfg = conf
	c.CfgSrc = src

	// If profile identifier is not provided, use the default, or empty if the
	// default cannot be obtained.
	c.Profile = profile.Profile{}
	if f.Profile == "" {
		if name, p, err := c.Cfg.GetDefaultUpboundProfile(); err == nil {
			c.Profile = p
			c.ProfileName = name
		}
	} else {
		p, err := c.Cfg.GetUpboundProfile(f.Profile)
		if err != nil && !c.allowMissingProfile {
			return nil, errors.Errorf(errProfileNotFoundFmt, f.Profile)
		}
		c.Profile = p
		c.ProfileName = f.Profile
	}

	of, err := c.applyOverrides(f, c.ProfileName)
	if err != nil {
		return nil, err
	}

	// Use flag values for account and domain if they're set - these override
	// the settings in the profile.
	c.Organization = of.Organization
	// Fall back to the deprecated account flag.
	if c.Organization == "" {
		c.Organization = of.Account
	}
	c.Domain = of.Domain

	// If account has not already been set, use the profile default.
	if c.Organization == "" {
		c.Organization = c.Profile.Organization
	}
	// If domain has not already been set, use the profile default. If the
	// profile doesn't have a domain, use the global default.
	if c.Domain == nil {
		domain := c.Profile.Domain
		if domain == "" {
			domain = config.DefaultDomain
		}

		c.Domain, err = url.Parse(domain)
		if err != nil {
			return nil, errors.Wrap(err, "invalid domain in profile")
		}
	}

	c.APIEndpoint = of.APIEndpoint
	if c.APIEndpoint == nil {
		u := *c.Domain
		u.Host = apiSubdomain + u.Host
		c.APIEndpoint = &u
	}

	c.AuthEndpoint = of.AuthEndpoint
	if c.AuthEndpoint == nil {
		u := *c.Domain
		u.Host = authSubdomain + u.Host
		c.AuthEndpoint = &u
	}

	c.ProxyEndpoint = of.ProxyEndpoint
	if c.ProxyEndpoint == nil {
		u := *c.Domain
		u.Host = proxySubdomain + u.Host
		u.Path = proxyPath
		c.ProxyEndpoint = &u
	}

	c.RegistryEndpoint = of.RegistryEndpoint
	if c.RegistryEndpoint == nil {
		u := *c.Domain
		u.Host = xpkgSubdomain + u.Host
		c.RegistryEndpoint = &u
	}

	c.AccountsEndpoint = of.AccountsEndpoint
	if c.AccountsEndpoint == nil {
		u := *c.Domain
		u.Host = accountsSubdomain + u.Host
		c.AccountsEndpoint = &u
	}

	c.InsecureSkipTLSVerify = of.InsecureSkipTLSVerify

	// setup logging
	c.DebugLevel = of.Debug
	if c.Log == nil {
		zapOpts := []zap.Opts{}
		if f.Debug > 0 {
			zapOpts = append(zapOpts, zap.Level(zapcore.DebugLevel))
		}
		c.zl = zap.New(zapOpts...).WithName("up")
		c.Log = xplogging.NewLogrLogger(c.zl)
	}

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = f.Kube.Kubeconfig
	c.Kubecfg = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules,
		&clientcmd.ConfigOverrides{CurrentContext: f.Kube.Context},
	)

	return c, nil
}

// SetupLogging sets up the logger in controller-runtime and kube's klog.
func (c *Context) SetupLogging() {
	if c.DebugLevel > 1 {
		logging.SetKlogLogger(c.DebugLevel, c.zl)
	}
	ctrl.SetLogger(c.zl)
}

// HideLogging disables logging for the context retrospectively. This is
// not thread safe.
func (c *Context) HideLogging() {
	c.zl = zap.New(zap.Level(zapcore.FatalLevel))
	c.Log = xplogging.NewLogrLogger(c.zl)
	c.SetupLogging()
}

// BuildSDKConfig builds an Upbound SDK config suitable for usage with any
// service client.
func (c *Context) BuildSDKConfig() (*up.Config, error) {
	return c.buildSDKConfig(c.APIEndpoint)
}

// BuildSDKAuthConfig builds an Upbound SDK config pointed at the Upbound auth
// endpoint.
func (c *Context) BuildSDKAuthConfig() (*up.Config, error) {
	return c.buildSDKConfig(c.AuthEndpoint)
}

func (c *Context) buildSDKConfig(endpoint *url.URL) (*up.Config, error) {
	cj, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	if c.Profile.Session != "" {
		cj.SetCookies(c.APIEndpoint, []*http.Cookie{
			{
				Name:  CookieName,
				Value: c.Profile.Session,
			},
		})
	}
	var tr http.RoundTripper = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: c.InsecureSkipTLSVerify, //nolint:gosec // Let the user be unsafe.
		},
	}
	client := up.NewClient(func(u *up.HTTPClient) {
		u.BaseURL = endpoint
		u.HTTP = &http.Client{
			Jar:       cj,
			Transport: tr,
		}
		u.UserAgent = version.UserAgent()
	})
	return up.NewConfig(func(conf *up.Config) {
		conf.Client = client
	}), nil
}

type cookieImpersonatingRoundTripper struct {
	session string
	rt      http.RoundTripper
}

func (rt *cookieImpersonatingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = utilnet.CloneRequest(req)
	req.AddCookie(&http.Cookie{
		Name:  CookieName,
		Value: rt.session,
	})
	return rt.rt.RoundTrip(req)
}

// BuildControllerClientConfig builds a REST config suitable for usage with any
// K8s controller-runtime client.
func (c *Context) BuildControllerClientConfig() (*rest.Config, error) {
	var tr http.RoundTripper = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: c.InsecureSkipTLSVerify, //nolint:gosec // Let the user be unsafe.
		},
	}

	// mcp-api doesn't support bearer token auth through to spaces APIs, yet.
	// For now, we need to add the SID cookie to every request to authenticate
	// it.
	tr = &cookieImpersonatingRoundTripper{session: c.Profile.Session, rt: tr}

	cfg := &rest.Config{
		Host:      c.APIEndpoint.String(),
		APIPath:   controllerClientPath,
		Transport: tr,
		UserAgent: version.UserAgent(),
	}

	if c.Profile.Session != "" {
		cfg.BearerToken = c.Profile.Session
	}
	return cfg, nil
}

// applyOverrides applies applicable overrides to the given Flags based on the
// pre-existing configs, if there are any.
func (c *Context) applyOverrides(f Flags, profileName string) (Flags, error) {
	// profile doesn't exist, return the supplied flags
	if _, ok := c.Cfg.Upbound.Profiles[profileName]; !ok {
		return f, nil
	}

	of := Flags{}

	baseReader, err := c.Cfg.BaseToJSON(profileName)
	if err != nil {
		return of, err
	}

	overlayBytes, err := json.Marshal(f)
	if err != nil {
		return of, err
	}

	resolver, err := JSON(baseReader, bytes.NewReader(overlayBytes))
	if err != nil {
		return of, err
	}
	parser, err := kong.New(&of, kong.Resolvers(resolver))
	if err != nil {
		return of, err
	}

	if _, err = parser.Parse([]string{}); err != nil {
		return of, err
	}

	return of, nil
}

// MarshalJSON marshals the Flags struct, converting the url.URL to strings.
func (f Flags) MarshalJSON() ([]byte, error) {
	flags := struct {
		Domain                string `json:"domain,omitempty"`
		Profile               string `json:"profile,omitempty"`
		Account               string `json:"account,omitempty"`
		Organization          string `json:"organization,omitempty"`
		InsecureSkipTLSVerify bool   `json:"insecure_skip_tls_verify,omitempty"` //nolint:tagliatelle // Not a k8s JSON.
		Debug                 int    `json:"debug,omitempty"`
		APIEndpoint           string `json:"override_api_endpoint,omitempty"`      //nolint:tagliatelle // Not a k8s JSON.
		AuthEndpoint          string `json:"override_auth_endpoint,omitempty"`     //nolint:tagliatelle // Not a k8s JSON.
		ProxyEndpoint         string `json:"override_proxy_endpoint,omitempty"`    //nolint:tagliatelle // Not a k8s JSON.
		RegistryEndpoint      string `json:"override_registry_endpoint,omitempty"` //nolint:tagliatelle // Not a k8s JSON.
	}{
		Domain:                nullableURL(f.Domain),
		Profile:               f.Profile,
		Account:               f.Account,
		Organization:          f.Organization,
		InsecureSkipTLSVerify: f.InsecureSkipTLSVerify,
		Debug:                 f.Debug,
		APIEndpoint:           nullableURL(f.APIEndpoint),
		AuthEndpoint:          nullableURL(f.AuthEndpoint),
		ProxyEndpoint:         nullableURL(f.ProxyEndpoint),
		RegistryEndpoint:      nullableURL(f.RegistryEndpoint),
	}
	return json.Marshal(flags)
}

func nullableURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.String()
}
