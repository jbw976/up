// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package profile

import (
	"fmt"
	"net/url"
	"testing"

	"gotest.tools/v3/assert"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

func TestCreateAfterApply(t *testing.T) {
	t.Parallel()

	tcs := map[string]struct {
		name         string
		profileType  profile.Type
		organization string
		ctx          *upbound.Context

		errText string
	}{
		"SuccessCloud": {
			name:         "new-cloud",
			profileType:  profile.TypeCloud,
			organization: "my-org",
			ctx: &upbound.Context{
				ProfileName: "default",
				Profile:     profile.Profile{},
				Cfg: &config.Config{
					Upbound: config.Upbound{
						Default: "default",
						Profiles: map[string]profile.Profile{
							"default": {},
						},
					},
				},
			},
		},
		"SuccessDisconnectedWithOrganization": {
			name:         "new-disconnected",
			profileType:  profile.TypeDisconnected,
			organization: "my-org",
			ctx: &upbound.Context{
				ProfileName: "default",
				Profile:     profile.Profile{},
				Cfg: &config.Config{
					Upbound: config.Upbound{
						Default: "default",
						Profiles: map[string]profile.Profile{
							"default": {},
						},
					},
				},
			},
		},
		"SuccessDisconnectedWithoutOrganization": {
			name:        "new-disconnected",
			profileType: profile.TypeDisconnected,
			ctx: &upbound.Context{
				ProfileName: "default",
				Profile:     profile.Profile{},
				Cfg: &config.Config{
					Upbound: config.Upbound{
						Default: "default",
						Profiles: map[string]profile.Profile{
							"default": {},
						},
					},
				},
			},
		},
		"CloudWithoutOrganization": {
			name:        "new-cloud",
			profileType: profile.TypeCloud,
			ctx: &upbound.Context{
				ProfileName: "default",
				Profile:     profile.Profile{},
				Cfg: &config.Config{
					Upbound: config.Upbound{
						Default: "default",
						Profiles: map[string]profile.Profile{
							"default": {},
						},
					},
				},
			},
			errText: "organization is required for cloud profiles",
		},
		"DuplicateName": {
			name:        "default",
			profileType: profile.TypeDisconnected,
			ctx: &upbound.Context{
				ProfileName: "default",
				Profile:     profile.Profile{},
				Cfg: &config.Config{
					Upbound: config.Upbound{
						Default: "default",
						Profiles: map[string]profile.Profile{
							"default": {},
						},
					},
				},
			},
			errText: fmt.Sprintf("a profile named %q already exists; use `up profile set` to update it if desired", "default"),
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := &createCmd{
				Name: tc.name,
				Type: tc.profileType,
			}
			flags := upbound.Flags{
				Organization: tc.organization,
			}

			err := c.AfterApply(flags, tc.ctx)
			if tc.errText == "" {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tc.errText)
			}
		})
	}
}

func TestCreateRun(t *testing.T) {
	t.Parallel()

	domain, _ := url.Parse("https://donotuse.example.com")
	kubeconfig := clientcmdapi.Config{
		CurrentContext: "default",
		Contexts: map[string]*clientcmdapi.Context{
			"default": clientcmdapi.NewContext(),
		},
		Clusters:  map[string]*clientcmdapi.Cluster{},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{},
	}

	tcs := map[string]struct {
		name         string
		use          bool
		profileType  profile.Type
		organization string
		ctx          *upbound.Context
		updateFunc   func(t *testing.T) func(c *config.Config) error
	}{
		"FirstProfileDisconnected": {
			name:         "new-disconnected",
			profileType:  profile.TypeDisconnected,
			organization: "my-org",
			ctx: &upbound.Context{
				Domain:  domain,
				Kubecfg: clientcmd.NewDefaultClientConfig(kubeconfig, nil),
			},
			updateFunc: func(t *testing.T) func(c *config.Config) error {
				return func(c *config.Config) error {
					assert.DeepEqual(t, c.Upbound.Profiles["new-disconnected"], profile.Profile{
						Type:            profile.TypeDisconnected,
						Organization:    "my-org",
						SpaceKubeconfig: &kubeconfig,
						Domain:          domain.String(),
					})
					assert.Equal(t, c.Upbound.Default, "new-disconnected")
					return nil
				}
			},
		},
		"FirstProfileCloud": {
			name:         "new-cloud",
			profileType:  profile.TypeCloud,
			organization: "my-org",
			ctx: &upbound.Context{
				Domain: domain,
			},
			updateFunc: func(t *testing.T) func(c *config.Config) error {
				return func(c *config.Config) error {
					assert.DeepEqual(t, c.Upbound.Profiles["new-cloud"], profile.Profile{
						Type:         profile.TypeCloud,
						Organization: "my-org",
						Domain:       domain.String(),
					})
					assert.Equal(t, c.Upbound.Default, "new-cloud")
					return nil
				}
			},
		},
		"SecondProfileDisconnected": {
			name:         "new-disconnected",
			profileType:  profile.TypeDisconnected,
			organization: "my-org",
			ctx: &upbound.Context{
				Domain:  domain,
				Kubecfg: clientcmd.NewDefaultClientConfig(kubeconfig, nil),
				Cfg: &config.Config{
					Upbound: config.Upbound{
						Default: "old-profile",
						Profiles: map[string]profile.Profile{
							"old-profile": {},
						},
					},
				},
			},
			updateFunc: func(t *testing.T) func(c *config.Config) error {
				return func(c *config.Config) error {
					assert.DeepEqual(t, c.Upbound.Profiles["new-disconnected"], profile.Profile{
						Type:            profile.TypeDisconnected,
						Organization:    "my-org",
						SpaceKubeconfig: &kubeconfig,
						Domain:          domain.String(),
					})
					assert.Equal(t, c.Upbound.Default, "old-profile")
					return nil
				}
			},
		},
		"SecondProfileCloud": {
			name:         "new-cloud",
			profileType:  profile.TypeCloud,
			organization: "my-org",
			ctx: &upbound.Context{
				Domain: domain,
				Cfg: &config.Config{
					Upbound: config.Upbound{
						Default: "old-profile",
						Profiles: map[string]profile.Profile{
							"old-profile": {},
						},
					},
				},
			},
			updateFunc: func(t *testing.T) func(c *config.Config) error {
				return func(c *config.Config) error {
					assert.DeepEqual(t, c.Upbound.Profiles["new-cloud"], profile.Profile{
						Type:         profile.TypeCloud,
						Organization: "my-org",
						Domain:       domain.String(),
					})
					assert.Equal(t, c.Upbound.Default, "old-profile")
					return nil
				}
			},
		},
		"SecondProfileDisconnectedWithUse": {
			name:         "new-disconnected",
			use:          true,
			profileType:  profile.TypeDisconnected,
			organization: "my-org",
			ctx: &upbound.Context{
				Domain:  domain,
				Kubecfg: clientcmd.NewDefaultClientConfig(kubeconfig, nil),
				Cfg: &config.Config{
					Upbound: config.Upbound{
						Default: "old-profile",
						Profiles: map[string]profile.Profile{
							"old-profile": {},
						},
					},
				},
			},
			updateFunc: func(t *testing.T) func(c *config.Config) error {
				return func(c *config.Config) error {
					assert.DeepEqual(t, c.Upbound.Profiles["new-disconnected"], profile.Profile{
						Type:            profile.TypeDisconnected,
						Organization:    "my-org",
						SpaceKubeconfig: &kubeconfig,
						Domain:          domain.String(),
					})
					assert.Equal(t, c.Upbound.Default, "new-disconnected")
					return nil
				}
			},
		},
		"SecondProfileCloudWithUse": {
			name:         "new-cloud",
			use:          true,
			profileType:  profile.TypeCloud,
			organization: "my-org",
			ctx: &upbound.Context{
				Domain: domain,
				Cfg: &config.Config{
					Upbound: config.Upbound{
						Default: "old-profile",
						Profiles: map[string]profile.Profile{
							"old-profile": {},
						},
					},
				},
			},
			updateFunc: func(t *testing.T) func(c *config.Config) error {
				return func(c *config.Config) error {
					assert.DeepEqual(t, c.Upbound.Profiles["new-cloud"], profile.Profile{
						Type:         profile.TypeCloud,
						Organization: "my-org",
						Domain:       domain.String(),
					})
					assert.Equal(t, c.Upbound.Default, "new-cloud")
					return nil
				}
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := &createCmd{
				Name: tc.name,
				Use:  tc.use,
				Type: tc.profileType,
			}
			flags := upbound.Flags{
				Organization: tc.organization,
			}

			if tc.ctx.Cfg == nil {
				tc.ctx.Cfg = &config.Config{
					Upbound: config.Upbound{},
				}
			}
			tc.ctx.CfgSrc = &config.MockSource{
				UpdateConfigFn: tc.updateFunc(t),
			}
			err := c.Run(flags, tc.ctx)
			assert.NilError(t, err)
		})
	}
}
