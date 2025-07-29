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
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up-sdk-go"
	authorizationv1alpha1 "github.com/upbound/up-sdk-go/apis/authorization/v1alpha1"
	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	ctxcmd "github.com/upbound/up/cmd/up/ctx"
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
	parser             install.ParameterParser
	consumerClient     client.Client
	consumerRestConfig *rest.Config

	spaceClient client.Client
	sdkConfig   *up.Config

	// All below will be used to construct secret, object names in the consumer
	// cluster to connect to the control plane. These are derived from the exported variables
	// below, so we don't loose the original values and have to pass them around.

	name             string
	group            string
	space            string
	organization     string
	spacesHostname   string
	controlPlaneName string

	// Lifecycle flags
	Upgrade bool   `help:"Upgrade or downgrade the API Connector to --version, even if it is already installed."`
	Version string `help:"Version of the API Connector to install. If not provided, the latest, known to CLI, will be installed."`

	// Identity flags
	Name         string `help:"Name of the related objects for named connection. If not provided, control plane name will be used with api-connector prefix. "`
	UpboundToken string `help:"API token used to authenticate to the provider control plane. Mutually exclusive with --robot-name."`

	// Installation flags
	SkipConnection     bool   `help:"Skip secret and connection initialization to the control plane. If provided, the connector will be installed without connecting to the control plane."`
	ConsumerKubeconfig string `help:"Path to the kubeconfig file for the consumer cluster. If not provided, the default kubeconfig resolution will be used."`
	ConsumerContext    string `help:"Context to use in the kubeconfig file. If not provided, the current context will be used."`

	// Advanced/Developer flags
	HelmDirectory string `help:"Directory to store the Helm chart. If not provided, the default will be used." hidden:"true"`

	install.CommonParams
}

func (c *installCmd) Help() string {
	return `
The 'install' command installs the API Connector into a consumer cluster.

Note: API Connector is a preview feature. The feature is under active development and subject to breaking changes. Use for testing and evaluation purposes only.

Examples:
    up controlplane api-connector install ` +
		`--consumer-kubeconfig <` +
		nice("kubeconfig-path-for-consumer-cluster") +
		`>
        Installs the API Connector into the consumer cluster and connects it to the control plane which 'up ctx' is set to.

    up controlplane api-connector install ` +
		`--consumer-kubeconfig <` +
		nice("kubeconfig-path-for-consumer-cluster") +
		`> --robot-name <` +
		nice("upbound-robot-name") +
		`>
        Installs the API Connector into the cluster, connects it to the control plane which 'up ctx' is set to, and uses the provided robot name for authentication.

	up controlplane api-connector install ` +
		`--consumer-kubeconfig <` +
		nice("kubeconfig-path-for-consumer-cluster") +
		`> --skip-connection

        Installs the API Connector into the cluster, but does not provision any ClusterConnection resource, nor create any robot.
`
}

// AfterApply sets default values in command after assignment and validation.
func (c *installCmd) AfterApply(_ *kong.Context, upCtx *upbound.Context) error {
	// Check if we match current context:
	po := clientcmd.NewDefaultPathOptions()
	conf, err := po.GetStartingConfig()
	if err != nil {
		return err
	}
	initialState, err := ctxcmd.DeriveState(context.Background(), upCtx, conf, kube.GetIngressHost)
	if err != nil {
		return err
	}

	parts := strings.Split(initialState.Breadcrumbs().String(), "/")
	if len(parts) != 4 {
		return errors.New("current context must be set to a control plane (expected format: organization/space/group/controlplane)")
	}

	c.organization = parts[0]
	c.space = parts[1]
	c.group = parts[2]
	c.controlPlaneName = parts[3]

	if c.Name != "" {
		c.name = c.Name
	} else {
		c.name = fmt.Sprintf("api-connector-%s", c.controlPlaneName)
	}

	if !strings.HasPrefix(c.space, "upbound-") {
		return errors.New("space name must start with 'upbound-'")
	}

	// TODO(mjudeikis): Once "spaces" arg is configurable this will need to be updated.
	c.spacesHostname = "https://" + c.space + spacesHostnameSuffix

	// validate if user by mistake provided upbound context for consumer cluster
	var consumerRestConfig *rest.Config
	consumerConfigLoader := clientcmd.ClientConfigLoadingRules{
		ExplicitPath: c.ConsumerKubeconfig,
	}

	consumerConfig, err := consumerConfigLoader.Load()
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	if consumerConfig.CurrentContext == "upbound" && c.ConsumerContext == "" {
		return errors.New("cannot use upbound context for consumer cluster")
	}

	consumerRestConfig, err = kube.GetKubeConfigWithContext(c.ConsumerKubeconfig, c.ConsumerContext)
	if err != nil {
		return fmt.Errorf("failed to get target kubeconfig: %w", err)
	}
	c.consumerRestConfig = consumerRestConfig

	consumerKubeClient, err := client.New(consumerRestConfig, client.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		return err
	}
	c.consumerClient = consumerKubeClient

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

func (c *installCmd) deploy(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context, printer upterm.ObjectPrinter) error { //nolint:gocognit // its a state machine
	stepSpinner := upterm.CheckmarkSuccessSpinner.WithShowTimer(true)

	provisioner := newProvisioner(c.sdkConfig, p, printer)
	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseInstallParameters)
	}

	totalSteps := 5

	if !c.SkipConnection {
		// Step 1: Check if connection already exists. Fail early if it does.
		if err := upterm.WrapWithSuccessSpinner(
			upterm.StepCounter("Checking if connection already exists", 1, totalSteps),
			stepSpinner,
			func() error {
				// First and foremost - check if we have a connection with same name already as
				// further action will bork if we have a connection with same name.
				_, err = provisioner.getConnection(ctx, c.consumerClient, c.controlPlaneName)
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
			p.Printfln("Creating a robot named %s in the organization %s.", nice(c.name), nice(upCtx.Profile.Organization))
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
					return provisioner.seedAccess(ctx, c.spaceClient, c.controlPlaneName)
				},
				printer,
			); err != nil {
				return err
			}
		}
	}

	// Step 3: Install the connector.
	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter("Installing the connector", 5, totalSteps),
		stepSpinner,
		func() error {
			installOptions := installOptions{
				namespace: defaultInstallationNamespace,
				version:   c.Version,
				chartPath: c.HelmDirectory,
				params:    params,
				upgrade:   c.Upgrade,
			}

			if err := provisioner.installOrUpgradeConnector(ctx, c.consumerRestConfig, installOptions); err != nil {
				return errors.Wrap(err, "failed to install or upgrade")
			}

			return nil
		},
		printer,
	); err != nil {
		return err
	}

	p.Printfln("API Connector installed")

	// If skip connection is provided, we are done.
	if c.SkipConnection {
		return nil
	}

	// Step 4: Create a connection secret.
	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter("Creating connection secret", 5, totalSteps),
		stepSpinner,
		func() error {
			return provisioner.seedConnectionSecret(ctx, c.consumerClient, c.controlPlaneName, defaultInstallationNamespace, c.spacesHostname, c.group, c.controlPlaneName)
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
			return provisioner.seedConnection(ctx, c.consumerClient, c.controlPlaneName, defaultInstallationNamespace)
		},
		printer,
	); err != nil {
		return err
	}

	p.Printfln("Connected to the control plane %s.", nice(c.controlPlaneName))
	p.Println("See connection status with the following command: \n\n$ kubectl get clusterconnections")

	return nil
}

func urlMustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
