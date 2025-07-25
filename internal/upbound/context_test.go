// Copyright 2025 Upbound Inc.
// All rights reserved

package upbound

import (
	"fmt"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/profile"
)

var (
	defaultConfigJSON = `{
		"upbound": {
		  "default": "default",
		  "profiles": {
			"default": {
			  "id": "someone@upbound.io",
			  "type": "user",
			  "session": "a token"
			}
		  }
		}
	  }
	`
	domainConfigJSON = `{
		"upbound": {
		  "default": "default",
		  "profiles": {
			"default": {
			  "id": "someone@upbound.io",
			  "type": "user",
			  "session": "a token",
			  "domain": "https://local.upbound.io"
			}
		  }
		}
	  }
	`
	accountConfigJSON = `{
		"upbound": {
		  "default": "default",
		  "profiles": {
			"default": {
			  "id": "someone@upbound.io",
			  "type": "user",
			  "session": "a token",
			  "account": "my-org"
			}
		  }
		}
	  }
	`
	organizationConfigJSON = `{
		"upbound": {
		  "default": "default",
		  "profiles": {
			"default": {
			  "id": "someone@upbound.io",
			  "type": "user",
			  "session": "a token",
			  "organization": "my-org"
			}
		  }
		}
	  }
	`
	baseConfigJSON = `{
		"upbound": {
		  "default": "default",
		  "profiles": {
			"default": {
			  "id": "someone@upbound.io",
			  "type": "user",
			  "session": "a token",
			  "base": {
				"UP_DOMAIN": "https://local.upbound.io",
				"UP_ACCOUNT": "my-org",
				"UP_INSECURE_SKIP_TLS_VERIFY": "true"
			  }
			},
			"cool-profile": {
				"id": "someone@upbound.io",
				"type": "user",
				"session": "a token",
				"base": {
				  "UP_DOMAIN": "https://local.upbound.io",
				  "UP_ACCOUNT": "my-org",
				  "UP_INSECURE_SKIP_TLS_VERIFY": "true"
				}
			  }
		  }
		}
	  }
	`
)

func withConfig(config string) Option {
	return func(ctx *Context) {
		// establish fs and create config.json
		fs := afero.NewMemMapFs()
		fs.MkdirAll(filepath.Dir("/.up/"), 0o755)
		f, _ := fs.Create("/.up/config.json")

		f.WriteString(config)

		ctx.fs = fs
	}
}

func withFS(fs afero.Fs) Option {
	return func(ctx *Context) {
		ctx.fs = fs
	}
}

func withPath(p string) Option {
	return func(ctx *Context) {
		ctx.cfgPath = p
	}
}

func withURL(uri string) *url.URL {
	u, _ := url.Parse(uri)
	return u
}

func TestNewFromFlags(t *testing.T) {
	type args struct {
		flags []string
		opts  []Option
	}
	type want struct {
		err error
		c   *Context
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoPreExistingProfile": {
			reason: "We should successfully return a Context if a pre-existing profile does not exist.",
			args: args{
				flags: []string{},
				opts: []Option{
					withFS(afero.NewMemMapFs()),
				},
			},
			want: want{
				c: &Context{
					Organization:     "",
					APIEndpoint:      withURL("https://api.upbound.io"),
					Cfg:              &config.Config{},
					Domain:           withURL("https://upbound.io"),
					Profile:          profile.Profile{},
					AuthEndpoint:     withURL("https://auth.upbound.io"),
					ProxyEndpoint:    withURL("https://proxy.upbound.io/v1/controlPlanes"),
					RegistryEndpoint: withURL("https://xpkg.upbound.io"),
					AccountsEndpoint: withURL("https://accounts.upbound.io"),
				},
			},
		},
		"ErrorSuppliedNotExist": {
			reason: "We should return an error if profile is supplied and it does not exist.",
			args: args{
				flags: []string{
					"--profile=not-here",
				},
			},
			want: want{
				err: errors.Errorf(errProfileNotFoundFmt, "not-here"),
			},
		},
		"SuppliedNotExistAllowEmpty": {
			reason: "We should successfully return a Context if a supplied profile does not exist and.",
			args: args{
				flags: []string{
					"--profile=not-here",
				},
				opts: []Option{
					withFS(afero.NewMemMapFs()),
					AllowMissingProfile(),
				},
			},
			want: want{
				c: &Context{
					ProfileName:      "not-here",
					Organization:     "",
					APIEndpoint:      withURL("https://api.upbound.io"),
					Cfg:              &config.Config{},
					Domain:           withURL("https://upbound.io"),
					Profile:          profile.Profile{},
					AuthEndpoint:     withURL("https://auth.upbound.io"),
					ProxyEndpoint:    withURL("https://proxy.upbound.io/v1/controlPlanes"),
					RegistryEndpoint: withURL("https://xpkg.upbound.io"),
					AccountsEndpoint: withURL("https://accounts.upbound.io"),
				},
			},
		},
		"PreExistingProfileNoBaseConfig": {
			reason: "We should successfully return a Context if a pre-existing profile exists, but does not have a base config",
			args: args{
				flags: []string{},
				opts: []Option{
					withConfig(defaultConfigJSON),
					withPath("/.up/config.json"),
				},
			},
			want: want{
				c: &Context{
					ProfileName:           "default",
					Organization:          "",
					APIEndpoint:           withURL("https://api.upbound.io"),
					Domain:                withURL("https://upbound.io"),
					InsecureSkipTLSVerify: false,
					Profile: profile.Profile{
						ID:        "someone@upbound.io",
						Type:      profile.TypeCloud,
						TokenType: profile.TokenTypeUser,
						Session:   "a token",
						Account:   "",
					},
					AuthEndpoint:     withURL("https://auth.upbound.io"),
					ProxyEndpoint:    withURL("https://proxy.upbound.io/v1/controlPlanes"),
					RegistryEndpoint: withURL("https://xpkg.upbound.io"),
					AccountsEndpoint: withURL("https://accounts.upbound.io"),
					Token:            "",
				},
			},
		},
		"PreExistingProfileWithDomain": {
			reason: "We should successfully return a Context if a pre-existing profile exists with a domain configured",
			args: args{
				flags: []string{},
				opts: []Option{
					withConfig(domainConfigJSON),
					withPath("/.up/config.json"),
				},
			},
			want: want{
				c: &Context{
					ProfileName:           "default",
					Organization:          "",
					APIEndpoint:           withURL("https://api.local.upbound.io"),
					Domain:                withURL("https://local.upbound.io"),
					InsecureSkipTLSVerify: false,
					Profile: profile.Profile{
						ID:        "someone@upbound.io",
						Type:      profile.TypeCloud,
						TokenType: profile.TokenTypeUser,
						Session:   "a token",
						Account:   "",
						Domain:    "https://local.upbound.io",
					},
					AuthEndpoint:     withURL("https://auth.local.upbound.io"),
					ProxyEndpoint:    withURL("https://proxy.local.upbound.io/v1/controlPlanes"),
					RegistryEndpoint: withURL("https://xpkg.local.upbound.io"),
					AccountsEndpoint: withURL("https://accounts.local.upbound.io"),
					Token:            "",
				},
			},
		},
		"PreExistingProfileWithAccount": {
			reason: "We should successfully return a Context if a pre-existing profile exists with a (deprecated) account configured",
			args: args{
				flags: []string{},
				opts: []Option{
					withConfig(accountConfigJSON),
					withPath("/.up/config.json"),
				},
			},
			want: want{
				c: &Context{
					ProfileName:           "default",
					Organization:          "my-org",
					APIEndpoint:           withURL("https://api.upbound.io"),
					Domain:                withURL("https://upbound.io"),
					InsecureSkipTLSVerify: false,
					Profile: profile.Profile{
						ID:           "someone@upbound.io",
						Type:         profile.TypeCloud,
						TokenType:    profile.TokenTypeUser,
						Session:      "a token",
						Account:      "my-org",
						Organization: "my-org",
						Domain:       "",
					},
					AuthEndpoint:     withURL("https://auth.upbound.io"),
					ProxyEndpoint:    withURL("https://proxy.upbound.io/v1/controlPlanes"),
					RegistryEndpoint: withURL("https://xpkg.upbound.io"),
					AccountsEndpoint: withURL("https://accounts.upbound.io"),
					Token:            "",
				},
			},
		},
		"PreExistingProfileWithOrganization": {
			reason: "We should successfully return a Context if a pre-existing profile exists with an organization configured",
			args: args{
				flags: []string{},
				opts: []Option{
					withConfig(organizationConfigJSON),
					withPath("/.up/config.json"),
				},
			},
			want: want{
				c: &Context{
					ProfileName:           "default",
					Organization:          "my-org",
					APIEndpoint:           withURL("https://api.upbound.io"),
					Domain:                withURL("https://upbound.io"),
					InsecureSkipTLSVerify: false,
					Profile: profile.Profile{
						ID:           "someone@upbound.io",
						Type:         profile.TypeCloud,
						TokenType:    profile.TokenTypeUser,
						Session:      "a token",
						Organization: "my-org",
						Domain:       "",
					},
					AuthEndpoint:     withURL("https://auth.upbound.io"),
					ProxyEndpoint:    withURL("https://proxy.upbound.io/v1/controlPlanes"),
					RegistryEndpoint: withURL("https://xpkg.upbound.io"),
					AccountsEndpoint: withURL("https://accounts.upbound.io"),
					Token:            "",
				},
			},
		},
		"PreExistingProfileBaseConfigSetProfile": {
			reason: "We should return a Context that includes the persisted Profile from base config",
			args: args{
				flags: []string{},
				opts: []Option{
					withConfig(baseConfigJSON),
					withPath("/.up/config.json"),
				},
			},
			want: want{
				c: &Context{
					ProfileName:           "default",
					Organization:          "my-org",
					APIEndpoint:           withURL("https://api.local.upbound.io"),
					Domain:                withURL("https://local.upbound.io"),
					InsecureSkipTLSVerify: true,
					Profile: profile.Profile{
						ID:        "someone@upbound.io",
						Type:      profile.TypeCloud,
						TokenType: profile.TokenTypeUser,
						Session:   "a token",
						Account:   "",
						BaseConfig: map[string]string{
							"UP_ACCOUNT":                  "my-org",
							"UP_DOMAIN":                   "https://local.upbound.io",
							"UP_INSECURE_SKIP_TLS_VERIFY": "true",
						},
					},
					AuthEndpoint:     withURL("https://auth.local.upbound.io"),
					ProxyEndpoint:    withURL("https://proxy.local.upbound.io/v1/controlPlanes"),
					RegistryEndpoint: withURL("https://xpkg.local.upbound.io"),
					AccountsEndpoint: withURL("https://accounts.local.upbound.io"),
					Token:            "",
				},
			},
		},
		"PreExistingBaseConfigOverrideThroughFlags": {
			reason: "We should return a Context that includes the persisted Profile from base config overridden based on flags",
			args: args{
				flags: []string{
					"--profile=cool-profile",
					"--account=not-my-org",
					fmt.Sprintf("--domain=%s", withURL("http://a.domain.org")),
					fmt.Sprintf("--override-api-endpoint=%s", withURL("http://not.a.url")),
				},
				opts: []Option{
					withConfig(baseConfigJSON),
					withPath("/.up/config.json"),
				},
			},
			want: want{
				c: &Context{
					ProfileName:           "cool-profile",
					Organization:          "not-my-org",
					APIEndpoint:           withURL("http://not.a.url"),
					Domain:                withURL("http://a.domain.org"),
					InsecureSkipTLSVerify: true,
					Profile: profile.Profile{
						ID:        "someone@upbound.io",
						Type:      profile.TypeCloud,
						TokenType: profile.TokenTypeUser,
						Session:   "a token",
						Account:   "",
						BaseConfig: map[string]string{
							"UP_ACCOUNT":                  "my-org",
							"UP_DOMAIN":                   "https://local.upbound.io",
							"UP_INSECURE_SKIP_TLS_VERIFY": "true",
						},
					},
					AuthEndpoint:     withURL("http://auth.a.domain.org"),
					ProxyEndpoint:    withURL("http://proxy.a.domain.org/v1/controlPlanes"),
					RegistryEndpoint: withURL("http://xpkg.a.domain.org"),
					AccountsEndpoint: withURL("http://accounts.a.domain.org"),
					Token:            "",
				},
			},
		},
		"DebugCounterFlag": {
			reason: "Multiple debug flags should increase debug level.",
			args: args{
				flags: []string{"-d", "--debug", "-d"},
				opts: []Option{
					withFS(afero.NewMemMapFs()),
				},
			},
			want: want{
				c: &Context{
					Organization:     "",
					APIEndpoint:      withURL("https://api.upbound.io"),
					Cfg:              &config.Config{},
					Domain:           withURL("https://upbound.io"),
					Profile:          profile.Profile{},
					AuthEndpoint:     withURL("https://auth.upbound.io"),
					ProxyEndpoint:    withURL("https://proxy.upbound.io/v1/controlPlanes"),
					RegistryEndpoint: withURL("https://xpkg.upbound.io"),
					AccountsEndpoint: withURL("https://accounts.upbound.io"),
					DebugLevel:       3,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			flags := Flags{}
			parser, _ := kong.New(&flags)
			parser.Parse(tc.args.flags)

			upCtx, err := NewFromFlags(flags, tc.args.opts...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("NewFromFlags(...): -want error, +got error:\n%s", diff)
			}
			if upCtx == nil {
				return
			}
			upCtx.SetupLogging()

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nNewFromFlags(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.c, upCtx,
				cmpopts.IgnoreUnexported(Context{}),
				// NOTE(tnthornton): we're not concerned about the FSSource's
				// internal components.
				cmpopts.IgnoreFields(Context{}, "CfgSrc"),
				// NOTE(tnthornton) we're not concerned about the Cfg's
				// internal components.
				cmpopts.IgnoreFields(Context{}, "Cfg"),
				// NOTE(redbackthomson) we're not concerned about the logic used
				// to load the default kubeconfig.
				cmpopts.IgnoreFields(Context{}, "Kubecfg"),
				// an interface pointer we cannot compare
				cmpopts.IgnoreFields(Context{}, "Log"),
			); diff != "" {
				t.Errorf("\n%s\nNewFromFlags(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
