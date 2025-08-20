// Copyright 2025 Upbound Inc.
// All rights reserved

// AI Generated. Human reviewed.

package apiconnector

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

func createTempKubeconfig(t *testing.T) string {
	// Create space extension to enable proper context derivation
	spaceExtension := upbound.NewCloudV1Alpha1SpaceExtension("test-org", "upbound-gcp-us-west-1")
	extensionMap := map[string]runtime.Object{upbound.ContextExtensionKeySpace: spaceExtension}

	cfg := &clientcmdapi.Config{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "test-context",
		Contexts: map[string]*clientcmdapi.Context{
			"test-context": {
				Cluster:    "test-cluster",
				AuthInfo:   "test-auth",
				Namespace:  "default",
				Extensions: extensionMap,
			},
		},
		Clusters: map[string]*clientcmdapi.Cluster{
			"test-cluster": {
				Server: "https://upbound-gcp-us-west-1.spaces.upbound.io/apis/spaces.upbound.io/v1beta1/namespaces/default/controlplanes/test-cp/k8s",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"test-auth": {
				Token: "test-token",
			},
		},
	}

	// Write to temporary file
	tmpDir := t.TempDir()

	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
	if err := clientcmd.WriteToFile(*cfg, kubeconfigPath); err != nil {
		t.Fatalf("Failed to write kubeconfig: %v", err)
	}

	return kubeconfigPath
}

func createTempKubeconfigWithInvalidSpace(t *testing.T) string {
	// Create space extension with invalid space name (doesn't start with "upbound-")
	spaceExtension := upbound.NewCloudV1Alpha1SpaceExtension("test-org", "invalid-space-name")
	extensionMap := map[string]runtime.Object{upbound.ContextExtensionKeySpace: spaceExtension}

	cfg := &clientcmdapi.Config{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "test-context",
		Contexts: map[string]*clientcmdapi.Context{
			"test-context": {
				Cluster:    "test-cluster",
				AuthInfo:   "test-auth",
				Namespace:  "default",
				Extensions: extensionMap,
			},
		},
		Clusters: map[string]*clientcmdapi.Cluster{
			"test-cluster": {
				Server: "https://invalid-space-name.spaces.upbound.io/apis/spaces.upbound.io/v1beta1/namespaces/default/controlplanes/test-cp/k8s",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"test-auth": {
				Token: "test-token",
			},
		},
	}

	// Write to temporary file
	tmpDir := t.TempDir()

	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
	if err := clientcmd.WriteToFile(*cfg, kubeconfigPath); err != nil {
		t.Fatalf("Failed to write kubeconfig: %v", err)
	}

	return kubeconfigPath
}

func createMockUpboundContext() *upbound.Context {
	// Create space extension to enable proper context derivation
	spaceExtension := upbound.NewCloudV1Alpha1SpaceExtension("test-org", "upbound-gcp-us-west-1")
	extensionMap := map[string]runtime.Object{upbound.ContextExtensionKeySpace: spaceExtension}

	cfg := &clientcmdapi.Config{
		CurrentContext: "test-context",
		Contexts: map[string]*clientcmdapi.Context{
			"test-context": {
				Cluster:    "test-cluster",
				AuthInfo:   "test-auth",
				Namespace:  "default",
				Extensions: extensionMap,
			},
		},
		Clusters: map[string]*clientcmdapi.Cluster{
			"test-cluster": {
				Server: "https://upbound-gcp-us-west-1.spaces.upbound.io/apis/spaces.upbound.io/v1beta1/namespaces/default/controlplanes/test-cp/k8s",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"test-auth": {},
		},
	}

	return &upbound.Context{
		Profile: profile.Profile{
			Organization:    "test-org",
			Type:            profile.TypeCloud,
			SpaceKubeconfig: cfg,
		},
		Organization: "test-org",
		Kubecfg:      clientcmd.NewDefaultClientConfig(*cfg, nil),
	}
}

func createMockUpboundContextWithoutCurrentContext() *upbound.Context {
	cfg := &clientcmdapi.Config{}
	return &upbound.Context{
		Profile: profile.Profile{
			Organization: "test-org",
			Type:         profile.TypeCloud,
		},
		Kubecfg: clientcmd.NewDefaultClientConfig(*cfg, nil),
	}
}

func TestCmd_AfterApply(t *testing.T) {
	tests := map[string]struct {
		reason                          string
		input                           installCmd
		expectedErr                     error
		expected                        *installCmd
		useContextWithoutCurrentContext bool
		useInvalidSpaceName             bool
	}{
		"NoCurrentContext": {
			reason:                          "Should return error when no current context is available",
			input:                           installCmd{},
			expectedErr:                     errors.New("current context must be set to a control plane (expected format: organization/space/group/controlplane)"),
			useContextWithoutCurrentContext: true,
		},
		"InvalidSpaceName": {
			reason:              "Should return error when space name doesn't start with 'upbound-'",
			input:               installCmd{},
			expectedErr:         errors.New("space name must start with 'upbound-'"),
			useInvalidSpaceName: true,
		},
		"ValidContextParsing": {
			reason: "Should successfully parse context and set up command",
			input:  installCmd{},
			expected: &installCmd{
				organization:     "test-org",
				space:            "upbound-gcp-us-west-1",
				group:            "default",
				name:             "api-connector-test-cp",
				controlPlaneName: "test-cp",
				spacesHostname:   "https://upbound-gcp-us-west-1.spaces.upbound.io",
			},
		},
		"ValidContextWithCustomName": {
			reason: "Should successfully parse context and use custom name",
			input: installCmd{
				Name: "custom-name",
			},
			expected: &installCmd{
				organization:     "test-org",
				space:            "upbound-gcp-us-west-1",
				group:            "default",
				name:             "custom-name",
				Name:             "custom-name",
				controlPlaneName: "test-cp",
				spacesHostname:   "https://upbound-gcp-us-west-1.spaces.upbound.io",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// This test is not parallel safe due to the use of t.TempDir()
			var upCtx *upbound.Context
			var cleanup func()

			switch {
			case tc.useContextWithoutCurrentContext:
				upCtx = createMockUpboundContextWithoutCurrentContext()
			case tc.useInvalidSpaceName:
				// Set up temporary kubeconfig with invalid space name
				kubeconfigPath := createTempKubeconfigWithInvalidSpace(t)
				cleanup = func() {
					os.RemoveAll(filepath.Dir(kubeconfigPath))
				}
				defer func() {
					if cleanup != nil {
						cleanup()
					}
				}()

				// Set KUBECONFIG environment variable
				originalKubeconfig := os.Getenv("KUBECONFIG")
				t.Setenv("KUBECONFIG", kubeconfigPath)
				defer t.Setenv("KUBECONFIG", originalKubeconfig)

				upCtx = createMockUpboundContext()
				tc.input.ConsumerKubeconfig = kubeconfigPath
			default:
				// Set up temporary kubeconfig for context validation
				kubeconfigPath := createTempKubeconfig(t)
				cleanup = func() {
					os.RemoveAll(filepath.Dir(kubeconfigPath))
				}
				defer func() {
					if cleanup != nil {
						cleanup()
					}
				}()

				// Set KUBECONFIG environment variable
				originalKubeconfig := os.Getenv("KUBECONFIG")
				t.Setenv("KUBECONFIG", kubeconfigPath)
				defer t.Setenv("KUBECONFIG", originalKubeconfig)

				upCtx = createMockUpboundContext()
				tc.input.ConsumerKubeconfig = kubeconfigPath
			}

			cmd := tc.input

			err := cmd.AfterApply(nil, upCtx)
			if tc.expectedErr != nil {
				if !strings.Contains(err.Error(), tc.expectedErr.Error()) { // we do contains as with "nicer" error messages we can't do direct comparison.
					t.Errorf("expected error '%v' but got: '%v'", tc.expectedErr, err)
				}
			}
			if tc.expectedErr == nil && err != nil {
				t.Errorf("expected no error but got: '%v'", err)
			}

			// Nil fields in cmd we dont test for.
			cmd.parser = nil
			cmd.consumerClient = nil
			cmd.spaceClient = nil
			cmd.sdkConfig = nil
			cmd.consumerRestConfig = nil
			cmd.ConsumerKubeconfig = "" // Clear the temporary path for comparison

			if tc.expected != nil {
				if !reflect.DeepEqual(&cmd, tc.expected) {
					t.Errorf("diff: %s", cmp.Diff(&cmd, tc.expected, cmp.AllowUnexported(installCmd{})))
				}
			}
		})
	}
}
