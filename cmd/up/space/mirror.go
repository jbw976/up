// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package space contains functions for handling spaces
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
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/oci"
	"github.com/upbound/up/internal/upterm"
)

func (c *mirrorCmd) Help() string {
	return `
The 'mirror' command mirrors all required OCI artifacts for a specific Space version.

Examples:
	space mirror -v 1.9.0 --output-dir=/tmp/output --token-file=upbound-token.json
		This command mirrors all artifacts for Space version 1.9.0
		into a local directory as .tar.gz files, using the token file for authentication.

	space mirror -v 1.9.0 --destination-registry=myregistry.io --token-file=upbound-token.json
		This command mirrors all artifacts for Space version 1.9.0
		to a specified container registry, using the token file for authentication.
    	Note: Ensure you log in to the registry first using a command like 'docker login myregistry.io'.

	space mirror -v 1.9.0 --output-dir=/tmp/output --token-file=upbound-token.json --dry-run
		This command performs a dry run to verify mirroring of all artifacts for
		Space version 1.9.0 into a local directory as .tar.gz files,
		using the token file for authentication.
		A request is made to the Upbound registry to confirm network access.
`
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
	Registry authorizedRegistryFlags `embed:""`

	OutputDir           string `help:"The local directory path where exported artifacts will be saved as .tgz files." optional:"" short:"t"`
	DestinationRegistry string `help:"The target container registry where the artifacts will be mirrored."            optional:"" short:"d"`
	Version             string `help:"The specific Spaces version for which the artifacts will be mirrored."          required:"" short:"v"`

	craneOpts []crane.Option

	fetchManifest      func(ref string, opts ...crane.Option) ([]byte, error)
	getValuesFromChart func(chart, version string, pathNavigator oci.PathNavigator, username, password string) ([]string, error)
	defaultPrint       func(format string, a ...interface{})

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
	c.defaultPrint = pterm.Printfln

	return nil
}

// Run executes the mirror command.
func (c *mirrorCmd) Run(printer upterm.ObjectPrinter) error {
	artifacts, err := initPathNavigator()
	if err != nil {
		return errors.Wrap(err, "unable to get artifact list")
	}

	for _, repo := range artifacts {
		if err := c.mirror(printer, repo); err != nil {
			return errors.Wrap(err, "mirror artifacts failed")
		}
	}

	return nil
}

func (c *mirrorCmd) mirror(printer upterm.ObjectPrinter, repo repository) (rErr error) {
	chart, tag, err := c.parseChartReference(repo.Chart)
	if err != nil {
		return err
	}

	if err := c.mirrorArtifact(chart, tag, printer); err != nil {
		return errors.Wrap(err, "failed to mirror artifact")
	}

	if err := c.mirrorSubResources(chart, tag, repo.SubResources, printer); err != nil {
		return errors.Wrap(err, "failed to mirror subresources")
	}

	if err := c.mirrorImages(repo.Images, printer); err != nil {
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

func (c *mirrorCmd) mirrorSubResources(chart, tag string, subResources []subResource, printer upterm.ObjectPrinter) error {
	for _, subResource := range subResources {
		if subResource.PathNavigator == nil {
			continue
		}

		versions, err := c.getValuesFromChart(chart, tag, subResource.PathNavigator, c.Registry.Username, c.Registry.Password)
		if err != nil {
			return errors.Wrap(err, "unable to extract")
		}
		if err := c.processSubResource(subResource, versions, printer); err != nil {
			return errors.Wrap(err, "unable to process sub resources")
		}
	}
	return nil
}

func (c *mirrorCmd) processSubResource(subResource subResource, versions []string, printer upterm.ObjectPrinter) error {
	for _, version := range versions {
		if len(subResource.Chart) > 0 {
			if err := c.mirrorArtifact(subResource.Chart, version, printer); err != nil {
				return errors.Wrapf(err, "mirroring chart image %s", subResource.Chart)
			}
		}
		if len(subResource.Image) > 0 {
			versionWithV := version
			if !strings.HasPrefix(version, "v") {
				versionWithV = "v" + version
			}
			if err := c.mirrorArtifact(subResource.Image, versionWithV, printer); err != nil {
				return errors.Wrap(err, "unable to mirror artifact")
			}
		}
	}
	return nil
}

func (c *mirrorCmd) mirrorImages(images []imageReference, printer upterm.ObjectPrinter) error {
	baseVersion, err := semver.NewVersion(c.Version)
	if err != nil {
		return errors.Wrapf(err, "error parsing space version")
	}

	for _, image := range images {
		if err := c.processImage(image, baseVersion, printer); err != nil {
			return errors.Wrap(err, "unable to process image")
		}
	}

	return nil
}

func (c *mirrorCmd) processImage(image imageReference, baseVersion *semver.Version, printer upterm.ObjectPrinter) error {
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
		return nil
	}

	imageName := image.Image
	version := fmt.Sprintf("v%s", c.Version)
	if parts := strings.Split(imageName, ":"); len(parts) > 1 {
		imageName = parts[0]
		version = parts[1]
	}

	return c.mirrorArtifact(imageName, version, printer)
}

func (c *mirrorCmd) mirrorArtifact(image, version string, printer upterm.ObjectPrinter) error {
	var artifact artifactHandler

	switch {
	case printer.DryRun:
		artifact = &dryRunMirror{
			folder:        c.path,
			registry:      c.DestinationRegistry,
			opts:          c.craneOpts,
			fetchManifest: c.fetchManifest,
			defaultPrint:  c.defaultPrint,
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

	return artifact.handle(fmt.Sprintf("%s:%s", image, version))
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
	handle(artifact string) error
}

type dryRunMirror struct {
	folder        string
	registry      string
	opts          []crane.Option
	fetchManifest func(artifact string, opts ...crane.Option) ([]byte, error)
	defaultPrint  func(format string, a ...interface{})
}

func (h *dryRunMirror) handle(artifact string) error {
	if _, err := h.fetchManifest(artifact, h.opts...); err != nil {
		return errors.Wrapf(err, "artifact is not available in registry %s", artifact)
	}
	if h.folder != "" {
		h.defaultPrint("crane pull %s %s.tgz", artifact, filepath.Join(h.folder, oci.GetArtifactName(artifact)))
	}
	if h.registry != "" {
		h.defaultPrint("crane copy %s %s/%s", artifact, h.registry, oci.RemoveDomainAndOrg(artifact))
	}
	return nil
}

type localMirror struct {
	folder string
	opts   []crane.Option
}

func (h *localMirror) handle(artifact string) error {
	path := filepath.Join(h.folder, fmt.Sprintf("%s.tgz", oci.GetArtifactName(artifact)))

	img, err := crane.Pull(artifact, h.opts...)
	if err != nil {
		return errors.Wrap(err, "error pulling image")
	}
	if err := crane.Save(img, artifact, path); err != nil {
		return errors.Wrapf(err, "error saving image %s", path)
	}
	pterm.Printfln("Successfully mirrored artifact '%s' to destination '%s'", artifact, path)
	return nil
}

type registryMirror struct {
	registry string
	opts     []crane.Option
}

func (h *registryMirror) handle(artifact string) error {
	registry := fmt.Sprintf("%s/%s", h.registry, oci.RemoveDomainAndOrg(artifact))
	if err := crane.Copy(artifact, registry, h.opts...); err != nil {
		return errors.Wrapf(err, "copy/push failed %s", artifact)
	}
	pterm.Printfln("Successfully mirrored artifact '%s' to destination '%s'", artifact, registry)
	return nil
}

// StaticKeychain is a simple keychain that returns different credentials for specific registries
type StaticKeychain struct {
	credentials map[string]authn.AuthConfig
}

// Resolve returns an authenticator for the given registry
func (s *StaticKeychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	if creds, ok := s.credentials[target.RegistryStr()]; ok {
		return authn.FromConfig(creds), nil
	}
	return authn.Anonymous, nil // Fallback to anonymous if no match is found
}
