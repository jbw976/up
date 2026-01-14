// Copyright 2025 Upbound Inc.
// All rights reserved

package space

import (
	"context"
	"fmt"
	"io"
	"maps"
	"strings"

	"github.com/Masterminds/semver/v3"
	"helm.sh/helm/v3/pkg/chart"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	upboundv1alpha1 "github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
	"github.com/upbound/up/cmd/up/space/defaults"
	spacefeature "github.com/upbound/up/cmd/up/space/features"
	"github.com/upbound/up/cmd/up/space/prerequisites"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/registry"
	"github.com/upbound/up/internal/registry/pullsecret"
	"github.com/upbound/up/internal/resources"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/version"
)

const (
	hcGroup          = "internal.spaces.upbound.io"
	hcVersion        = "v1alpha1"
	hcResourcePlural = "xhostclusters"
)

//nolint:gochecknoglobals // global variables for space initialization.
var (
	watcherTimeout int64 = 600

	hostclusterGVR = schema.GroupVersionResource{
		Group:    hcGroup,
		Version:  hcVersion,
		Resource: hcResourcePlural,
	}

	defaultAcct = "disconnected"
)

const (
	defaultImagePullSecret = "upbound-pull-secret"
	ns                     = "upbound-system"

	errReadParametersFile     = "unable to read parameters file"
	errParseInstallParameters = "unable to parse install parameters"
	errCreateImagePullSecret  = "failed to create image pull secret"
	errCreateSpace            = "failed to create Space"
)

// initCmd installs Upbound Spaces.
type initCmd struct {
	upbound.RequiresContext
	install.CommonParams

	Registry registry.AuthorizedFlags `embed:""`

	Version       string `arg:""                                           help:"Upbound Spaces version to install."`
	Yes           bool   `help:"Answer yes to all questions"               name:"yes"                                type:"bool"`
	PublicIngress bool   `help:"For AKS,EKS,GKE expose ingress publically" name:"public-ingress"                     type:"bool"`

	helmMgr    install.Manager
	prereqs    *prerequisites.Manager
	helmParams map[string]any
	kClient    kubernetes.Interface
	dClient    dynamic.Interface
	pullSecret *pullsecret.Manager
	features   *feature.Flags
}

func init() {
	// NOTE(tnthornton) we override the runtime.ErrorHandlers so that Helm
	// doesn't leak Println logs.
	kruntime.ErrorHandlers = []kruntime.ErrorHandler{func(_ context.Context, _ error, _ string, _ ...any) {}} //nolint:reassign // disable logging

	kruntime.Must(upboundv1alpha1.AddToScheme(scheme.Scheme))
}

// BeforeApply sets default values in login before assignment and validation.
func (c *initCmd) BeforeApply() error {
	c.Set = make(map[string]string)
	return nil
}

// AfterApply sets default values in command after assignment and validation.
func (c *initCmd) AfterApply(upCtx *upbound.Context, p upterm.Printer) error { //nolint:gocyclo // lot of checks
	if err := c.Registry.AfterApply(); err != nil {
		return err
	}

	kubeconfig, err := upCtx.GetKubeconfig()
	if err != nil {
		return err
	}

	kClient, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = kClient

	// set the defaults
	cloud := c.Set[defaults.ClusterTypeStr]
	defs, err := defaults.GetConfig(c.kClient, cloud, p)
	if err != nil {
		return err
	}

	defs.PublicIngress = c.PublicIngress
	if c.PublicIngress {
		p.PrintWarning("Public ingress will be exposed")
	}

	spacesValues := map[string]string{
		// todo(avalanche123): Remove these defaults once we can default to using
		// Upbound IAM, through connected spaces, to authenticate users in the
		// cluster
		"authentication.hubIdentities": "true",
		"authorization.hubRBAC":        "true",
	}

	if supportsNativeRouterPortConfig(c.Version) {
		if defs.PublicIngress {
			spacesValues["router.proxy.service.type"] = "LoadBalancer"
			switch defs.ClusterType {
			case defaults.AmazonEKS:
				spacesValues["router.proxy.service.annotations.service\\.beta\\.kubernetes\\.io/aws-load-balancer-type"] = "external"
				spacesValues["router.proxy.service.annotations.service\\.beta\\.kubernetes\\.io/aws-load-balancer-scheme"] = "internet-facing"
				spacesValues["router.proxy.service.annotations.service\\.beta\\.kubernetes\\.io/aws-load-balancer-nlb-target-type"] = "ip"
			case defaults.GoogleGKE:
				spacesValues["router.proxy.service.annotations.cloud\\.google\\.com/l4-rbs"] = "enabled"
			case defaults.AzureAKS, defaults.Generic, defaults.Kind:
			}
		} else if defs.ClusterType == defaults.Kind {
			spacesValues["router.proxy.hostPort"] = "443"
		}
	}

	// User supplied values override the defaults
	maps.Copy(spacesValues, c.Set)
	c.Set = spacesValues

	c.pullSecret = pullsecret.NewManagerFromFlags(kClient, defaultImagePullSecret, ns, c.Registry)
	dClient, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	c.dClient = dClient
	mgr, err := helm.NewManager(kubeconfig,
		spacesChart,
		c.Registry.Repository,
		ns,
		helm.WithBasicAuth(c.Registry.Username, c.Registry.Password),
		helm.WithChart(c.Bundle),
		helm.Wait(),
	)
	if err != nil {
		return err
	}
	c.helmMgr = mgr

	base := map[string]any{}
	if c.File != nil {
		defer c.File.Close() //nolint:errcheck // nothing we do with the err
		b, err := io.ReadAll(c.File)
		if err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
		if err := yaml.Unmarshal(b, &base); err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
		if err := c.File.Close(); err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
	}
	parser := helm.NewParser(base, c.Set)
	c.helmParams, err = parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseInstallParameters)
	}
	c.features = &feature.Flags{}
	spacefeature.EnableFeatures(c.features, c.helmParams)

	prereqs, err := prerequisites.New(kubeconfig, defs, c.features, c.Version, p)
	if err != nil {
		return err
	}
	c.prereqs = prereqs

	return nil
}

// Run executes the install command.
func (c *initCmd) Run(ctx context.Context, upCtx *upbound.Context, printer upterm.Printer) error { //nolint:gocyclo // lot of checks
	overrideRegistry(c.Registry.Repository.String(), c.helmParams)
	ensureAccount(upCtx, c.helmParams)

	if c.helmParams["account"] == defaultAcct {
		printer.PrintWarning("No Upbound organization name was provided. Spaces initialized without an organization cannot be attached to the Upbound console! This cannot be changed later.")
		result, _ := upterm.Confirm(fmt.Sprintf("Would you like to proceed with the default Upbound organization %q?", defaultAcct), false)
		if !result {
			printer.PrintError("Not proceeding without an Upbound organization; pass the --organization flag or create a profile (`up login` or `up profile create`).")
			return nil
		}
	}

	// check if required prerequisites are installed
	status, err := c.prereqs.Check()
	if err != nil {
		printer.PrintError("error checking prerequisites status:", err)
		return err
	}

	// At least 1 prerequisite is not installed, check if we should install the
	// missing ones for the client.
	if len(status.NotInstalled) > 0 {
		printer.PrintWarning("One or more required prerequisites are not installed:")
		printer.Println()
		for _, p := range status.NotInstalled {
			printer.Println(fmt.Sprintf("❌ %s", p.GetName()))
		}

		if !c.Yes {
			result, _ := upterm.Confirm("Would you like to install them now?", false)
			if !result {
				printer.PrintError("prerequisites must be met in order to proceed with installation")
				return nil
			}
		}
		if err := installPrereqs(status, printer); err != nil {
			return err
		}
	}

	printer.PrintInfo("Required prerequisites met!")
	printer.PrintInfo("Proceeding with Upbound Spaces installation...")

	if err := c.applySecret(ctx, printer); err != nil {
		return err
	}

	if err := c.deploySpace(ctx, c.helmParams, printer); err != nil {
		return err
	}

	profileName, err := c.ensureProfile(upCtx)
	if err != nil {
		return err
	}

	printer.PrintSuccess(fmt.Sprintf("Your Upbound Space is Ready! Using new Upbound profile %q for your new Space.", profileName))

	outputNextSteps(printer)

	return nil
}

func installPrereqs(status *prerequisites.Status, printer upterm.Printer) error {
	for i, p := range status.NotInstalled {
		if err := printer.WrapWithSuccessSpinner(
			upterm.StepCounter(
				fmt.Sprintf("Installing %s", p.GetName()),
				i+1,
				len(status.NotInstalled),
			),
			p.Install,
		); err != nil {
			return err
		}
	}
	return nil
}

func (c *initCmd) applySecret(ctx context.Context, printer upterm.Printer) error {
	createPullSecret := func() error {
		return errors.Wrap(c.pullSecret.CreateOrUpdate(ctx), errCreateImagePullSecret)
	}

	if err := printer.WrapWithSuccessSpinner(
		upterm.StepCounter(fmt.Sprintf("Creating pull secret %s", defaultImagePullSecret), 1, 3),
		createPullSecret,
	); err != nil {
		return err
	}
	return nil
}

func initVersionBounds(ch *chart.Chart) error {
	return checkVersion(fmt.Sprintf("unsupported chart version %q", ch.Metadata.Version), initVersionConstraints, ch.Metadata.Version)
}

func upVersionBounds(ch *chart.Chart) error {
	s, found := ch.Metadata.Annotations[chartAnnotationUpConstraints]
	if !found {
		return nil
	}
	constraints, err := parseChartUpConstraints(s)
	if err != nil {
		return fmt.Errorf("up version constraints %q provided by the chart are invalid: %w", s, err)
	}

	return checkVersion(fmt.Sprintf("unsupported up version %q", version.Version()), constraints, version.Version())
}

func (c *initCmd) deploySpace(ctx context.Context, params map[string]any, printer upterm.Printer) error {
	install := func() error {
		if err := c.helmMgr.Install(strings.TrimPrefix(c.Version, "v"), params, initVersionBounds, upVersionBounds); err != nil {
			return err
		}
		return nil
	}

	if err := printer.WrapWithSuccessSpinner(
		upterm.StepCounter("Initializing Space components", 2, 3),
		install,
	); err != nil {
		return err
	}

	version, _ := semver.NewVersion(c.Version)
	requiresUXP, _ := semver.NewConstraint("< v1.7.0-0")

	return printer.WrapWithSuccessSpinner(
		upterm.StepCounter("Starting Space Components", 3, 3),
		func() error {
			if !requiresUXP.Check(version) {
				return nil
			}
			errC, err := kube.DynamicWatch(ctx, c.dClient.Resource(hostclusterGVR), &watcherTimeout, func(u *unstructured.Unstructured) (bool, error) {
				up := resources.HostCluster{Unstructured: *u}
				if resource.IsConditionTrue(up.GetCondition(xpv1.TypeReady)) {
					return true, nil
				}
				return false, nil
			})
			if err != nil {
				return err
			}
			if err := <-errC; err != nil {
				return err
			}
			return nil
		},
	)
}

func (c *initCmd) ensureProfile(upCtx *upbound.Context) (string, error) {
	// If the user is already in a disconnected profile, and it has a
	// kubeconfig, assume the user created a profile for this space init and
	// wants to keep it.
	if upCtx.Profile.Type == profile.TypeDisconnected && upCtx.Profile.SpaceKubeconfig != nil {
		return upCtx.ProfileName, nil
	}

	// Otherwise, create a new disconnected profile for interacting with the new
	// space.
	kubeconfig, err := upCtx.GetRawKubeconfig()
	if err != nil {
		return "", errors.Wrap(err, "failed to get kubeconfig")
	}
	if err := clientcmdapi.MinifyConfig(&kubeconfig); err != nil {
		return "", errors.Wrap(err, "failed to create kubeconfig for Upbound profile")
	}

	p := &profile.Profile{
		Type: profile.TypeDisconnected,
		// If the user already has a logged in profile, copy the login data over
		// to the disconnected profile.
		ID:        upCtx.Profile.ID,
		TokenType: upCtx.Profile.TokenType,
		Session:   upCtx.Profile.Session,
		// Use the domain and org that the space was initialized with. The org
		// might be empty if this space isn't going to be connected; that's
		// fine.
		Domain:       upCtx.Domain.String(),
		Organization: upCtx.Organization,
		// Give the profile a kubeconfig containing only the active context.
		SpaceKubeconfig: &kubeconfig,
	}

	if err := upCtx.Cfg.AddOrUpdateUpboundProfile(kubeconfig.CurrentContext, *p); err != nil {
		return "", errors.Wrap(err, "failed to create profile for new space")
	}
	if err := upCtx.Cfg.SetDefaultUpboundProfile(kubeconfig.CurrentContext); err != nil {
		return "", errors.Wrap(err, "failed to select default profile")
	}

	if err := upCtx.CfgSrc.UpdateConfig(upCtx.Cfg); err != nil {
		return "", errors.Wrap(err, "failed to save configuration")
	}

	return kubeconfig.CurrentContext, nil
}

func outputNextSteps(p upterm.Printer) {
	p.Println()
	p.PrintInfo("Next Steps 👇")
	p.Println()
	p.Println("👉 Check out Upbound Spaces docs @ https://docs.upbound.io/manuals/spaces/overview/")
}
