// Copyright 2025 Upbound Inc.
// All rights reserved

package space

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/blang/semver/v4"
	"helm.sh/helm/v3/pkg/chart"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"

	spacefeature "github.com/upbound/up/cmd/up/space/features"
	"github.com/upbound/up/cmd/up/space/prerequisites"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/registry"
	"github.com/upbound/up/internal/registry/pullsecret"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	msgUpgrading                   = "Upgrading"
	msgDowngrading                 = "Downgrading"
	errFailedGettingCurrentVersion = "failed to retrieve current version"
	errInvalidVersionFmt           = "invalid version %q"
	errAborted                     = "aborted"
	warnDowngrade                  = "Downgrades are not supported."
	warnMajorUpgrade               = "Upgrades to a new major version are only supported for explicitly documented releases."
	warnMinorVersionSkip           = "Upgrades which skip a minor version are not supported."
)

// upgradeCmd upgrades Upbound.
type upgradeCmd struct {
	upbound.RequiresContext
	install.CommonParams

	Registry registry.AuthorizedFlags `embed:""`

	// NOTE(hasheddan): version is currently required for upgrade with OCI image
	// as latest strategy is undetermined.
	Version  string `arg:""                                                             help:"Upbound Spaces version to upgrade to."`
	Yes      bool   `help:"Answer yes to all questions"                                 name:"yes"                                   type:"bool"`
	Rollback bool   `help:"Rollback to previously installed version on failed upgrade."`

	helmMgr    install.Manager
	prereqs    *prerequisites.Manager
	helmParams map[string]any
	kClient    kubernetes.Interface
	pullSecret *pullsecret.Manager
	features   *feature.Flags
	oldVersion string
	downgrade  bool
}

// BeforeApply sets default values in login before assignment and validation.
func (c *upgradeCmd) BeforeApply() error {
	c.Set = make(map[string]string)
	return nil
}

// AfterApply sets default values in command after assignment and validation.
func (c *upgradeCmd) AfterApply(upCtx *upbound.Context, p upterm.Printer) error { //nolint:gocyclo // lot of checks
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

	c.pullSecret = pullsecret.NewManagerFromFlags(kClient, defaultImagePullSecret, ns, c.Registry)
	mgr, err := helm.NewManager(kubeconfig,
		spacesChart,
		c.Registry.Repository,
		ns,
		helm.WithBasicAuth(c.Registry.Username, c.Registry.Password),
		helm.WithChart(c.Bundle),
		helm.RollbackOnError(c.Rollback),
		helm.Wait())
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

	// validate versions
	c.oldVersion, err = mgr.GetCurrentVersion()
	if err != nil {
		return errors.Wrap(err, errFailedGettingCurrentVersion)
	}

	if c.Bundle == nil {
		from, err := semver.Parse(c.oldVersion)
		if err != nil {
			return errors.Wrapf(err, errInvalidVersionFmt, c.oldVersion)
		}
		to, err := semver.Parse(strings.TrimPrefix(c.Version, "v"))
		if err != nil {
			return errors.Wrapf(err, errInvalidVersionFmt, c.Version)
		}
		c.downgrade = from.GT(to)

		if err := c.validateVersions(from, to, p); err != nil {
			return err
		}
	}

	c.features = &feature.Flags{}
	spacefeature.EnableFeatures(c.features, c.helmParams)

	prereqs, err := prerequisites.New(kubeconfig, nil, c.features, c.Version, p)
	if err != nil {
		return err
	}
	c.prereqs = prereqs

	return nil
}

// Run executes the upgrade command.
func (c *upgradeCmd) Run(ctx context.Context, printer upterm.Printer) error {
	overrideRegistry(c.Registry.Repository.String(), c.helmParams)

	// check if required prerequisites are installed
	status, err := c.prereqs.Check()
	if err != nil {
		printer.PrintError("error checking prerequisites status")
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
				printer.PrintError("prerequisites must be met in order to proceed with upgrade")
				return nil
			}
		}
		if err := installPrereqs(status, printer); err != nil {
			return err
		}
	}

	printer.PrintInfo("Required prerequisites met!")
	printer.PrintInfo("Proceeding with Upbound Spaces upgrade...")

	// Create or update image pull secret.
	pullSecret := func() error {
		return errors.Wrap(c.pullSecret.CreateOrUpdate(ctx), errCreateImagePullSecret)
	}

	if err := printer.WrapWithSuccessSpinner(
		upterm.StepCounter(fmt.Sprintf("Creating pull secret %s", defaultImagePullSecret), 1, 2),
		pullSecret,
	); err != nil {
		return err
	}

	if err := c.upgradeUpbound(c.helmParams, printer); err != nil {
		return err
	}

	printer.PrintSuccess("Your Upbound Space is Ready after Upgrade!")

	outputNextSteps(printer)

	return nil
}

func upgradeVersionBounds(_ string, ch *chart.Chart) error {
	return checkVersion(fmt.Sprintf("unsupported target chart version %s", ch.Metadata.Version), upgradeVersionConstraints, ch.Metadata.Version)
}

func upgradeFromVersionBounds(from string, ch *chart.Chart) error {
	return checkVersion(fmt.Sprintf("unsupported installed chart version %s", ch.Metadata.Version), upgradeFromVersionConstraints, from)
}

func upgradeUpVersionBounds(_ string, ch *chart.Chart) error {
	return upVersionBounds(ch)
}

func (c *upgradeCmd) upgradeUpbound(params map[string]any, printer upterm.Printer) error {
	version := strings.TrimPrefix(c.Version, "v")
	upgrade := func() error {
		if err := c.helmMgr.Upgrade(version, params, upgradeUpVersionBounds, upgradeFromVersionBounds, upgradeVersionBounds); err != nil {
			return err
		}
		return nil
	}

	verb := msgUpgrading
	if c.downgrade {
		verb = msgDowngrading
	}

	if err := printer.WrapWithSuccessSpinner(
		upterm.StepCounter(fmt.Sprintf("%s Space from v%s to v%s", verb, c.oldVersion, version), 2, 2),
		upgrade,
	); err != nil {
		return err
	}

	return nil
}

// validateVersions checks whether the upgrade/downgrade is allowed based on version changes.
func (c *upgradeCmd) validateVersions(from, to semver.Version, p upterm.Printer) error {
	switch {
	case c.downgrade:
		return warnAndConfirm(p, warnDowngrade)
	case to.Major > from.Major:
		return warnAndConfirm(p, warnMajorUpgrade)
	case to.Minor > from.Minor+1:
		return warnAndConfirm(p, warnMinorVersionSkip)
	default:
		// No warning means the validation passed
		return nil
	}
}

// warnAndConfirm displays a warning and prompts for confirmation.
func warnAndConfirm(p upterm.Printer, warning string, args ...any) error {
	p.PrintWarning(fmt.Sprintf(warning, args...)) // Display the warning message

	if result, _ := upterm.Confirm("Are you sure you want to proceed?", false); !result {
		return errors.New(errAborted)
	}

	return nil
}
