// Copyright 2025 Upbound Inc
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

package ctx

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/upbound/up/internal/upbound"
)

func TestFileWriterWrite(t *testing.T) {
	t.Parallel()

	spaceExtension := upbound.NewCloudV1Alpha1SpaceExtension("my-org", "my-space")
	extensionMap := map[string]runtime.Object{upbound.ContextExtensionKeySpace: spaceExtension}

	tests := map[string]struct {
		outConf *clientcmdapi.Config
		inConf  *clientcmdapi.Config

		wantConf *clientcmdapi.Config
		wantLast string
	}{
		"Empty": {
			outConf: &clientcmdapi.Config{
				Contexts:  map[string]*clientcmdapi.Context{},
				Clusters:  map[string]*clientcmdapi.Cluster{},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{},
			},
			inConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token"}},
			},
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token"}},
			},
			wantLast: "",
		},
		"NoUpboundContext": {
			outConf: &clientcmdapi.Config{
				CurrentContext: "other",
				Contexts: map[string]*clientcmdapi.Context{
					"other": {Cluster: "other", AuthInfo: "other"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"other": {Server: "https://other"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"other": {Token: "other"},
				},
			},
			inConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token"}},
			},
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"other":   {Cluster: "other", AuthInfo: "other"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"upbound": {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}},
					"other":   {Server: "https://other"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"upbound": {Token: "token"},
					"other":   {Token: "other"},
				},
			},
			wantLast: "other",
		},
		"UpboundNotCurrentContext": {
			outConf: &clientcmdapi.Config{
				CurrentContext: "other",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"other":   {Cluster: "other", AuthInfo: "other"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"upbound": {Server: "https://old-ingress", CertificateAuthorityData: []uint8{1, 2, 3}},
					"other":   {Server: "https://other"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"upbound": {Token: "old-token"},
					"other":   {Token: "other"},
				},
			},
			inConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token"}},
			},
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"other":   {Cluster: "other", AuthInfo: "other"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"upbound": {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}},
					"other":   {Server: "https://other"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"upbound": {Token: "token"},
					"other":   {Token: "other"},
				},
			},
			wantLast: "other",
		},
		"UpboundIsCurrentContext": {
			outConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"other":   {Cluster: "other", AuthInfo: "other"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"upbound": {Server: "https://old-ingress", CertificateAuthorityData: []uint8{1, 2, 3}},
					"other":   {Server: "https://other"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"upbound": {Token: "old-token"},
					"other":   {Token: "other"},
				},
			},
			inConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token"}},
			},
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"upbound-previous": {Cluster: "upbound-previous", AuthInfo: "upbound-previous", Extensions: extensionMap},
					"other":            {Cluster: "other", AuthInfo: "other"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"upbound":          {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}},
					"upbound-previous": {Server: "https://old-ingress", CertificateAuthorityData: []uint8{1, 2, 3}},
					"other":            {Server: "https://other"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"upbound":          {Token: "token"},
					"upbound-previous": {Token: "old-token"},
					"other":            {Token: "other"},
				},
			},
			wantLast: "upbound-previous",
		},
		"UpboundPreviousIsCurrentContext": {
			outConf: &clientcmdapi.Config{
				CurrentContext: "upbound-previous",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"upbound-previous": {Cluster: "upbound-previous", AuthInfo: "upbound-previous", Extensions: extensionMap},
					"other":            {Cluster: "other", AuthInfo: "other"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"upbound":          {Server: "https://old-ingress", CertificateAuthorityData: []uint8{1, 2, 3}},
					"upbound-previous": {Server: "https://previous-ingress", CertificateAuthorityData: []uint8{1, 2, 3}},
					"other":            {Server: "https://other"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"upbound":          {Token: "old-token"},
					"upbound-previous": {Token: "previous-token"},
					"other":            {Token: "other"},
				},
			},
			inConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token"}},
			},
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"upbound-previous": {Cluster: "upbound-previous", AuthInfo: "upbound-previous", Extensions: extensionMap},
					"other":            {Cluster: "other", AuthInfo: "other"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"upbound":          {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}},
					"upbound-previous": {Server: "https://previous-ingress", CertificateAuthorityData: []uint8{1, 2, 3}},
					"other":            {Server: "https://other"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					"upbound":          {Token: "token"},
					"upbound-previous": {Token: "previous-token"},
					"other":            {Token: "other"},
				},
			},
			wantLast: "upbound-previous",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var last string
			var conf *clientcmdapi.Config
			upCtx := &upbound.Context{Kubecfg: clientcmd.NewDefaultClientConfig(*tt.outConf, nil)}
			writer := &fileWriter{
				upCtx:            upCtx,
				kubeContext:      "upbound",
				writeLastContext: func(c string) error { last = c; return nil },
				verify:           func(_ *clientcmdapi.Config) error { return nil },
				modifyConfig: func(_ clientcmd.ConfigAccess, newConfig clientcmdapi.Config, _ bool) error {
					conf = &newConfig
					return nil
				},
			}

			err := writer.Write(tt.inConf)
			if diff := cmp.Diff(nil, err); diff != "" {
				t.Fatalf("Write(...): -want err, +got err:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantConf, conf); diff != "" {
				t.Errorf("Write(...): -want conf, +got conf:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantLast, last); diff != "" {
				t.Errorf("Write(...): -want last, +got last:\n%s", diff)
			}
		})
	}
}
