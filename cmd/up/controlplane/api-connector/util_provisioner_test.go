// Copyright 2025 Upbound Inc.
// All rights reserved

package apiconnector

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/pterm/pterm"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up-sdk-go"
	upboundfake "github.com/upbound/up-sdk-go/fake"
	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/robots"
)

func TestProvisioner_SeedOrganizations(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		reason          string
		provisioner     func() *provisioner
		organizationArg string

		// These should be set if list is called and worked.
		expectedOrganizationID    uint
		expectedOrganizationName  string
		expectedOrganizationIDStr string

		expectedError error
	}{
		"SuccessfulRetrieveOrganizations": {
			organizationArg: "test-org",
			provisioner: func() *provisioner {
				orgs := []organizations.Organization{
					{
						ID:   123,
						Name: "test-org",
					},
				}
				accResp, err := json.Marshal(orgs)
				if err != nil {
					t.Fatalf("Failed to marshal organizations: %v", err)
				}
				return &provisioner{
					organizationsClient: organizations.NewClient(&up.Config{
						Client: &upboundfake.MockClient{
							MockNewRequest: upboundfake.NewMockNewRequestFn(nil, nil),
							MockDo: func(_ *http.Request, obj interface{}) error {
								return json.Unmarshal(accResp, &obj)
							},
						},
					}),
				}
			},
			expectedOrganizationID:    123,
			expectedOrganizationName:  "test-org",
			expectedOrganizationIDStr: "123",
		},
		"OrganizationNotFound": {
			organizationArg: "test-org",
			provisioner: func() *provisioner {
				return &provisioner{
					organizationsClient: organizations.NewClient(&up.Config{
						Client: &upboundfake.MockClient{
							MockNewRequest: upboundfake.NewMockNewRequestFn(nil, nil),
							MockDo: func(_ *http.Request, obj interface{}) error {
								return json.Unmarshal([]byte("[]"), &obj)
							},
						},
					}),
				}
			},
			expectedError: errors.New("organization not found"),
		},
		"ErrorRetrievingOrganizations": {
			organizationArg: "test-org",
			provisioner: func() *provisioner {
				return &provisioner{
					organizationsClient: organizations.NewClient(&up.Config{
						Client: &upboundfake.MockClient{
							MockNewRequest: upboundfake.NewMockNewRequestFn(nil, nil),
							MockDo: func(_ *http.Request, _ interface{}) error {
								return errors.New("error retrieving organizations")
							},
						},
					}),
				}
			},
			expectedError: errors.New("error retrieving organizations"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			provisioner := tc.provisioner()
			err := provisioner.seedOrganizations(t.Context(), tc.organizationArg)
			if tc.expectedError != nil && err == nil {
				t.Error("expected error but got none")
			}
			if tc.expectedError == nil && err != nil {
				t.Errorf("expected no error but got: '%v'", err)
			}
			if tc.expectedError != nil && err != nil {
				if !strings.Contains(err.Error(), tc.expectedError.Error()) {
					t.Errorf("expected error '%v' but got '%v'", tc.expectedError, err)
				}
			}

			if tc.expectedOrganizationID != provisioner.results.OrganizationID {
				t.Errorf("expected organization ID to be '%d' but got '%d'", tc.expectedOrganizationID, provisioner.results.OrganizationID)
			}
			if tc.expectedOrganizationName != provisioner.results.OrganizationName {
				t.Errorf("expected organization name to be '%s' but got '%s'", tc.expectedOrganizationName, provisioner.results.OrganizationName)
			}
			if tc.expectedOrganizationIDStr != provisioner.results.OrganizationIDStr {
				t.Errorf("expected organization ID string to be '%s' but got '%s'", tc.expectedOrganizationIDStr, provisioner.results.OrganizationIDStr)
			}
		})
	}
}

func TestProvisioner_SeedRobots(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		reason      string
		provisioner func() *provisioner
		clusterName string

		// These should be set if list is called and worked.
		expectedOrganizationRobots []organizations.Robot
		expectedRobot              organizations.Robot

		expectedError error
	}{
		"SuccessfulUseExistingRobot": {
			clusterName: "test-cluster",
			provisioner: func() *provisioner {
				robotID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
				robots := []organizations.Robot{
					{
						ID:   robotID,
						Name: "test-cluster",
					},
				}
				robotsResp, err := json.Marshal(robots)
				if err != nil {
					t.Fatalf("Failed to marshal robots: %v", err)
				}
				return &provisioner{
					organizationsClient: organizations.NewClient(&up.Config{
						Client: &upboundfake.MockClient{
							MockNewRequest: upboundfake.NewMockNewRequestFn(nil, nil),
							MockDo: func(_ *http.Request, obj interface{}) error {
								return json.Unmarshal(robotsResp, &obj)
							},
						},
					}),
					printer: &pterm.DefaultBasicText,
					results: provisionerResults{
						OrganizationID:   123,
						OrganizationName: "test-org",
					},
				}
			},
			expectedOrganizationRobots: []organizations.Robot{
				{
					ID:   uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
					Name: "test-cluster",
				},
			},
			expectedRobot: organizations.Robot{
				ID:   uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
				Name: "test-cluster",
			},
		},
		"SuccessfulCreateRobot": {
			clusterName: "test-cluster-new",
			provisioner: func() *provisioner {
				exitingRobotID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174001")
				robotsList := []organizations.Robot{
					{
						ID:   exitingRobotID,
						Name: "test-cluster-old",
					},
				}
				robotsResp, err := json.Marshal(robotsList)
				if err != nil {
					t.Fatalf("Failed to marshal robots: %v", err)
				}
				newRobotID := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
				newRobot := robots.RobotResponse{
					DataSet: common.DataSet{
						ID: newRobotID,
					},
				}
				newRobotResp, err := json.Marshal(newRobot)
				if err != nil {
					t.Fatalf("Failed to marshal new robot: %v", err)
				}
				return &provisioner{
					organizationsClient: organizations.NewClient(&up.Config{
						Client: &upboundfake.MockClient{
							MockNewRequest: upboundfake.NewMockNewRequestFn(nil, nil),
							MockDo: func(_ *http.Request, obj interface{}) error {
								return json.Unmarshal(robotsResp, &obj)
							},
						},
					}),
					robotsClient: robots.NewClient(&up.Config{
						Client: &upboundfake.MockClient{
							MockNewRequest: upboundfake.NewMockNewRequestFn(nil, nil),
							MockDo: func(_ *http.Request, obj interface{}) error {
								return json.Unmarshal(newRobotResp, &obj)
							},
						},
					}),
					printer: &pterm.DefaultBasicText,
					results: provisionerResults{
						OrganizationID:   123,
						OrganizationName: "test-org",
					},
				}
			},
			expectedOrganizationRobots: []organizations.Robot{
				{
					ID:   uuid.MustParse("123e4567-e89b-12d3-a456-426614174001"),
					Name: "test-cluster-old",
				},
			},
			expectedRobot: organizations.Robot{
				ID: uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			provisioner := tc.provisioner()
			err := provisioner.seedRobots(t.Context(), tc.clusterName)
			t.Logf("Debug: Robot ID after seedRobots: %s", provisioner.results.Robot.ID.String())
			if tc.expectedError != nil && err == nil {
				t.Error("expected error but got none")
			}
			if tc.expectedError == nil && err != nil {
				t.Errorf("expected no error but got: '%v'", err)
			}
			if tc.expectedError != nil && err != nil {
				if !strings.Contains(err.Error(), tc.expectedError.Error()) {
					t.Errorf("expected error '%v' but got '%v'", tc.expectedError, err)
				}
			}

			if len(tc.expectedOrganizationRobots) != len(provisioner.results.OrganizationRobots) {
				t.Errorf("expected %d organization robots but got %d", len(tc.expectedOrganizationRobots), len(provisioner.results.OrganizationRobots))
			}

			for i, expectedRobot := range tc.expectedOrganizationRobots {
				if i < len(provisioner.results.OrganizationRobots) {
					if expectedRobot.ID != provisioner.results.OrganizationRobots[i].ID {
						t.Errorf("expected robot ID to be '%s' but got '%s'", expectedRobot.ID, provisioner.results.OrganizationRobots[i].ID)
					}
					if expectedRobot.Name != provisioner.results.OrganizationRobots[i].Name {
						t.Errorf("expected robot name to be '%s' but got '%s'", expectedRobot.Name, provisioner.results.OrganizationRobots[i].Name)
					}
				}
			}

			if tc.expectedError == nil {
				if tc.expectedRobot.ID != provisioner.results.Robot.ID {
					t.Errorf("expected robot ID to be '%s' but got '%s'", tc.expectedRobot.ID, provisioner.results.Robot.ID)
				}
			}
		})
	}
}
