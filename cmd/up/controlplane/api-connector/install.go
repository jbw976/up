// Copyright 2025 Upbound Inc.
// All rights reserved

package apiconnector

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/alecthomas/kong"
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
	"github.com/upbound/up/internal/style"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// nice is a helper function to render a string with the Upbound brand color.
// It basically shortens the name of the function to make it easier to read.
func nice(s string) string {
	return style.UpboundRootStyle.Render(s)
}

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
	UpboundToken                   string `help:"API token used to authenticate. If not provided, a new robot and a token will be created."`

	// Installation flags
	SkipConnection          bool   `help:"Skip connection creation to the control plane. If provided, the connector will be installed without connecting to the control plane."`
	TargetKubeconfig        string `help:"Path to the kubeconfig file for the consumer cluster. If not provided, the default kubeconfig resolution will be used."`
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
		return fmt.Errorf("failed to get target kubeconfig: %w", err)
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
		return fmt.Errorf("failed to get space kubeconfig: %w", err)
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
func (c *installCmd) Run(p pterm.TextPrinter, upCtx *upbound.Context, printer upterm.ObjectPrinter) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	return c.deploy(ctx, p, upCtx, printer)
}

func (c *installCmd) deploy(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context, printer upterm.ObjectPrinter) error {
	stepSpinner := upterm.CheckmarkSuccessSpinner.WithShowTimer(true)

	provisioner := newProvisioner(c.sdkConfig, p, printer)
	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseInstallParameters)
	}

	totalSteps := 5

	// Step 1: Check if connection already exists. Fail early if it does.
	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter("Checking if connection already exists", 1, totalSteps),
		stepSpinner,
		func() error {
			// First and foremost - check if we have a connection with same name already as
			// further action will bork if we have a connection with same name.
			_, err = provisioner.getConnection(ctx, c.targetClient, c.name)
			if err != nil && !apierrors.IsNotFound(err) {
				return errors.Wrap(err, "failed to get connection")
			}
			return nil
		},
		printer,
	); err != nil {
		return err
	}

	// Step 2: If token is provided, we use it. Otherwise, we create a robot and a token, and wire things up.
	if c.UpboundToken != "" {
		if c.organization == "" {
			return errors.New("organization is required when providing a token")
		}
		// Now this is where we stomp over the BYO permissions.
		// This is not quite nice place for this, but code is bit tricky to get right now.
		provisioner.results.Token = c.UpboundToken
		provisioner.results.OrganizationName = c.organization
	} else { // No token provided, so we need to create a robot and a token, and wire things up.
		// Step 2.1: Read organization configuration.
		p.Printfln("Creating a robot & related resources for the cluster %s in the organization %s.", nice(c.name), nice(upCtx.Profile.Organization))
		if err := upterm.WrapWithSuccessSpinner(
			upterm.StepCounter("Reading organization configuration", 2, totalSteps),
			stepSpinner,
			func() error {
				if err := provisioner.seedOrganizations(ctx, c.organization); err != nil {
					return errors.Wrap(err, "failed to seed organizations")
				}
				return nil
			},
			printer,
		); err != nil {
			return err
		}

		// Step 2.2: Create a robot and a token.
		if err := upterm.WrapWithSuccessSpinner(
			upterm.StepCounter("Creating robot", 3, totalSteps),
			stepSpinner,
			func() error {
				if err := provisioner.seedRobots(ctx, c.name); err != nil {
					return errors.Wrap(err, "failed to seed robots")
				}
				return nil
			},
			printer,
		); err != nil {
			return err
		}

		// Step 2.3: Create a token.
		if err := upterm.WrapWithSuccessSpinner(
			upterm.StepCounter("Creating token", 3, totalSteps),
			stepSpinner,
			func() error {
				return provisioner.seedToken(ctx)
			},
			printer,
		); err != nil {
			return err
		}

		// Step 2.4: Seed access to the namespace.
		if err := upterm.WrapWithSuccessSpinner(
			upterm.StepCounter("Creating access in the control plane", 4, totalSteps),
			stepSpinner,
			func() error {
				namespace, err := upCtx.GetCurrentContextNamespace()
				if err != nil {
					return errors.Wrap(err, "failed to get current context namespace")
				}
				return provisioner.seedAccess(ctx, c.spaceClient, namespace)
			},
			printer,
		); err != nil {
			return err
		}
	}

	// Step 3: Install the connector.
	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter("Installing the connector", 5, totalSteps),
		stepSpinner,
		func() error {
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
			return nil
		},
		printer,
	); err != nil {
		return err
	}

	// If skip connection is provided, we are done.
	if c.SkipConnection {
		return nil
	}

	// Step 4: Create a connection secret.
	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter("Creating connection secret", 5, totalSteps),
		stepSpinner,
		func() error {
			return provisioner.seedConnectionSecret(ctx, c.targetClient, c.name, c.installationNamespace, c.spacesHostname, c.group, c.controlPlaneName)
		},
		printer,
	); err != nil {
		return err
	}

	// Step 5: Create a connection.
	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter("Creating connection", 5, totalSteps),
		stepSpinner,
		func() error {
			return provisioner.seedConnection(ctx, c.targetClient, c.name, c.installationNamespace)
		},
		printer,
	); err != nil {
		return err
	}

	p.Printfln("API Connector installed")
	return nil
}

func urlMustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
