// Copyright 2025 Upbound Inc.
// All rights reserved

package ctx

import (
	"context"
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

func TestGetCurrentGroup(t *testing.T) {
	cloudExt := &upbound.SpaceExtension{
		Spec: &upbound.SpaceExtensionSpec{
			Cloud: &upbound.CloudConfiguration{
				Organization: "myorg",
				SpaceName:    "myspace",
			},
		},
	}
	cloudExtRaw, err := json.Marshal(cloudExt)
	assert.NilError(t, err)
	cloudExtUnknown := &runtime.Unknown{
		Raw: cloudExtRaw,
	}

	tests := map[string]struct {
		upCtx         *upbound.Context
		wantSpacePath string
		wantGroupPath string
		wantErr       error
	}{
		"NotInSpace": {
			upCtx: &upbound.Context{
				Profile: profile.Profile{
					Type:         profile.TypeCloud,
					Organization: "myorg",
				},
				Kubecfg: &clientcmd.DefaultClientConfig,
			},
			wantErr: errors.New("current kubeconfig context is not in an Upbound Space"),
		},
		"Space": {
			upCtx: &upbound.Context{
				Profile: profile.Profile{
					Type:         profile.TypeCloud,
					Organization: "myorg",
				},
				Kubecfg: clientcmd.NewDefaultClientConfig(clientcmdapi.Config{
					CurrentContext: "upbound",
					Contexts: map[string]*clientcmdapi.Context{
						"upbound": {
							Cluster:  "upbound",
							AuthInfo: "upbound",
							Extensions: map[string]runtime.Object{
								upbound.ContextExtensionKeySpace: cloudExtUnknown,
							},
						},
					},
					Clusters: map[string]*clientcmdapi.Cluster{
						"upbound": {
							Server:                   "https://myspace.myorg.spaces.upbound.io",
							CertificateAuthorityData: []byte("fake-ca"),
						},
					},
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"upbound": {
							Token: "fake-token",
						},
					},
				}, &clientcmd.ConfigOverrides{}),
			},
			wantSpacePath: "myorg/myspace",
		},
		"Group": {
			upCtx: &upbound.Context{
				Profile: profile.Profile{
					Type:         profile.TypeCloud,
					Organization: "myorg",
				},
				Kubecfg: clientcmd.NewDefaultClientConfig(clientcmdapi.Config{
					CurrentContext: "upbound",
					Contexts: map[string]*clientcmdapi.Context{
						"upbound": {
							Cluster:  "upbound",
							AuthInfo: "upbound",
							Extensions: map[string]runtime.Object{
								upbound.ContextExtensionKeySpace: cloudExtUnknown,
							},
							Namespace: "mygroup",
						},
					},
					Clusters: map[string]*clientcmdapi.Cluster{
						"upbound": {
							Server:                   "https://myspace.myorg.spaces.upbound.io",
							CertificateAuthorityData: []byte("fake-ca"),
						},
					},
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"upbound": {
							Token: "fake-token",
						},
					},
				}, &clientcmd.ConfigOverrides{}),
			},
			wantSpacePath: "myorg/myspace",
			wantGroupPath: "myorg/myspace/mygroup",
		},
		"ControlPlane": {
			upCtx: &upbound.Context{
				Profile: profile.Profile{
					Type:         profile.TypeCloud,
					Organization: "myorg",
				},
				Kubecfg: clientcmd.NewDefaultClientConfig(clientcmdapi.Config{
					CurrentContext: "upbound",
					Contexts: map[string]*clientcmdapi.Context{
						"upbound": {
							Cluster:  "upbound",
							AuthInfo: "upbound",
							Extensions: map[string]runtime.Object{
								upbound.ContextExtensionKeySpace: cloudExtUnknown,
							},
							Namespace: "mygroup",
						},
					},
					Clusters: map[string]*clientcmdapi.Cluster{
						"upbound": {
							Server:                   "https://myspace.myorg.spaces.upbound.io/apis/spaces.upbound.io/v1beta1/namespaces/mygroup/controlplanes/myctp/k8s",
							CertificateAuthorityData: []byte("fake-ca"),
						},
					},
					AuthInfos: map[string]*clientcmdapi.AuthInfo{
						"upbound": {
							Token: "fake-token",
						},
					},
				}, &clientcmd.ConfigOverrides{}),
			},
			wantSpacePath: "myorg/myspace",
			wantGroupPath: "myorg/myspace/mygroup",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gotSpace, gotGroup, err := GetCurrentGroup(context.Background(), tc.upCtx)
			if tc.wantErr != nil {
				assert.Error(t, err, tc.wantErr.Error())
				return
			}

			assert.NilError(t, err)
			assert.Equal(t, tc.wantSpacePath, gotSpace.Breadcrumbs().String())
			if tc.wantGroupPath == "" {
				assert.Assert(t, cmp.Nil(gotGroup))
			} else {
				assert.Equal(t, tc.wantGroupPath, gotGroup.Breadcrumbs().String())
			}
		})
	}
}
