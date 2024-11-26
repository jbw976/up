// Copyright 2024 Upbound Inc
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

// Package space contains functions for handling spaces
package space

import (
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"gotest.tools/v3/assert"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/oci"
	"github.com/upbound/up/internal/upterm"
)

func TestMirror(t *testing.T) {
	t.Parallel()
	// Define test cases.
	tcs := map[string]struct {
		version                string
		outputDir              string
		destinationRegistry    string
		expectedError          string
		expectedOutput         []string
		mockFetchManifest      func(ref string, opts ...crane.Option) ([]byte, error)
		mockGetValuesFromChart func(chart, version string, pathNavigator oci.PathNavigator, username, password string) ([]string, error)
	}{
		"SpaceVersion194FolderOutput": {
			version:       "1.9.4",
			outputDir:     "testdata/output",
			expectedError: "",
			mockFetchManifest: func(_ string, _ ...crane.Option) ([]byte, error) {
				return []byte(`{"schemaVersion": 2}`), nil
			},
			mockGetValuesFromChart: mockGetValuesFromChart(pathNavigatorMockData{
				imageTag:         []string{"v0.0.0-441.g68777b9"},
				kubeVersionPath:  []string{"v1.31.0"},
				registerImageTag: []string{"v0.0.0-441.g68777b9"},
				uxpVersionsPath:  []string{"1.16.0-up.1", "1.16.2-up.2", "1.16.4-up.1", "1.17.1-up.1", "1.17.3-up.1", "1.18.0-up.1"},
				xgqlVersionPath:  []string{"v0.2.0-rc.0.167.gb4b3e68"},
			}),
			expectedOutput: []string{
				"crane pull xpkg.upbound.io/spaces-artifacts/agent:0.0.0-441.g68777b9 testdata/output/agent-0.0.0-441.g68777b9.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/agent:v0.0.0-441.g68777b9 testdata/output/agent-v0.0.0-441.g68777b9.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/coredns:1.10.1 testdata/output/coredns-1.10.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/coredns:latest testdata/output/coredns-latest.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/crossplane:v1.16.0-up.1 testdata/output/crossplane-v1.16.0-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/crossplane:v1.16.2-up.2 testdata/output/crossplane-v1.16.2-up.2.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/crossplane:v1.16.4-up.1 testdata/output/crossplane-v1.16.4-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/crossplane:v1.17.1-up.1 testdata/output/crossplane-v1.17.1-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/crossplane:v1.17.3-up.1 testdata/output/crossplane-v1.17.3-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/crossplane:v1.18.0-up.1 testdata/output/crossplane-v1.18.0-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/envoy:v1.26-latest testdata/output/envoy-v1.26-latest.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/etcd:3.5.15-0 testdata/output/etcd-3.5.15-0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/external-secrets-chart:0.10.4-up.1 testdata/output/external-secrets-chart-0.10.4-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/external-secrets:v0.10.4-up.1 testdata/output/external-secrets-v0.10.4-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/external-secrets:v0.9.20-up.1 testdata/output/external-secrets-v0.9.20-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/hyperspace:v1.9.4 testdata/output/hyperspace-v1.9.4.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kine:v0.0.0-224.g6a07aa9 testdata/output/kine-v0.0.0-224.g6a07aa9.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kube-apiserver:v1.31.0 testdata/output/kube-apiserver-v1.31.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kube-controller-manager:v1.31.0 testdata/output/kube-controller-manager-v1.31.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kube-scheduler:v1.31.0 testdata/output/kube-scheduler-v1.31.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kube-state-metrics:v2.8.1-upbound003 testdata/output/kube-state-metrics-v2.8.1-upbound003.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kubectl:1.31.0 testdata/output/kubectl-1.31.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kyverno-background-controller:v1.11.4 testdata/output/kyverno-background-controller-v1.11.4.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kyverno-cleanup-controller:v1.11.4 testdata/output/kyverno-cleanup-controller-v1.11.4.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kyverno-kyverno:v1.11.4 testdata/output/kyverno-kyverno-v1.11.4.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kyverno-kyvernopre:v1.11.4 testdata/output/kyverno-kyvernopre-v1.11.4.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kyverno-reports-controller:v1.11.4 testdata/output/kyverno-reports-controller-v1.11.4.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/mcp-connector-server:v0.7.0 testdata/output/mcp-connector-server-v0.7.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/mcp-connector:0.7.0 testdata/output/mcp-connector-0.7.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/mxp-benchmark:v1.9.4 testdata/output/mxp-benchmark-v1.9.4.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/mxp-charts:v1.9.4 testdata/output/mxp-charts-v1.9.4.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/opentelemetry-collector-contrib:0.98.0 testdata/output/opentelemetry-collector-contrib-0.98.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/opentelemetry-collector-spaces:v1.9.4 testdata/output/opentelemetry-collector-spaces-v1.9.4.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/register-init:v0.0.0-441.g68777b9 testdata/output/register-init-v0.0.0-441.g68777b9.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/spaces:1.9.4 testdata/output/spaces-1.9.4.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.16.0-up.1 testdata/output/universal-crossplane-1.16.0-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.16.2-up.2 testdata/output/universal-crossplane-1.16.2-up.2.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.16.4-up.1 testdata/output/universal-crossplane-1.16.4-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.17.1-up.1 testdata/output/universal-crossplane-1.17.1-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.17.3-up.1 testdata/output/universal-crossplane-1.17.3-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.18.0-up.1 testdata/output/universal-crossplane-1.18.0-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/uxp-bootstrapper:v1.10.4-up.2 testdata/output/uxp-bootstrapper-v1.10.4-up.2.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/vcluster:0.15.7 testdata/output/vcluster-0.15.7.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/vector:0.41.1-distroless-libc testdata/output/vector-0.41.1-distroless-libc.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/xgql:v0.2.0-rc.0.167.gb4b3e68 testdata/output/xgql-v0.2.0-rc.0.167.gb4b3e68.tgz",
			},
		},
		"SpaceVersion180FolderOutput": {
			version:       "1.8.0",
			outputDir:     "testdata/output",
			expectedError: "",
			mockFetchManifest: func(_ string, _ ...crane.Option) ([]byte, error) {
				return []byte(`{"schemaVersion": 2}`), nil
			},
			mockGetValuesFromChart: mockGetValuesFromChart(pathNavigatorMockData{
				imageTag:         []string{"v0.0.0-441.g68777b9"},
				kubeVersionPath:  []string{"v1.31.0"},
				registerImageTag: []string{"v0.0.0-441.g68777b9"},
				uxpVersionsPath:  []string{"1.15.0-up.1", "1.15.1-up.1", "1.15.2-up.1", "1.15.3-up.1", "1.15.5-up.2", "1.16.0-up.1", "1.16.2-up.2", "1.17.1-up.1"},
				xgqlVersionPath:  []string{"v0.2.0-rc.0.167.gb4b3e68"},
			}),
			expectedOutput: []string{
				"crane pull xpkg.upbound.io/spaces-artifacts/agent:0.0.0-441.g68777b9 testdata/output/agent-0.0.0-441.g68777b9.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/agent:v0.0.0-441.g68777b9 testdata/output/agent-v0.0.0-441.g68777b9.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/coredns:1.10.1 testdata/output/coredns-1.10.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/coredns:latest testdata/output/coredns-latest.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/crossplane:v1.15.0-up.1 testdata/output/crossplane-v1.15.0-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/crossplane:v1.15.1-up.1 testdata/output/crossplane-v1.15.1-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/crossplane:v1.15.2-up.1 testdata/output/crossplane-v1.15.2-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/crossplane:v1.15.3-up.1 testdata/output/crossplane-v1.15.3-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/crossplane:v1.15.5-up.2 testdata/output/crossplane-v1.15.5-up.2.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/crossplane:v1.16.0-up.1 testdata/output/crossplane-v1.16.0-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/crossplane:v1.16.2-up.2 testdata/output/crossplane-v1.16.2-up.2.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/crossplane:v1.17.1-up.1 testdata/output/crossplane-v1.17.1-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/envoy:v1.26-latest testdata/output/envoy-v1.26-latest.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/etcd:3.5.15-0 testdata/output/etcd-3.5.15-0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/external-secrets:v0.9.20-up.1 testdata/output/external-secrets-v0.9.20-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/hyperspace:v1.8.0 testdata/output/hyperspace-v1.8.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kine:v0.0.0-224.g6a07aa9 testdata/output/kine-v0.0.0-224.g6a07aa9.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kube-apiserver:v1.31.0 testdata/output/kube-apiserver-v1.31.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kube-controller-manager:v1.31.0 testdata/output/kube-controller-manager-v1.31.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kube-scheduler:v1.31.0 testdata/output/kube-scheduler-v1.31.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kube-state-metrics:v2.8.1-upbound003 testdata/output/kube-state-metrics-v2.8.1-upbound003.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kubectl:1.31.0 testdata/output/kubectl-1.31.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kyverno-background-controller:v1.11.4 testdata/output/kyverno-background-controller-v1.11.4.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kyverno-cleanup-controller:v1.11.4 testdata/output/kyverno-cleanup-controller-v1.11.4.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kyverno-kyverno:v1.11.4 testdata/output/kyverno-kyverno-v1.11.4.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kyverno-kyvernopre:v1.11.4 testdata/output/kyverno-kyvernopre-v1.11.4.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/kyverno-reports-controller:v1.11.4 testdata/output/kyverno-reports-controller-v1.11.4.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/mcp-connector-server:v0.7.0 testdata/output/mcp-connector-server-v0.7.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/mcp-connector:0.7.0 testdata/output/mcp-connector-0.7.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/mxp-benchmark:v1.8.0 testdata/output/mxp-benchmark-v1.8.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/mxp-charts:v1.8.0 testdata/output/mxp-charts-v1.8.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/opentelemetry-collector-contrib:0.98.0 testdata/output/opentelemetry-collector-contrib-0.98.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/opentelemetry-collector-spaces:v1.8.0 testdata/output/opentelemetry-collector-spaces-v1.8.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/register-init:v0.0.0-441.g68777b9 testdata/output/register-init-v0.0.0-441.g68777b9.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/spaces:1.8.0 testdata/output/spaces-1.8.0.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.15.0-up.1 testdata/output/universal-crossplane-1.15.0-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.15.1-up.1 testdata/output/universal-crossplane-1.15.1-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.15.2-up.1 testdata/output/universal-crossplane-1.15.2-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.15.3-up.1 testdata/output/universal-crossplane-1.15.3-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.15.5-up.2 testdata/output/universal-crossplane-1.15.5-up.2.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.16.0-up.1 testdata/output/universal-crossplane-1.16.0-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.16.2-up.2 testdata/output/universal-crossplane-1.16.2-up.2.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.17.1-up.1 testdata/output/universal-crossplane-1.17.1-up.1.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/uxp-bootstrapper:v1.10.4-up.2 testdata/output/uxp-bootstrapper-v1.10.4-up.2.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/vcluster:0.15.7 testdata/output/vcluster-0.15.7.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/vector:0.41.1-distroless-libc testdata/output/vector-0.41.1-distroless-libc.tgz",
				"crane pull xpkg.upbound.io/spaces-artifacts/xgql:v0.2.0-rc.0.167.gb4b3e68 testdata/output/xgql-v0.2.0-rc.0.167.gb4b3e68.tgz",
			},
		},
		"SpaceVersion180RegistryOutput": {
			version:             "1.8.0",
			destinationRegistry: "haarchri.io/spaces",
			expectedError:       "",
			mockFetchManifest: func(_ string, _ ...crane.Option) ([]byte, error) {
				return []byte(`{"schemaVersion": 2}`), nil
			},
			mockGetValuesFromChart: mockGetValuesFromChart(pathNavigatorMockData{
				imageTag:         []string{"v0.0.0-441.g68777b9"},
				kubeVersionPath:  []string{"v1.31.0"},
				registerImageTag: []string{"v0.0.0-441.g68777b9"},
				uxpVersionsPath:  []string{"1.15.0-up.1", "1.15.1-up.1", "1.15.2-up.1", "1.15.3-up.1", "1.15.5-up.2", "1.16.0-up.1", "1.16.2-up.2", "1.17.1-up.1"},
				xgqlVersionPath:  []string{"v0.2.0-rc.0.167.gb4b3e68"},
			}),
			expectedOutput: []string{
				"crane copy xpkg.upbound.io/spaces-artifacts/agent:0.0.0-441.g68777b9 haarchri.io/spaces/agent:0.0.0-441.g68777b9",
				"crane copy xpkg.upbound.io/spaces-artifacts/agent:v0.0.0-441.g68777b9 haarchri.io/spaces/agent:v0.0.0-441.g68777b9",
				"crane copy xpkg.upbound.io/spaces-artifacts/coredns:1.10.1 haarchri.io/spaces/coredns:1.10.1",
				"crane copy xpkg.upbound.io/spaces-artifacts/coredns:latest haarchri.io/spaces/coredns:latest",
				"crane copy xpkg.upbound.io/spaces-artifacts/crossplane:v1.15.0-up.1 haarchri.io/spaces/crossplane:v1.15.0-up.1",
				"crane copy xpkg.upbound.io/spaces-artifacts/crossplane:v1.15.1-up.1 haarchri.io/spaces/crossplane:v1.15.1-up.1",
				"crane copy xpkg.upbound.io/spaces-artifacts/crossplane:v1.15.2-up.1 haarchri.io/spaces/crossplane:v1.15.2-up.1",
				"crane copy xpkg.upbound.io/spaces-artifacts/crossplane:v1.15.3-up.1 haarchri.io/spaces/crossplane:v1.15.3-up.1",
				"crane copy xpkg.upbound.io/spaces-artifacts/crossplane:v1.15.5-up.2 haarchri.io/spaces/crossplane:v1.15.5-up.2",
				"crane copy xpkg.upbound.io/spaces-artifacts/crossplane:v1.16.0-up.1 haarchri.io/spaces/crossplane:v1.16.0-up.1",
				"crane copy xpkg.upbound.io/spaces-artifacts/crossplane:v1.16.2-up.2 haarchri.io/spaces/crossplane:v1.16.2-up.2",
				"crane copy xpkg.upbound.io/spaces-artifacts/crossplane:v1.17.1-up.1 haarchri.io/spaces/crossplane:v1.17.1-up.1",
				"crane copy xpkg.upbound.io/spaces-artifacts/envoy:v1.26-latest haarchri.io/spaces/envoy:v1.26-latest",
				"crane copy xpkg.upbound.io/spaces-artifacts/etcd:3.5.15-0 haarchri.io/spaces/etcd:3.5.15-0",
				"crane copy xpkg.upbound.io/spaces-artifacts/external-secrets:v0.9.20-up.1 haarchri.io/spaces/external-secrets:v0.9.20-up.1",
				"crane copy xpkg.upbound.io/spaces-artifacts/hyperspace:v1.8.0 haarchri.io/spaces/hyperspace:v1.8.0",
				"crane copy xpkg.upbound.io/spaces-artifacts/kine:v0.0.0-224.g6a07aa9 haarchri.io/spaces/kine:v0.0.0-224.g6a07aa9",
				"crane copy xpkg.upbound.io/spaces-artifacts/kube-apiserver:v1.31.0 haarchri.io/spaces/kube-apiserver:v1.31.0",
				"crane copy xpkg.upbound.io/spaces-artifacts/kube-controller-manager:v1.31.0 haarchri.io/spaces/kube-controller-manager:v1.31.0",
				"crane copy xpkg.upbound.io/spaces-artifacts/kube-scheduler:v1.31.0 haarchri.io/spaces/kube-scheduler:v1.31.0",
				"crane copy xpkg.upbound.io/spaces-artifacts/kube-state-metrics:v2.8.1-upbound003 haarchri.io/spaces/kube-state-metrics:v2.8.1-upbound003",
				"crane copy xpkg.upbound.io/spaces-artifacts/kubectl:1.31.0 haarchri.io/spaces/kubectl:1.31.0",
				"crane copy xpkg.upbound.io/spaces-artifacts/kyverno-background-controller:v1.11.4 haarchri.io/spaces/kyverno-background-controller:v1.11.4",
				"crane copy xpkg.upbound.io/spaces-artifacts/kyverno-cleanup-controller:v1.11.4 haarchri.io/spaces/kyverno-cleanup-controller:v1.11.4",
				"crane copy xpkg.upbound.io/spaces-artifacts/kyverno-kyverno:v1.11.4 haarchri.io/spaces/kyverno-kyverno:v1.11.4",
				"crane copy xpkg.upbound.io/spaces-artifacts/kyverno-kyvernopre:v1.11.4 haarchri.io/spaces/kyverno-kyvernopre:v1.11.4",
				"crane copy xpkg.upbound.io/spaces-artifacts/kyverno-reports-controller:v1.11.4 haarchri.io/spaces/kyverno-reports-controller:v1.11.4",
				"crane copy xpkg.upbound.io/spaces-artifacts/mcp-connector-server:v0.7.0 haarchri.io/spaces/mcp-connector-server:v0.7.0",
				"crane copy xpkg.upbound.io/spaces-artifacts/mcp-connector:0.7.0 haarchri.io/spaces/mcp-connector:0.7.0",
				"crane copy xpkg.upbound.io/spaces-artifacts/mxp-benchmark:v1.8.0 haarchri.io/spaces/mxp-benchmark:v1.8.0",
				"crane copy xpkg.upbound.io/spaces-artifacts/mxp-charts:v1.8.0 haarchri.io/spaces/mxp-charts:v1.8.0",
				"crane copy xpkg.upbound.io/spaces-artifacts/opentelemetry-collector-contrib:0.98.0 haarchri.io/spaces/opentelemetry-collector-contrib:0.98.0",
				"crane copy xpkg.upbound.io/spaces-artifacts/opentelemetry-collector-spaces:v1.8.0 haarchri.io/spaces/opentelemetry-collector-spaces:v1.8.0",
				"crane copy xpkg.upbound.io/spaces-artifacts/register-init:v0.0.0-441.g68777b9 haarchri.io/spaces/register-init:v0.0.0-441.g68777b9",
				"crane copy xpkg.upbound.io/spaces-artifacts/spaces:1.8.0 haarchri.io/spaces/spaces:1.8.0",
				"crane copy xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.15.0-up.1 haarchri.io/spaces/universal-crossplane:1.15.0-up.1",
				"crane copy xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.15.1-up.1 haarchri.io/spaces/universal-crossplane:1.15.1-up.1",
				"crane copy xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.15.2-up.1 haarchri.io/spaces/universal-crossplane:1.15.2-up.1",
				"crane copy xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.15.3-up.1 haarchri.io/spaces/universal-crossplane:1.15.3-up.1",
				"crane copy xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.15.5-up.2 haarchri.io/spaces/universal-crossplane:1.15.5-up.2",
				"crane copy xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.16.0-up.1 haarchri.io/spaces/universal-crossplane:1.16.0-up.1",
				"crane copy xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.16.2-up.2 haarchri.io/spaces/universal-crossplane:1.16.2-up.2",
				"crane copy xpkg.upbound.io/spaces-artifacts/universal-crossplane:1.17.1-up.1 haarchri.io/spaces/universal-crossplane:1.17.1-up.1",
				"crane copy xpkg.upbound.io/spaces-artifacts/uxp-bootstrapper:v1.10.4-up.2 haarchri.io/spaces/uxp-bootstrapper:v1.10.4-up.2",
				"crane copy xpkg.upbound.io/spaces-artifacts/vcluster:0.15.7 haarchri.io/spaces/vcluster:0.15.7",
				"crane copy xpkg.upbound.io/spaces-artifacts/vector:0.41.1-distroless-libc haarchri.io/spaces/vector:0.41.1-distroless-libc",
				"crane copy xpkg.upbound.io/spaces-artifacts/xgql:v0.2.0-rc.0.167.gb4b3e68 haarchri.io/spaces/xgql:v0.2.0-rc.0.167.gb4b3e68",
			},
		},
		"InvalidVersion": {
			version:       "v2.invalid",
			outputDir:     "testdata/output",
			expectedError: "mirror artifacts failed: unable to find spaces version: manifest not found",
			mockFetchManifest: func(_ string, _ ...crane.Option) ([]byte, error) {
				return nil, errors.New("manifest not found")
			},
		},
	}

	for testName, tc := range tcs {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			// Capture output
			var capturedOutput []string
			mockPrinter := func(format string, a ...interface{}) {
				capturedOutput = append(capturedOutput, fmt.Sprintf(format, a...))
			}

			// Create a new command instance
			cmd := &mirrorCmd{
				Version:             tc.version,
				path:                tc.outputDir,
				DestinationRegistry: tc.destinationRegistry,
				fetchManifest:       tc.mockFetchManifest,      // Inject the mock
				getValuesFromChart:  tc.mockGetValuesFromChart, // Inject the mock
				defaultPrint:        mockPrinter,               // Inject the mock
			}

			printer := upterm.DefaultObjPrinter
			printer.DryRun = true

			// Run the mirror command
			err := cmd.Run(printer)

			// Validate results
			if tc.expectedError != "" {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NilError(t, err)
			}

			// Use maps to track occurrences
			expectedMap := make(map[string]int)
			capturedMap := make(map[string]int)

			// Populate the maps
			for _, entry := range tc.expectedOutput {
				expectedMap[entry]++
			}
			for _, entry := range capturedOutput {
				capturedMap[entry]++
			}

			// Check for missing and extra entries
			missingEntries := []string{}
			extraEntries := []string{}

			for key, expectedCount := range expectedMap {
				capturedCount, found := capturedMap[key]
				if !found || capturedCount < expectedCount {
					missingEntries = append(missingEntries, fmt.Sprintf("%s (expected: %d, got: %d)", key, expectedCount, capturedCount))
				}
			}

			for key, capturedCount := range capturedMap {
				expectedCount, found := expectedMap[key]
				if !found || capturedCount > expectedCount {
					extraEntries = append(extraEntries, fmt.Sprintf("%s (expected: %d, got: %d)", key, expectedCount, capturedCount))
				}
			}

			// Report differences
			if len(missingEntries) > 0 || len(extraEntries) > 0 {
				if len(missingEntries) > 0 {
					t.Logf("Missing entries: %v", missingEntries)
				}
				if len(extraEntries) > 0 {
					t.Logf("Extra entries: %v", extraEntries)
				}
				t.Fatalf("Mismatched output: expected %d entries but got %d", len(tc.expectedOutput), len(capturedOutput))
			}
		})
	}
}

type pathNavigatorMockData struct {
	imageTag         []string
	kubeVersionPath  []string
	registerImageTag []string
	uxpVersionsPath  []string
	xgqlVersionPath  []string
}

func mockGetValuesFromChart(data pathNavigatorMockData) func(chart, version string, pathNavigator oci.PathNavigator, username, password string) ([]string, error) {
	return func(_, _ string, pathNavigator oci.PathNavigator, _, _ string) ([]string, error) {
		switch v := pathNavigator.(type) {
		case *uxpVersionsPath:
			return data.uxpVersionsPath, nil
		case *kubeVersionPath:
			return data.kubeVersionPath, nil
		case *xgqlVersionPath:
			return data.xgqlVersionPath, nil
		case *imageTag:
			return data.imageTag, nil
		case *registerImageTag:
			return data.registerImageTag, nil
		default:
			return nil, fmt.Errorf("unsupported type: %T", v)
		}
	}
}
