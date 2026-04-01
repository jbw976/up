// Copyright 2025 Upbound Inc.
// All rights reserved

package space

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/oci"
	"github.com/upbound/up/internal/registry"
	"github.com/upbound/up/internal/upterm"

	_ "embed"
)

const (
	// uxpV2HelmChart is the OCI chart for Upbound Crossplane (UXP) v2+ (major >= 2 in supportedVersions).
	// UXP v1 uses universal-crossplane from mirrorconfig.yaml.
	uxpV2HelmChart = "xpkg.upbound.io/spaces-artifacts/crossplane"
	// uxpV2ControllerManagerImage is mirrored for each UXP v2 supportedVersion alongside the crossplane runtime image.
	uxpV2ControllerManagerImage = "xpkg.upbound.io/spaces-artifacts/controller-manager"
)

//go:embed help/mirror.md
var mirrorHelp string

func (c *mirrorCmd) Help() string {
	return mirrorHelp
}

type repository struct {
	Chart        string           `yaml:"chart"`
	Images       []imageReference `yaml:"images"`
	SubResources []subResource    `yaml:"subResources"`
}

type imageReference struct {
	Image                  string `yaml:"image"`
	CompatibleChartVersion string `yaml:"compatibleChartVersion,omitempty"`
}

type subResource struct {
	PathNavigator     oci.PathNavigator `yaml:"pathNavigator,omitempty"`
	PathNavigatorType string            `yaml:"pathNavigatorType"`
	Chart             string            `yaml:"chart,omitempty"`
	Image             string            `yaml:"image,omitempty"`
}

type ociconfig struct {
	OCI []repository `yaml:"oci"`
}

type mirrorCmd struct {
	Registry registry.AuthorizedFlags `embed:""`

	OutputDir           string `help:"The local directory path where exported artifacts will be saved as .tgz files." optional:"" short:"t"`
	DestinationRegistry string `help:"The target container registry where the artifacts will be mirrored."            optional:"" short:"d"`
	Version             string `help:"The specific Spaces version for which the artifacts will be mirrored."          required:"" short:"v"`
	DryRun              bool   `help:"Print what would be mirrored but do not take action."`

	craneOpts []crane.Option

	fetchManifest       func(ref string, opts ...crane.Option) ([]byte, error)
	getValuesFromChart  func(chart, version string, pathNavigator oci.PathNavigator, username, password string) ([]string, error)
	getUxpV2RuntimeTags func(chart, version, username, password string) (crossplaneTag, controllerManagerTag string, err error)

	path string
}

func (c *mirrorCmd) AfterApply() error {
	if err := c.Registry.AfterApply(); err != nil {
		return err
	}
	// remove leading v
	c.Version = strings.TrimPrefix(c.Version, "v")
	multiKeychain := authn.NewMultiKeychain(authn.DefaultKeychain)

	if c.Registry.TokenFile != nil {
		staticKeychain := &StaticKeychain{
			credentials: map[string]authn.AuthConfig{
				c.Registry.Endpoint.Host: {
					Username: c.Registry.Username,
					Password: c.Registry.Password,
				},
			},
		}
		multiKeychain = authn.NewMultiKeychain(multiKeychain, staticKeychain)
	}

	c.craneOpts = append(c.craneOpts, crane.WithAuthFromKeychain(multiKeychain))

	if c.OutputDir != "" {
		fs := afero.NewBasePathFs(afero.NewOsFs(), c.OutputDir)
		if err := fs.MkdirAll("", 0o750); err != nil {
			return fmt.Errorf("failed to create or ensure directory exists: %w", err)
		}
		if baseFs, ok := fs.(*afero.BasePathFs); ok {
			c.path = afero.FullBaseFsPath(baseFs, "")
		}
	}

	c.fetchManifest = crane.Manifest
	c.getValuesFromChart = oci.GetValuesFromChart
	c.getUxpV2RuntimeTags = oci.GetUxpV2RuntimeTags

	return nil
}

// Run executes the mirror command.
func (c *mirrorCmd) Run(p upterm.Printer) error {
	artifacts, err := initPathNavigator()
	if err != nil {
		return errors.Wrap(err, "unable to get artifact list")
	}

	for _, repo := range artifacts {
		if err := c.mirror(p, repo); err != nil {
			return errors.Wrap(err, "mirror artifacts failed")
		}
	}

	return nil
}

func (c *mirrorCmd) mirror(p upterm.Printer, repo repository) (rErr error) {
	chart, tag, err := c.parseChartReference(repo.Chart)
	if err != nil {
		return err
	}

	if err := c.mirrorArtifact(p, chart, tag); err != nil {
		return errors.Wrap(err, "failed to mirror artifact")
	}

	if err := c.mirrorSubResources(p, chart, tag, repo.SubResources); err != nil {
		return errors.Wrap(err, "failed to mirror subresources")
	}

	if err := c.mirrorImages(p, repo.Images); err != nil {
		return errors.Wrap(err, "failed to mirror images")
	}

	return nil
}

func (c *mirrorCmd) parseChartReference(chartRef string) (chart string, tag string, rErr error) {
	ref, err := name.ParseReference(chartRef)
	if err != nil {
		return "", "", errors.Wrap(err, "error parsing reference")
	}
	chart = ref.Context().Name()

	if tagged, ok := ref.(name.Tag); ok && tagged.TagStr() != "latest" {
		tag = tagged.TagStr()
	} else {
		tag = c.Version
	}

	if _, err := c.fetchManifest(fmt.Sprintf("%s:%s", chart, tag), c.craneOpts...); err != nil {
		return "", "", errors.Wrap(err, "unable to find spaces version")
	}

	return chart, tag, nil
}

func (c *mirrorCmd) mirrorSubResources(p upterm.Printer, chart, tag string, subResources []subResource) error {
	for _, subResource := range subResources {
		if subResource.PathNavigator == nil {
			continue
		}

		versions, err := c.getValuesFromChart(chart, tag, subResource.PathNavigator, c.Registry.Username, c.Registry.Password)
		if err != nil {
			return errors.Wrap(err, "unable to extract")
		}
		if err := c.processSubResource(p, subResource, versions); err != nil {
			return errors.Wrap(err, "unable to process sub resources")
		}
	}
	return nil
}

func (c *mirrorCmd) processSubResource(p upterm.Printer, subResource subResource, versions []string) error {
	if _, ok := subResource.PathNavigator.(*uxpVersionsPath); ok {
		return c.processUxpVersionsSubResource(p, subResource, versions)
	}
	for _, version := range versions {
		if len(subResource.Chart) > 0 {
			if err := c.mirrorArtifact(p, subResource.Chart, version); err != nil {
				return errors.Wrapf(err, "mirroring chart image %s", subResource.Chart)
			}
		}
		if len(subResource.Image) > 0 {
			versionWithV := version
			if !strings.HasPrefix(version, "v") {
				versionWithV = "v" + version
			}
			if err := c.mirrorArtifact(p, subResource.Image, versionWithV); err != nil {
				return errors.Wrap(err, "unable to mirror artifact")
			}
		}
	}
	return nil
}

func (c *mirrorCmd) processUxpVersionsSubResource(p upterm.Printer, subResource subResource, versions []string) error {
	for _, version := range versions {
		sv, err := semver.NewVersion(strings.TrimPrefix(version, "v"))
		if err != nil {
			return errors.Wrapf(err, "parse supported crossplane version %q", version)
		}
		if sv.Major() >= 2 {
			if len(subResource.Chart) > 0 {
				if err := c.mirrorArtifact(p, uxpV2HelmChart, version); err != nil {
					return errors.Wrapf(err, "mirroring chart image %s", uxpV2HelmChart)
				}
			}
			cxTag, cmTag, err := c.getUxpV2RuntimeTags(uxpV2HelmChart, version, c.Registry.Username, c.Registry.Password)
			if err != nil {
				return errors.Wrap(err, "uxp v2 runtime tags from chart")
			}
			if len(subResource.Image) > 0 {
				if err := c.mirrorArtifact(p, subResource.Image, cxTag); err != nil {
					return errors.Wrap(err, "unable to mirror crossplane image")
				}
			}
			if err := c.mirrorArtifact(p, uxpV2ControllerManagerImage, cmTag); err != nil {
				return errors.Wrap(err, "unable to mirror controller-manager image")
			}
			continue
		}
		if len(subResource.Chart) > 0 {
			if err := c.mirrorArtifact(p, subResource.Chart, version); err != nil {
				return errors.Wrapf(err, "mirroring chart image %s", subResource.Chart)
			}
		}
		if len(subResource.Image) > 0 {
			versionWithV := version
			if !strings.HasPrefix(version, "v") {
				versionWithV = "v" + version
			}
			if err := c.mirrorArtifact(p, subResource.Image, versionWithV); err != nil {
				return errors.Wrap(err, "unable to mirror artifact")
			}
		}
	}
	return nil
}

func (c *mirrorCmd) mirrorImages(p upterm.Printer, images []imageReference) error {
	baseVersion, err := semver.NewVersion(c.Version)
	if err != nil {
		return errors.Wrapf(err, "error parsing space version")
	}

	processed := make(map[string]struct{})
	for _, image := range images {
		i := strings.Split(image.Image, ":")[0]
		if _, ok := processed[i]; ok {
			continue
		}
		matches, err := c.processImage(p, image, baseVersion)
		if err != nil {
			return errors.Wrap(err, "unable to process image")
		}
		if matches && len(image.CompatibleChartVersion) > 0 {
			processed[i] = struct{}{}
		}
	}

	return nil
}

func (c *mirrorCmd) processImage(p upterm.Printer, image imageReference, baseVersion *semver.Version) (bool, error) {
	include := true
	if image.CompatibleChartVersion != "" {
		constraint, err := semver.NewConstraint(image.CompatibleChartVersion)
		if err == nil {
			include = constraint.Check(baseVersion) || oci.CheckPreReleaseConstraint(constraint, baseVersion)
		} else {
			include = false
		}
	}

	if !include {
		return false, nil
	}

	imageName := image.Image
	version := fmt.Sprintf("v%s", c.Version)
	if parts := strings.Split(imageName, ":"); len(parts) > 1 {
		imageName = parts[0]
		version = parts[1]
	}

	return true, c.mirrorArtifact(p, imageName, version)
}

func (c *mirrorCmd) mirrorArtifact(p upterm.Printer, image, version string) error {
	var artifact artifactHandler

	switch {
	case c.DryRun:
		artifact = &dryRunMirror{
			folder:        c.path,
			registry:      c.DestinationRegistry,
			opts:          c.craneOpts,
			fetchManifest: c.fetchManifest,
		}
	case len(c.path) > 0:
		artifact = &localMirror{
			folder: c.path,
			opts:   c.craneOpts,
		}
	default:
		artifact = &registryMirror{
			registry: c.DestinationRegistry,
			opts:     c.craneOpts,
		}
	}

	return artifact.handle(p, fmt.Sprintf("%s:%s", image, version))
}

func initPathNavigator() (repo []repository, rErr error) {
	configData, pathNavigator := initConfig()
	var artifacts ociconfig
	err := yaml.Unmarshal(configData, &artifacts)
	if err != nil {
		return nil, errors.Wrap(err, "error reading artifacts config")
	}

	for _, repo := range artifacts.OCI {
		for i := range repo.SubResources {
			subChart := &repo.SubResources[i]
			if subChart.PathNavigatorType == "" {
				continue
			}

			pathValueType, ok := pathNavigator[subChart.PathNavigatorType]
			if !ok {
				return nil, fmt.Errorf("unknown PathNavigatorType: %s", subChart.PathNavigatorType)
			}

			pathValue := reflect.New(pathValueType).Interface()
			pathNavigator, ok := pathValue.(oci.PathNavigator)
			if !ok {
				return nil, fmt.Errorf("failed to assert type oci.PathNavigator for PathNavigatorType: %s", subChart.PathNavigatorType)
			}

			subChart.PathNavigator = pathNavigator
		}
	}

	return artifacts.OCI, nil
}

type artifactHandler interface {
	handle(p upterm.Printer, artifact string) error
}

type dryRunMirror struct {
	folder        string
	registry      string
	opts          []crane.Option
	fetchManifest func(artifact string, opts ...crane.Option) ([]byte, error)
}

func (h *dryRunMirror) handle(p upterm.Printer, artifact string) error {
	if _, err := h.fetchManifest(artifact, h.opts...); err != nil {
		return errors.Wrapf(err, "artifact is not available in registry %s", artifact)
	}
	if h.folder != "" {
		p.Printfln("crane pull %s %s.tgz", artifact, filepath.Join(h.folder, oci.GetArtifactName(artifact)))
	}
	if h.registry != "" {
		p.Printfln("crane copy %s %s/%s", artifact, h.registry, oci.RemoveDomainAndOrg(artifact))
	}
	return nil
}

type localMirror struct {
	folder string
	opts   []crane.Option
}

func (h *localMirror) handle(p upterm.Printer, artifact string) error {
	path := filepath.Join(h.folder, fmt.Sprintf("%s.tgz", oci.GetArtifactName(artifact)))

	img, err := crane.Pull(artifact, h.opts...)
	if err != nil {
		return errors.Wrap(err, "error pulling image")
	}
	if err := crane.Save(img, artifact, path); err != nil {
		return errors.Wrapf(err, "error saving image %s", path)
	}
	p.Printfln("Successfully mirrored artifact '%s' to destination '%s'", artifact, path)
	return nil
}

type registryMirror struct {
	registry string
	opts     []crane.Option
}

func (h *registryMirror) handle(p upterm.Printer, artifact string) error {
	registry := fmt.Sprintf("%s/%s", h.registry, oci.RemoveDomainAndOrg(artifact))
	if err := crane.Copy(artifact, registry, h.opts...); err != nil {
		return errors.Wrapf(err, "copy/push failed %s", artifact)
	}
	p.Printfln("Successfully mirrored artifact '%s' to destination '%s'", artifact, registry)
	return nil
}

// StaticKeychain is a simple keychain that returns different credentials for specific registries.
type StaticKeychain struct {
	credentials map[string]authn.AuthConfig
}

// Resolve returns an authenticator for the given registry.
func (s *StaticKeychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	if creds, ok := s.credentials[target.RegistryStr()]; ok {
		return authn.FromConfig(creds), nil
	}
	return authn.Anonymous, nil // Fallback to anonymous if no match is found
}
