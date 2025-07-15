// Copyright 2025 Upbound Inc.
// All rights reserved

package apiconnector

import (
	"context"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/lipgloss"
	"github.com/pterm/pterm"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go"
	authorizationv1alpha1 "github.com/upbound/up-sdk-go/apis/authorization/v1alpha1"
	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

var (
	//nolint:gochecknoglobals // We'd make these consts if we could.
	upboundBrandColor = lipgloss.AdaptiveColor{Light: "#5e3ba5", Dark: "#af7efd"}

	//nolint:gochecknoglobals // We'd make these consts if we could.
	upboundRootStyle = lipgloss.NewStyle().Foreground(upboundBrandColor)
)

var mcpRepoURL = urlMustParse("xpkg.upbound.io/spaces-artifacts") //nolint:gochecknoglobals // Would make this a const if we could.

func init() {
	kruntime.Must(authorizationv1alpha1.AddToScheme(scheme.Scheme))
	kruntime.Must(spacesv1beta1.AddToScheme(scheme.Scheme))
}

const (
	connectorName                = "api-connector"
	defaultInstallationNamespace = "upbound-system"

	spacesHostnameSuffix = ".spaces.upbound.io"

	errReadParametersFile     = "unable to read parameters file"
	errParseInstallParameters = "unable to parse install parameters"
)

// installCmd connects the current cluster to a control plane in an account on
// Upbound.
type installCmd struct {
	parser           install.ParameterParser
	targetClient     client.Client
	targetRestConfig *rest.Config
	spaceClient      client.Client
	sdkConfig        *up.Config

	// name will be used to construct object names in the api layer and target cluster
	name string

	// All below will be used to construct secret name in the target cluster
	// to connect to the control plane.
	group            string
	space            string
	organization     string
	spacesHostname   string
	controlPlaneName string

	installationNamespace string

	// Lifecycle flags
	Upgrade bool   `help:"Upgrade the API Connector if it is already installed."`
	Version string `help:"Version of the API Connector to install. If not provided, the latest, known to CLI, will be installed."`

	// Identity flags
	Team                           string `help:"Team to create the robot in. If not provided, new team will be created."`
	FullyQualifiedControlPlaneName string `arg:""                                                                                                                          help:"Full qualified name of the control plane. If not provided, the name argument value will be used. Example: organization-name/upbound-gcp-us-west-1/default/my-control-plane"`
	Name                           string `help:"Name of the related objects for named connection. If not provided, last segment of the full qualified name will be used."`
	Token                          string `help:"API token used to authenticate. If not provided, a new robot and a token will be created."`

	// Installation flags
	SkipConnection          bool   `help:"Skip connection creation to the control plane. If provided, the connector will be installed without connecting to the control plane."`
	TargetKubeconfig        string `help:"Path to the kubeconfig file for the cluster. If not provided, the current context will be used."`
	TargetKubeconfigContext string `help:"Context to use in the kubeconfig file. If not provided, the current context will be used."`

	ControlPlaneSecretNamespace string `default:"upbound-system"                                                    help:"Namespace of the secret that contains the kubeconfig for a control plane."`
	ControlPlaneSecretName      string `help:"Name of the secret that contains the kubeconfig for a control plane."`

	// Advanced/Developer flags
	HelmDirectory string `help:"Directory to store the Helm chart. If not provided, the default will be used." hidden:"true"`

	install.CommonParams
}

func (c *installCmd) Help() string {
	return `
The 'install' command installs the API Connector into a cluster.

Examples:
    up controlplane api-connector install <` +
		nice("control-plane-name-full-qualified-path") +
		`> --target-kubeconfig <` +
		nice("kubeconfig-path-for-deployment-cluster") +
		`> --token <` +
		nice("api-token") +
		`>
        Installs the API Connector into the cluster and connects it to the control plane 'my-control-plane'.
		Current context must be set to the organization that contains the control plane.

    up controlplane api-connector install <` +
		nice("control-plane-name-full-qualified-path") +
		`> --name <` +
		nice("connection-resources-name") +
		`> --target-kubeconfig <` +
		nice("kubeconfig-path-for-deployment-cluster") +
		`> --token <` +
		nice("api-token") +
		`>
        Installs the API Connector into the cluster and connects it to the control plane 'my-control-plane' with a custom name for the connection resources.
		Current context must be set to the organization that contains the control plane.

   where:
   * ` + nice("control-plane-name-full-qualified-path") + ` is in the format from "up ctx ." command with controlplane name appended.
    Example: ` + nice("organization-name/upbound-gcp-us-west-1/default/my-control-plane") + `
   * ` + nice("kubeconfig-path-for-deployment-cluster") + ` is the path to the kubeconfig file for the cluster.
   * ` + nice("connection-resources-name") + ` is the name of the connection resources to be created.
`
}

// AfterApply sets default values in command after assignment and validation.
func (c *installCmd) AfterApply(_ *kong.Context, upCtx *upbound.Context) error {
	parts := strings.Split(c.FullyQualifiedControlPlaneName, "/")
	if len(parts) != 4 {
		return errors.New("control plane name must be in the format: " + nice("organization-name/upbound-gcp-us-west-1/default/my-control-plane"))
	}
	c.organization = parts[0]
	c.space = parts[1]
	c.group = parts[2]
	c.name = parts[3] // this can be overridden by --name flag below.
	c.controlPlaneName = parts[3]

	if c.Name != "" {
		c.name = c.Name
	}

	if c.Team == "" {
		c.Team = c.name
	}

	if !strings.HasPrefix(c.space, "upbound-") {
		return errors.New("space name must start with 'upbound-'")
	}
	c.spacesHostname = c.space + spacesHostnameSuffix

	c.installationNamespace = defaultInstallationNamespace

	var targetRestConfig *rest.Config
	var err error
	if c.TargetKubeconfig != "" {
		targetRestConfig, err = kube.GetKubeConfig(c.TargetKubeconfig, c.TargetKubeconfigContext)
	} else {
		targetRestConfig, err = upCtx.Kubecfg.ClientConfig()
	}
	if err != nil {
		return err
	}
	c.targetRestConfig = targetRestConfig

	targetKubeClient, err := client.New(targetRestConfig, client.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		return err
	}
	c.targetClient = targetKubeClient

	spaceRestConfig, err := upCtx.Kubecfg.ClientConfig()
	if err != nil {
		return err
	}

	spaceKubeClient, err := client.New(spaceRestConfig, client.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		return err
	}
	c.spaceClient = spaceKubeClient

	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return errors.Wrap(err, "failed to build SDK config")
	}
	c.sdkConfig = cfg

	base := map[string]any{}
	if c.File != nil {
		defer c.File.Close() //nolint:errcheck // Can't do anything useful with this error.
		b, err := io.ReadAll(c.File)
		if err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
		if err := yaml.Unmarshal(b, &base); err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
	}
	c.parser = helm.NewParser(base, c.Set)
	return nil
}

// Run executes the connect command.
func (c *installCmd) Run(p pterm.TextPrinter, upCtx *upbound.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	return c.deploy(ctx, p, upCtx)
}

func (c *installCmd) deploy(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context) error {
	provisioner := newProvisioner(p, c.sdkConfig)
	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseInstallParameters)
	}

	// First and foremost - check if we have a connection with same name already as
	// further action will bork if we have a connection with same name.
	_, err = provisioner.getConnection(ctx, c.targetClient, c.name)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to get connection")
	}

	if err == nil { // we only expect 404 here,
		return errors.New("connection with same name already exists. Please delete the connection first or provide a different name with --name flag")
	}

	// First part is - BYO permissions.
	if c.Token != "" {
		if c.organization == "" {
			return errors.New("organization is required when providing a token")
		}
		// Now this is where we stomp over the BYO permissions.
		// This is not quite nice place for this, but code is bit tricky to get right now.
		provisioner.results.Token = c.Token
		provisioner.results.OrganizationName = c.organization
	} else { // No token provided, so we need to create a robot and a token, and wire things up.
		p.Printfln("Creating a robot & related resources for the cluster %s in the organization %s.", nice(c.name), nice(upCtx.Profile.Organization))

		if err := provisioner.seedOrganizations(ctx, c.organization); err != nil {
			return errors.Wrap(err, "failed to seed organizations")
		}
		if err := provisioner.seedRobots(ctx, c.name); err != nil {
			return errors.Wrap(err, "failed to seed robots")
		}
		if err := provisioner.seedTeams(ctx, c.name); err != nil {
			return errors.Wrap(err, "failed to seed teams")
		}
		if err := provisioner.seedToken(ctx); err != nil {
			return errors.Wrap(err, "failed to seed token")
		}

		namespace, err := upCtx.GetCurrentContextNamespace()
		if err != nil {
			return errors.Wrap(err, "failed to get current context namespace")
		}

		if err := provisioner.seedAccess(ctx, c.spaceClient, c.name, namespace); err != nil {
			return errors.Wrap(err, "failed to seed access")
		}
	}

	installOptions := installOptions{
		name:      c.name,
		namespace: c.installationNamespace,
		version:   c.Version,
		chartPath: c.HelmDirectory,
		params:    params,
		upgrade:   c.Upgrade,
	}

	if err := provisioner.installOrUpgradeConnector(ctx, c.targetRestConfig, installOptions); err != nil {
		return errors.Wrap(err, "failed to install or upgrade")
	}

	// If skip connection is provided, we are done.
	if c.SkipConnection {
		return nil
	}

	if err := provisioner.seedConnectionSecret(ctx, c.targetClient, c.name, c.installationNamespace, c.spacesHostname, c.group, c.controlPlaneName); err != nil {
		return errors.Wrap(err, "failed to seed connection secret")
	}

	if err := provisioner.seedConnection(ctx, c.targetClient, c.name, c.installationNamespace); err != nil {
		return errors.Wrap(err, "failed to seed connection")
	}

	return nil
}

func urlMustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func nice(s string) string {
	return upboundRootStyle.Render(s)
}
