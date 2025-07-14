// Copyright 2025 Upbound Inc.
// All rights reserved

// AI Generated. Human reviewed.

package apiconnector

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

func createMockUpboundContext() *upbound.Context {
	return &upbound.Context{
		Profile: profile.Profile{
			Organization: "test-org",
		},
		Kubecfg: &mockKubeConfig{},
	}
}

type mockKubeConfig struct {
	clientConfig *rest.Config
	clientErr    error
}

func (m *mockKubeConfig) ClientConfig() (*rest.Config, error) {
	if m.clientErr != nil {
		return nil, m.clientErr
	}
	if m.clientConfig == nil {
		return &rest.Config{}, nil
	}
	return m.clientConfig, nil
}

func (m *mockKubeConfig) RawConfig() (clientcmdapi.Config, error) {
	return clientcmdapi.Config{}, nil
}

func (m *mockKubeConfig) Namespace() (string, bool, error) {
	return "default", false, nil
}

func (m *mockKubeConfig) ConfigAccess() clientcmd.ConfigAccess {
	return nil
}

func TestCmd_AfterApply(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		reason      string
		input       installCmd
		extraOpts   map[string]string
		expectedErr error
		expected    *installCmd
	}{
		"InvalidFQCPName": {
			reason: "Should return error for invalid FQCP name",
			input: installCmd{
				FullyQualifiedControlPlaneName: "invalid-format",
			},
			expectedErr: errors.New("control plane name must be in the format: organization-name/upbound-gcp-us-west-1/default/my-control-plane"),
		},
		"InstallValidFQCPName": {
			reason: "Should successfully parse valid FQCP name for install",
			input: installCmd{
				FullyQualifiedControlPlaneName: "test-org/upbound-gcp-us-west-1/default/test-cp",
			},
			expected: &installCmd{
				FullyQualifiedControlPlaneName: "test-org/upbound-gcp-us-west-1/default/test-cp",
				organization:                   "test-org",
				space:                          "upbound-gcp-us-west-1",
				group:                          "default",
				name:                           "test-cp",
				Team:                           "test-cp",
				controlPlaneName:               "test-cp",
				spacesHostname:                 "upbound-gcp-us-west-1.spaces.upbound.io",
				installationNamespace:          "upbound-system",
			},
		},
		"InstallWithCustomName": {
			reason: "Should successfully parse valid FQCP name for install with custom name",
			input: installCmd{
				FullyQualifiedControlPlaneName: "test-org/upbound-gcp-us-west-1/default/test-cp",
				Name:                           "custom-name",
			},
			expected: &installCmd{
				FullyQualifiedControlPlaneName: "test-org/upbound-gcp-us-west-1/default/test-cp",
				organization:                   "test-org",
				space:                          "upbound-gcp-us-west-1",
				group:                          "default",
				name:                           "custom-name",
				Name:                           "custom-name",
				Team:                           "custom-name",
				controlPlaneName:               "test-cp",
				spacesHostname:                 "upbound-gcp-us-west-1.spaces.upbound.io",
				installationNamespace:          "upbound-system",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			upCtx := createMockUpboundContext()

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
			cmd.targetClient = nil
			cmd.spaceClient = nil
			cmd.sdkConfig = nil
			cmd.targetRestConfig = nil

			if tc.expected != nil {
				if !reflect.DeepEqual(&cmd, tc.expected) {
					t.Errorf("diff: %s", cmp.Diff(&cmd, tc.expected, cmp.AllowUnexported(installCmd{})))
				}
			}
		})
	}
}
