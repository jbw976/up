// Copyright 2025 Upbound Inc.
// All rights reserved

package apiconnector

import (
	"context"
	"encoding/base64"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pterm/pterm"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go"
	authorizationv1alpha1 "github.com/upbound/up-sdk-go/apis/authorization/v1alpha1"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up-sdk-go/service/teams"
	"github.com/upbound/up-sdk-go/service/tokens"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/version"
)

// provisioner is a helper to provision the resources needed for the API Connector.
// It takes care of helm install, rbac, and connection secret and connections.
// All this could live in the install.go, but it is nice to have it separated
// and makes readability bit easier.

const (
	labelConnectorOwned = "connect.upbound.io/connector-secret"
)

type provisionerResults struct {
	// These are read from api and stored for later use.
	OrganizationID    uint
	OrganizationIDStr string

	OrganizationRobots []organizations.Robot
	OrganizationTeams  []organizations.Team

	// These will be used to setup access.
	Robot            organizations.Robot
	Team             organizations.Team
	OrganizationName string
	Token            string
}

type provisioner struct {
	robotsClient        *robots.Client
	organizationsClient *organizations.Client
	teamsClient         *teams.Client
	tokensClient        *tokens.Client

	printer pterm.TextPrinter

	// These are used to store results for later use.
	results provisionerResults
}

func newProvisioner(p pterm.TextPrinter, cfg *up.Config) *provisioner {
	return &provisioner{
		robotsClient:        robots.NewClient(cfg),
		organizationsClient: organizations.NewClient(cfg),
		teamsClient:         teams.NewClient(cfg),
		tokensClient:        tokens.NewClient(cfg),
		printer:             p,
	}
}

func (p *provisioner) seedOrganizations(ctx context.Context, organizationArg string) error {
	orgs, err := p.organizationsClient.List(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get organizations")
	}

	for _, org := range orgs {
		if org.Name == organizationArg {
			p.results.OrganizationID = org.ID
			p.results.OrganizationName = org.Name
			p.results.OrganizationIDStr = strconv.FormatUint(uint64(org.ID), 10)
			break
		}
	}

	if p.results.OrganizationID == 0 {
		return errors.New("organization not found")
	}

	return nil
}

func (p *provisioner) seedRobots(ctx context.Context, clusterName string) error {
	if p.results.OrganizationID == 0 {
		return errors.New("programmer error: seedOrganizations should have been called first")
	}
	robotsList, err := p.organizationsClient.ListRobots(ctx, p.results.OrganizationID)
	if err != nil {
		return errors.Wrap(err, "failed to get robots")
	}
	p.results.OrganizationRobots = robotsList

	alreadyExists := false
	for _, robot := range p.results.OrganizationRobots {
		if robot.Name == clusterName {
			alreadyExists = true
			p.results.Robot = robot
			p.printer.Printfln("Robot %s already exists in the organization %s. Will use existing robot.", nice(clusterName), nice(p.results.OrganizationName))
			break
		}
	}

	if !alreadyExists {
		p.printer.Printfln("Creating a robot for the cluster %s in the organization %s.", nice(clusterName), nice(p.results.OrganizationName))
		payload := &robots.RobotCreateParameters{
			Attributes: robots.RobotAttributes{
				Name:        clusterName,
				Description: "API Connector robot for " + p.results.OrganizationName,
			},
			Relationships: robots.RobotRelationships{
				Owner: robots.RobotOwner{
					Data: robots.RobotOwnerData{
						Type: robots.RobotOwnerOrganization,
						ID:   strconv.FormatUint(uint64(p.results.OrganizationID), 10),
					},
				},
			},
		}
		robot, err := p.robotsClient.Create(ctx, payload)
		if err != nil {
			return errors.Wrap(err, "failed to create robot")
		}
		p.results.Robot = organizations.Robot{
			ID: robot.ID,
		}
	}
	return nil
}

func (p *provisioner) seedTeams(ctx context.Context, clusterName string) error {
	if p.results.Robot.ID.String() == "" {
		return errors.New("programmer error: seedRobots should have been called first")
	}
	teamsList, err := p.organizationsClient.ListTeams(ctx, p.results.OrganizationID)
	if err != nil {
		return errors.Wrap(err, "failed to get teams")
	}
	p.results.OrganizationTeams = teamsList

	alreadyExists := false
	for _, team := range teamsList {
		if team.Name == clusterName {
			alreadyExists = true
			p.results.Team = team
			p.printer.Printfln("Team %s already exists in the organization %s. Will use existing team.", nice(clusterName), nice(p.results.OrganizationName))
			break
		}
	}

	if !alreadyExists {
		p.printer.Printfln("Creating a team for the cluster %s in the organization %s.", nice(clusterName), nice(p.results.OrganizationName))
		payload := &teams.TeamCreateParameters{
			Name:           clusterName,
			OrganizationID: p.results.OrganizationID,
		}
		team, err := p.teamsClient.Create(ctx, payload)
		if err != nil {
			return errors.Wrap(err, "failed to create team")
		}
		p.results.Team = organizations.Team{
			ID: team.ID,
		}
	}

	// We dont need to check if its exists as api is idempotent.
	err = p.robotsClient.CreateTeamMembership(ctx, p.results.Robot.ID, &robots.RobotTeamMembershipResourceIdentifier{
		Type: robots.RobotTeamMembershipTypeTeam,
		ID:   p.results.Team.ID.String(),
	})
	if err != nil {
		return errors.Wrap(err, "failed to create team membership")
	}
	return nil
}

func (p *provisioner) seedToken(ctx context.Context) error {
	if p.results.Robot.ID.String() == "" {
		return errors.New("programmer error: seedRobots should have been called first")
	}
	token, err := p.tokensClient.Create(ctx, &tokens.TokenCreateParameters{
		Attributes: tokens.TokenAttributes{
			Name: "api-connector-token-" + time.Now().Format("20060102150405"),
		},
		Relationships: tokens.TokenRelationships{
			Owner: tokens.TokenOwner{
				Data: tokens.TokenOwnerData{
					Type: tokens.TokenOwnerRobot,
					ID:   p.results.Robot.ID.String(),
				},
			},
		},
	})
	if err != nil {
		if strings.Contains(err.Error(), "Conflict") {
			p.printer.Printfln("Token already exists for the robot %s. Either provide it with --token flag or delete and rerun the command.", nice(p.results.Robot.Name))
			return errors.New("token already exists")
		}
		return errors.Wrap(err, "failed to create token")
	}
	jwtToken, ok := token.DataSet.Meta["jwt"]
	if !ok {
		return errors.New("failed to get JWT token")
	}
	tokenString, ok := jwtToken.(string)
	if !ok {
		return errors.New("failed to get JWT token")
	}
	p.results.Token = tokenString
	return nil
}

func (p *provisioner) seedAccess(ctx context.Context, spacesClient client.Client, name, namespace string) error {
	if p.results.Team.ID.String() == "" {
		return errors.New("programmer error: seedTeams should have been called first")
	}

	orb := authorizationv1alpha1.ObjectRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-api-connector",
			Namespace: namespace,
		},
		Spec: authorizationv1alpha1.ObjectRoleBindingSpec{
			Subjects: []authorizationv1alpha1.SubjectBinding{
				{
					Kind: "UpboundTeam",
					Name: p.results.Team.ID.String(),
					Role: "admin",
				},
			},
			Object: authorizationv1alpha1.Object{
				APIGroup: "core",
				Resource: "namespaces",
				Name:     namespace,
			},
		},
	}

	current := &authorizationv1alpha1.ObjectRoleBinding{}
	err := spacesClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name + "-api-connector",
	}, current)
	if err != nil {
		if apierrors.IsNotFound(err) {
			p.printer.Printfln("No existing rbac found for the namespace %s. Will create a new one.", nice(namespace))
			return spacesClient.Create(ctx, &orb)
		}
		return errors.Wrap(err, "failed to get current rbac")
	}

	var found bool
	for _, subject := range current.Spec.Subjects {
		if subject.Kind == "UpboundTeam" && subject.Name == p.results.Team.ID.String() {
			p.printer.Printfln("Team %s already has access to the namespace %s.", nice(p.results.Team.Name), nice(namespace))
			found = true
			break
		}
	}
	if !found {
		p.printer.Printfln("Team %s does not have access to the namespace %s. Will add it.", p.results.Team.Name, namespace)
		current.Spec.Subjects = append(current.Spec.Subjects, authorizationv1alpha1.SubjectBinding{
			Kind: "UpboundTeam",
			Name: p.results.Team.ID.String(),
			Role: "admin",
		})
		err = spacesClient.Update(ctx, current)
		if err != nil {
			return errors.Wrap(err, "failed to update rbac")
		}
	}
	return nil
}

func (p *provisioner) seedConnectionSecret(ctx context.Context, targetClient client.Client, name, namespace, spacesBaseURL, groupName, controlPlaneName string) error {
	if p.results.Token == "" {
		return errors.New("programmer error: seedToken should have been called first")
	}

	gvk := schema.GroupVersionKind{
		Version: "v1",
		Kind:    "Secret",
	}
	secret := unstructured.Unstructured{}
	secret.SetGroupVersionKind(gvk)
	secret.SetName(name)
	secret.SetNamespace(namespace)
	secret.SetLabels(map[string]string{
		labelConnectorOwned: "true",
	})

	body := map[string]any{
		"token":                 base64Encode(p.results.Token),
		"controlPlaneGroupName": base64Encode(groupName),
		"controlPlaneName":      base64Encode(controlPlaneName),
		"organization":          base64Encode(p.results.OrganizationName),
		"spacesBaseURL":         base64Encode(spacesBaseURL),
	}

	err := targetClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			secret.Object["data"] = body
			return targetClient.Create(ctx, &secret)
		}
		return errors.Wrap(err, "failed to get secret")
	}

	secret.Object["data"] = body
	return targetClient.Update(ctx, &secret)
}

func (p *provisioner) deleteConnectionSecrets(ctx context.Context, targetClient client.Client, namespace string) error {
	gvk := schema.GroupVersionKind{
		Version: "v1",
		Kind:    "Secret",
	}
	secret := unstructured.Unstructured{}
	secret.SetGroupVersionKind(gvk)
	return targetClient.DeleteAllOf(ctx, &secret, client.InNamespace(namespace), client.MatchingLabels{
		labelConnectorOwned: "true",
	})
}

func (p *provisioner) deleteConnections(ctx context.Context, targetClient client.Client, namespace string) error {
	gvk := schema.GroupVersionKind{
		Group:   "connect.upbound.io",
		Version: "v1alpha1",
		Kind:    "ClusterConnection",
	}
	connection := unstructured.Unstructured{}
	connection.SetGroupVersionKind(gvk)
	connection.SetNamespace(namespace)
	return targetClient.DeleteAllOf(ctx, &connection)
}

func (p *provisioner) seedConnection(ctx context.Context, targetClient client.Client, name, namespace string) error {
	if p.results.Token == "" {
		return errors.New("programmer error: seedToken should have been called first")
	}

	gvk := schema.GroupVersionKind{
		Group:   "connect.upbound.io",
		Version: "v1alpha1",
		Kind:    "ClusterConnection",
	}

	connection := unstructured.Unstructured{}
	connection.SetGroupVersionKind(gvk)
	connection.SetName(name)
	connection.SetNamespace(namespace)
	connection.SetLabels(map[string]string{
		labelConnectorOwned: "true",
	})

	body := map[string]any{
		"secretRef": map[string]any{
			"kind":      "UpboundRobotToken",
			"name":      name,
			"namespace": namespace,
		},
		"crdManagement": map[string]any{
			"pullBehavior": "Pull",
		},
	}

	err := targetClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &connection)
	if err != nil {
		if apierrors.IsNotFound(err) {
			connection.Object["spec"] = body
			return targetClient.Create(ctx, &connection)
		}
		return errors.Wrap(err, "failed to get connection")
	}

	// Its an update
	connection.Object["spec"] = body
	return targetClient.Update(ctx, &connection)
}

func (p *provisioner) getConnection(ctx context.Context, targetClient client.Client, name string) (*unstructured.Unstructured, error) {
	gvk := schema.GroupVersionKind{
		Group:   "connect.upbound.io",
		Version: "v1alpha1",
		Kind:    "ClusterConnection",
	}

	connection := unstructured.Unstructured{}
	connection.SetGroupVersionKind(gvk)
	connection.SetName(name)

	err := targetClient.Get(ctx, client.ObjectKey{
		Name: name,
	}, &connection)
	if apierrors.IsNotFound(err) {
		return nil, err
	}
	return &connection, nil
}

type installOptions struct {
	name      string
	namespace string
	version   string
	chartPath string
	params    map[string]any
	upgrade   bool
}

func (p *provisioner) uninstallConnector(_ context.Context, targetRestConfig *rest.Config, o installOptions) error {
	opts := []helm.InstallerModifierFn{
		helm.WithNamespace(o.namespace),
		helm.Wait(),
	}

	mgr, err := helm.NewManager(targetRestConfig, connectorName, *mcpRepoURL, opts...)
	if err != nil {
		return err
	}

	return mgr.Uninstall()
}

func (p *provisioner) installOrUpgradeConnector(_ context.Context, targetRestConfig *rest.Config, o installOptions) error {
	if p.results.Token == "" {
		return errors.New("programmer error: seedToken should have been called first")
	}

	opts := []helm.InstallerModifierFn{
		helm.WithNamespace(o.namespace),
		helm.CreateNamespace(true),
		helm.Wait(),
	}

	// Add custom cache directory if provided
	if o.chartPath != "" {
		chart, err := os.Open(o.chartPath)
		if err != nil {
			return errors.Wrap(err, "failed to open chart")
		}
		opts = append(opts, helm.WithChart(chart))
	}

	mgr, err := helm.NewManager(targetRestConfig, connectorName, *mcpRepoURL, opts...)
	if err != nil {
		return err
	}

	cliDesiredVersion, err := mgr.GetCurrentVersion()
	if o.version != "" {
		p.printer.Printfln("Version flag provided. Using version %s.", nice(o.version))
		cliDesiredVersion = o.version
	}
	if err != nil {
		// error means that the connector is not installed
		p.printer.Printfln("Installing %s to %s.", nice(connectorName), nice(o.namespace))
		p.printer.Printfln("Using version %s. This may take a few minutes.", nice(cliDesiredVersion))
		if err = mgr.Install(cliDesiredVersion, o.params); err != nil {
			return err
		}
		return nil
	}
	// We already have the connector installed. Moving into the upgrade logic.
	switch {
	case cliDesiredVersion == version.APIConnectorVersion():
		p.printer.Printfln("API Connector is already installed. And matches the current known version. Skipping installation. Use --version to install a different version.")
		return nil
	case cliDesiredVersion != version.APIConnectorVersion() && o.upgrade:
		p.printer.Printfln("Upgrading API Connector from %s to %s.", nice(cliDesiredVersion), nice(version.APIConnectorVersion()))
		if err = mgr.Upgrade(cliDesiredVersion, o.params); err != nil {
			return err
		}
	default:
		p.printer.Printfln("API Connector is already installed. But does not match the current known version. Skipping installation. Use --upgrade to upgrade the connector.")
		return nil
	}

	p.printer.Printfln("Connected to the control plane %s.", nice(o.name))
	p.printer.Println("See connection status with the following command: \n\n$ kubectl get clusterconnections")

	return nil
}

func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
