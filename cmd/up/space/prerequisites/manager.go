// Copyright 2025 Upbound Inc.
// All rights reserved

package prerequisites

import (
	"github.com/Masterminds/semver/v3"
	"k8s.io/client-go/rest"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/feature"

	"github.com/upbound/up/cmd/up/space/defaults"
	spacefeature "github.com/upbound/up/cmd/up/space/features"
	"github.com/upbound/up/cmd/up/space/prerequisites/certmanager"
	"github.com/upbound/up/cmd/up/space/prerequisites/cloudnativepg"
	"github.com/upbound/up/cmd/up/space/prerequisites/ingressnginx"
	"github.com/upbound/up/cmd/up/space/prerequisites/opentelemetrycollector"
	"github.com/upbound/up/cmd/up/space/prerequisites/providers/helm"
	"github.com/upbound/up/cmd/up/space/prerequisites/providers/kubernetes"
	"github.com/upbound/up/cmd/up/space/prerequisites/uxp"
)

var errCreatePrerequisite = "failed to instantiate prerequisite manager"

// Prerequisite defines the API that is used to interogate an installation
// prerequisite.
type Prerequisite interface {
	GetName() string

	Install() error
	IsInstalled() (bool, error)
}

// Manager provides APIs for interacting with Prerequisites within the target
// cluster.
type Manager struct {
	prereqs []Prerequisite
}

// Status represents the overall status of the Prerequisite within the target
// cluster.
type Status struct {
	NotInstalled []Prerequisite
}

// New constructs a new Manager for working with installation Prerequisites.
func New(config *rest.Config, defs *defaults.CloudConfig, features *feature.Flags, versionStr string) (*Manager, error) { //nolint:gocyclo
	prereqs := []Prerequisite{}

	version, err := semver.NewVersion(versionStr)
	if err != nil {
		return nil, errors.New("invalid version format: " + err.Error())
	}

	requiresUXP, err := semver.NewConstraint("< v1.7.0-0")
	if err != nil {
		return nil, errors.New("invalid version constraint: " + err.Error())
	}

	// Check if the version satisfies the constraint
	if requiresUXP.Check(version) {
		uxp, err := uxp.New(config)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create UXP prerequisite")
		}
		prereqs = append(prereqs, uxp)

		pk8s, err := kubernetes.New(config)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create Kubernetes prerequisite")
		}
		prereqs = append(prereqs, pk8s)

		phelm, err := helm.New(config)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create Helm prerequisite")
		}
		prereqs = append(prereqs, phelm)
	}

	certmanager, err := certmanager.New(config)
	if err != nil {
		return nil, errors.Wrap(err, errCreatePrerequisite)
	}
	prereqs = append(prereqs, certmanager)

	svcType := ingressnginx.NodePort
	if defs != nil && defs.PublicIngress {
		svcType = ingressnginx.LoadBalancer
	}

	ingress, err := ingressnginx.New(config, svcType)
	if err != nil {
		return nil, errors.Wrap(err, errCreatePrerequisite)
	}
	prereqs = append(prereqs, ingress)

	if features.Enabled(spacefeature.EnableAlphaSharedTelemetry) {
		otelopr, err := opentelemetrycollector.New(config)
		if err != nil {
			return nil, errors.Wrap(err, errCreatePrerequisite)
		}
		prereqs = append(prereqs, otelopr)
	}

	if features.Enabled(spacefeature.EnableAlphaQueryAPI) {
		cnpg, err := cloudnativepg.New(config)
		if err != nil {
			return nil, errors.Wrap(err, errCreatePrerequisite)
		}
		prereqs = append(prereqs, cnpg)
	}

	return &Manager{
		prereqs: prereqs,
	}, nil
}

// Check performs IsInstalled checks for each of the Prerequisites against the
// target cluster.
func (m *Manager) Check() (*Status, error) {
	notInstalled := []Prerequisite{}
	for _, p := range m.prereqs {
		installed, err := p.IsInstalled()
		if err != nil {
			return nil, err
		}
		if !installed {
			notInstalled = append(notInstalled, p)
		}
	}

	return &Status{
		NotInstalled: notInstalled,
	}, nil
}
