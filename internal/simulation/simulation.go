// Copyright 2025 Upbound Inc.
// All rights reserved

// Package simulation contains a helper class and methods for managing control
// plane simulation runs.
package simulation

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient/wait"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	spacesv1alpha1 "github.com/upbound/up-sdk-go/apis/spaces/v1alpha1"
	upctx "github.com/upbound/up/cmd/up/ctx"
	"github.com/upbound/up/internal/diff"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

const (
	// simulationCompleteReason is the value present in the `reason` field of
	// the `AcceptingChanges` condition (on a Simulation) once the results have
	// been published.
	simulationCompleteReason = "SimulationComplete"
)

// Run stores state about a single invocation of a control plane
// simulation.
type Run struct {
	simulation *spacesv1alpha1.Simulation

	diffSet []diff.ResourceDiff

	debugPrintf func(format string, args ...any)
}

// RESTConfig returns a rest.Config pointing at the simulated control plane.
func (r *Run) RESTConfig(ctx context.Context, upCtx *upbound.Context) (*rest.Config, error) {
	po := clientcmd.NewDefaultPathOptions()
	var err error

	conf, err := po.GetStartingConfig()
	if err != nil {
		return nil, err
	}
	state, err := upctx.DeriveState(ctx, upCtx, conf, kube.GetIngressHost)
	if err != nil {
		return nil, err
	}

	var ok bool
	var space upctx.Space

	if space, ok = state.(upctx.Space); !ok {
		if group, ok := state.(*upctx.Group); ok {
			space = group.Space
		} else if ctp, ok := state.(*upctx.ControlPlane); ok {
			space = ctp.Group.Space
		} else {
			return nil, errors.New("current kubeconfig is not pointed at a space cluster")
		}
	}

	if r.simulation.Status.SimulatedControlPlaneName == nil {
		return nil, errors.New("simulation has not been populated with a simulated control plane name")
	}
	ctp := types.NamespacedName{Namespace: r.simulation.GetNamespace(), Name: *r.simulation.Status.SimulatedControlPlaneName}
	spaceClient, err := space.BuildKubeconfig(ctp)
	if err != nil {
		return nil, err
	}

	return spaceClient.ClientConfig()
}

// WaitForCondition polls the simulation until it matches the condition
// defined in the condition function.
func (r *Run) WaitForCondition(ctx context.Context, client client.Client, conditionFunc WaitConditionFunc, opts ...wait.Option) error {
	waitOpts := []wait.Option{
		wait.WithContext(ctx),
		wait.WithImmediate(),
		wait.WithInterval(time.Second * 2), //nolint:gomnd // default value
	}
	waitOpts = append(waitOpts, opts...)

	if err := wait.For(func(ctx context.Context) (bool, error) {
		if err := client.Get(ctx, types.NamespacedName{Name: r.simulation.Name, Namespace: r.simulation.Namespace}, r.simulation); err != nil {
			return false, err
		}
		return conditionFunc(r.simulation)
	}, waitOpts...); err != nil {
		return errors.Wrap(err, "error while waiting for simulation to complete")
	}
	return nil
}

// Terminate updates the Simulation, setting the desired state to "Terminated".
func (r *Run) Terminate(ctx context.Context, client client.Client) error {
	r.simulation.Spec.DesiredState = spacesv1alpha1.SimulationStateTerminated
	if err := client.Update(ctx, r.simulation); err != nil {
		return errors.Wrap(err, "unable to terminate simulation")
	}
	return nil
}

// Simulation returns the simulation defined in the run.
func (r *Run) Simulation() *spacesv1alpha1.Simulation {
	return r.simulation
}

// WaitConditionFunc defines a function that can be used with the
// `WaitForCondition` method on a simulation run.
type WaitConditionFunc func(sim *spacesv1alpha1.Simulation) (bool, error)

// AcceptingChanges returns a wait condition function that will wait until the
// simulation is ready to accept changes.
func AcceptingChanges() WaitConditionFunc {
	return func(sim *spacesv1alpha1.Simulation) (bool, error) {
		return sim.Status.GetCondition(spacesv1alpha1.TypeAcceptingChanges).Status == corev1.ConditionTrue, nil
	}
}

// Complete returns a wait condition function that will wait until the
// simulation has been marked as complete.
func Complete() WaitConditionFunc {
	return func(sim *spacesv1alpha1.Simulation) (bool, error) {
		if sim.Spec.DesiredState != spacesv1alpha1.SimulationStateComplete {
			return false, nil
		}
		return sim.Status.GetCondition(spacesv1alpha1.TypeAcceptingChanges).Reason == simulationCompleteReason, nil
	}
}

// Option defines a modifier to be applied to a simulation run when starting.
type Option func(run *Run, sim *spacesv1alpha1.Simulation)

// WithCompleteAfter sets a Duration type completion criteria.
func WithCompleteAfter(duration time.Duration) Option {
	return func(_ *Run, sim *spacesv1alpha1.Simulation) {
		sim.Spec.CompletionCriteria = []spacesv1alpha1.CompletionCriterion{{
			Type:     spacesv1alpha1.CompletionCriterionTypeDuration,
			Duration: metav1.Duration{Duration: duration},
		}}
	}
}

// WithName sets an explicit name on the optional simulation.
func WithName(name string) Option {
	return func(_ *Run, sim *spacesv1alpha1.Simulation) {
		sim.ObjectMeta.Name = name
		sim.ObjectMeta.GenerateName = ""
	}
}

// WithDebugPrintfFunc sets a debug printer function that can be called by
// internal methods.
func WithDebugPrintfFunc(debugPrintf func(format string, args ...any)) Option {
	return func(run *Run, _ *spacesv1alpha1.Simulation) {
		run.debugPrintf = debugPrintf
	}
}

// Start begins a simulation run for a given control plane with a provided set
// of optional features enabled. By default, the simulation will have a
// generated name and no completion criteria.
func Start(ctx context.Context, client client.Client, sourceControlPlane types.NamespacedName, opts ...Option) (*Run, error) {
	r := &Run{
		debugPrintf: func(_ string, _ ...any) {},
	}

	sim := &spacesv1alpha1.Simulation{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", sourceControlPlane.Name),
			Namespace:    sourceControlPlane.Namespace,
		},
		Spec: spacesv1alpha1.SimulationSpec{
			ControlPlaneName: sourceControlPlane.Name,
			DesiredState:     spacesv1alpha1.SimulationStateAcceptingChanges,
		},
	}

	for _, o := range opts {
		o(r, sim)
	}

	if err := client.Create(ctx, sim); err != nil {
		return nil, errors.Wrap(err, "error creating simulation")
	}

	r.simulation = sim

	return r, nil
}
