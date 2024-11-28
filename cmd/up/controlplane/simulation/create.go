// Copyright 2021 Upbound Inc
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

package simulation

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	diffv3 "github.com/r3labs/diff/v3"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient/wait"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"

	spacesv1alpha1 "github.com/upbound/up-sdk-go/apis/spaces/v1alpha1"
	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	upctx "github.com/upbound/up/cmd/up/ctx"
	"github.com/upbound/up/internal/diff"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	// controlPlaneReadyTimeout is the time to wait for a simulated control
	// plane to start and be ready to accept changes.
	controlPlaneReadyTimeout = 5 * time.Minute

	// fieldManagerName is the name used to server side apply changes to the
	// simulated control plan.
	fieldManagerName = "up-cli"

	// annotationKeyClonedState is the annotation key storing the JSON
	// representation of the state of the resource at control plane clone time.
	annotationKeyClonedState = "simulation.spaces.upbound.io/cloned-state"

	// simulationCompleteReason is the value present in the `reason` field of
	// the `AcceptingChanges` condition (on a Simulation) once the results have
	// been published.
	simulationCompleteReason = "SimulationComplete"
)

// failOnCondition is the simulation condition that signals a failure in the
// simulation command.
type failOnCondition string

const (
	// failOnNone signals that the command should never return a failure exit
	// code regardless of the results of the simulation.
	failOnNone failOnCondition = "none"
	// failOnDifference signals that the command should return a failure exit
	// code when any difference was detected.
	failOnDifference failOnCondition = "difference"
)

// CreateCmd creates a control plane simulation and outputs the differences
// detected.
type CreateCmd struct {
	SourceName string `arg:""     help:"Name of source control plane."                                                                                               required:""`
	Group      string `default:"" help:"The control plane group that the control plane is contained in. This defaults to the group specified in the current context" short:"g"`

	SimulationName string `help:"The name of the simulation resource" short:"n"`

	Changeset     []string       `help:"Path to the resources that will be applied as part of the simulation. Can either be a single file or a directory" required:"true"                                                                                       short:"f"`
	Recursive     bool           `default:"false"                                                                                                         help:"Process the directory used in -f, --changeset recursively."                                     short:"r"`
	CompleteAfter *time.Duration `default:"60s"                                                                                                           help:"The maximum amount of time the simulated control plane should run before ending the simulation"`

	FailOn            failOnCondition `default:"none"                                                                                              enum:"none, difference"                                                                                                       help:"Fail and exit with a code of '1' if a certain condition is met"`
	Output            string          `help:"Output the results of the simulation to the provided file. Defaults to standard out if not specified" short:"o"`
	Wait              bool            `default:"true"                                                                                              help:"Wait for the simulation to complete. If set to false, the command will exit immediately after the changeset is applied"`
	TerminateOnFinish bool            `default:"false"                                                                                             help:"Terminate the simulation after the completion criteria is met"`

	Flags upbound.Flags `embed:""`
}

// Validate performs custom argument validation for the create command.
func (c *CreateCmd) Validate() error {
	if c.TerminateOnFinish && !c.Wait {
		return errors.New("--wait=true is required when using --terminate-on-finish=true")
	}

	for _, path := range c.Changeset {
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return fmt.Errorf("changeset path %q does not exist", path)
		} else if err != nil {
			return err
		}
	}

	return nil
}

// AfterApply sets default values in command after assignment and validation.
func (c *CreateCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	if c.Group == "" {
		ns, _, err := upCtx.Kubecfg.Namespace()
		if err != nil {
			return err
		}
		c.Group = ns
	}

	c.debugPrintf(kongCtx.Stderr, "debug logging enabled\n")

	return nil
}

// Run executes the create command.
func (c *CreateCmd) Run(ctx context.Context, kongCtx *kong.Context, p pterm.TextPrinter, upCtx *upbound.Context, spacesClient client.Client) error { //nolint:gocyclo // TODO: simplify this
	stepSpinner := upterm.CheckmarkSuccessSpinner.WithShowTimer(true)

	var srcCtp spacesv1beta1.ControlPlane
	if err := spacesClient.Get(ctx, types.NamespacedName{Name: c.SourceName, Namespace: c.Group}, &srcCtp); err != nil {
		if kerrors.IsNotFound(err) {
			return fmt.Errorf("control plane %q not found", c.SourceName)
		}
		return err
	}

	totalSteps := 4
	if !c.Wait {
		totalSteps = 2
	}
	if c.TerminateOnFinish {
		totalSteps++
	}

	sim, err := c.createSimulation(ctx, spacesClient)
	if err != nil {
		return err
	}
	p.Printfln("Simulation %q created", sim.Name)

	// wait for simulated ctp to be able to accept changes
	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter("Waiting for simulated control plane to start", 1, totalSteps),
		upterm.CheckmarkSuccessSpinner,
		c.waitForSimulationAcceptingChangesStep(ctx, sim, spacesClient),
	); err != nil {
		return err
	}

	simConfig, err := getControlPlaneConfig(ctx, upCtx, types.NamespacedName{Namespace: c.Group, Name: *sim.Status.SimulatedControlPlaneName})
	if err != nil {
		return err
	}

	// apply changeset
	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter("Applying the changeset to the simulation control plane", 2, totalSteps),
		stepSpinner,
		c.applyChangesetStep(simConfig),
	); err != nil {
		return err
	}

	if !c.Wait {
		p.Printf("The simulation was started and the changeset was applied")
		return nil
	}

	// wait for simulation to complete
	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter("Waiting for simulation to complete", 3, totalSteps),
		stepSpinner,
		c.waitForSimulationCompleteStep(ctx, sim, spacesClient),
	); err != nil {
		return err
	}

	// compute + print diff
	s, _ := stepSpinner.Start(upterm.StepCounter("Computing simulated differences", 4, totalSteps))

	c.debugPrintf(kongCtx.Stderr, "total changes on the Simulation object: %d\n", len(sim.Status.Changes))

	diffSet, err := c.createResourceDiffSet(ctx, kongCtx, simConfig, sim.Status.Changes)
	if err != nil {
		return err
	}
	s.Success()

	c.debugPrintf(kongCtx.Stderr, "created resource diff set of size: %d\n", len(diffSet))

	if c.TerminateOnFinish {
		// terminate simulation
		if err := upterm.WrapWithSuccessSpinner(
			upterm.StepCounter("Terminating simulation", 5, totalSteps),
			stepSpinner,
			c.terminateSimulation(ctx, sim, spacesClient),
		); err != nil {
			return err
		}
	}

	if err := c.outputDiff(kongCtx, diffSet); err != nil {
		return errors.Wrap(err, "failed to write diff to output")
	}

	switch c.FailOn {
	case failOnNone:
		break
	case failOnDifference:
		if len(diffSet) > 0 {
			return errors.New("failing since differences were detected")
		}
	}

	return nil
}

// createSimulation creates a new simulation object.
func (c *CreateCmd) createSimulation(ctx context.Context, client client.Client) (*spacesv1alpha1.Simulation, error) {
	sim := &spacesv1alpha1.Simulation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.SimulationName,
			Namespace: c.Group,
		},
		Spec: spacesv1alpha1.SimulationSpec{
			ControlPlaneName: c.SourceName,
			DesiredState:     spacesv1alpha1.SimulationStateAcceptingChanges,
		},
	}

	if sim.Name == "" {
		sim.GenerateName = c.SourceName + "-"
	}

	if c.CompleteAfter != nil {
		sim.Spec.CompletionCriteria = []spacesv1alpha1.CompletionCriterion{{
			Type:     spacesv1alpha1.CompletionCriterionTypeDuration,
			Duration: metav1.Duration{Duration: *c.CompleteAfter},
		}}
	}

	if err := client.Create(ctx, sim); err != nil {
		return nil, errors.Wrap(err, "error creating simulation")
	}

	return sim, nil
}

// waitForSimulationAcceptingChangesStep pauses until the given simulation is
// able to accept changes, or times out.
func (c *CreateCmd) waitForSimulationAcceptingChangesStep(ctx context.Context, sim *spacesv1alpha1.Simulation, client client.Client) func() error {
	return func() error {
		if err := wait.For(func(ctx context.Context) (bool, error) {
			if err := client.Get(ctx, types.NamespacedName{Name: sim.Name, Namespace: sim.Namespace}, sim); err != nil {
				return false, err
			}
			return sim.Status.GetCondition(spacesv1alpha1.TypeAcceptingChanges).Status == corev1.ConditionTrue, nil
		}, wait.WithImmediate(), wait.WithInterval(time.Second*2), wait.WithTimeout(controlPlaneReadyTimeout), wait.WithContext(ctx)); err != nil {
			return errors.Wrap(err, "timed out before simulation could accept changes")
		}
		return nil
	}
}

// waitForSimulationCompleteStep pauses until the given simulation has been
// marked as complete.
func (c *CreateCmd) waitForSimulationCompleteStep(ctx context.Context, sim *spacesv1alpha1.Simulation, client client.Client) func() error {
	return func() error {
		if err := wait.For(func(ctx context.Context) (bool, error) {
			if err := client.Get(ctx, types.NamespacedName{Name: sim.Name, Namespace: sim.Namespace}, sim); err != nil {
				return false, err
			}
			if sim.Spec.DesiredState != spacesv1alpha1.SimulationStateComplete {
				return false, nil
			}
			return sim.Status.GetCondition(spacesv1alpha1.TypeAcceptingChanges).Reason == simulationCompleteReason, nil
		}, wait.WithImmediate(), wait.WithInterval(time.Second*2), wait.WithContext(ctx)); err != nil {
			return errors.Wrap(err, "error while waiting for simulation to complete")
		}
		return nil
	}
}

// terminateSimulation marks the simulation as terminated.
func (c *CreateCmd) terminateSimulation(ctx context.Context, sim *spacesv1alpha1.Simulation, client client.Client) func() error {
	return func() error {
		sim.Spec.DesiredState = spacesv1alpha1.SimulationStateTerminated
		if err := client.Update(ctx, sim); err != nil {
			return errors.Wrap(err, "unable to terminate simulation")
		}
		return nil
	}
}

// applyChangesetStep loads the changeset resources specified in the argument
// and applies them to the control plane.
func (c *CreateCmd) applyChangesetStep(config *rest.Config) func() error {
	return func() error {
		getter := kube.NewRESTClientGetter(config, "")

		objects, err := loadResources(getter, c.Changeset, c.Recursive)
		if err != nil {
			return errors.Wrap(err, "unable to load changeset resources")
		}

		for _, object := range objects {
			if err := applyOneObject(object); err != nil {
				return errors.Wrapf(err, "unable to apply object [%s]", object.String())
			}
		}

		return nil
	}
}

// removeFieldsForDiff removes any fields that should be excluded from the diff.
func (c *CreateCmd) removeFieldsForDiff(u *unstructured.Unstructured) error {
	// based on the filters in the simulation preprocessor
	// https://github.com/upbound/spaces/blob/v1.8.0/internal/controller/mxe/simulation/preprocess.go#L100-L108
	trim := []string{
		"metadata.generateName",
		"metadata.uid",
		"metadata.resourceVersion",
		"metadata.generation",
		"metadata.creationTimestamp",
		"metadata.ownerReferences",
		"metadata.managedFields",
		"metadata.annotations['kubectl.kubernetes.io/last-applied-configuration']",
		fmt.Sprintf("metadata.annotations['%s']", annotationKeyClonedState),
		"spec.compositionRevisionRef",
	}

	wildcards := []string{
		"status.conditions[*].lastTransitionTime",
	}

	p := fieldpath.Pave(u.UnstructuredContent())

	// expand each wildcard path and add to list to trim
	for _, wildcard := range wildcards {
		expanded, err := p.ExpandWildcards(wildcard)
		if err != nil {
			return errors.Wrap(err, "unable to expand wildcards in ignored fields")
		}
		trim = append(trim, expanded...)
	}

	for _, path := range trim {
		if err := p.DeleteField(path); err != nil {
			return errors.Wrap(err, "cannot delete field")
		}
	}

	return nil
}

// createResourceDiffSet reads through all of the changes from the simulation
// status and looks up the difference between the initial version of the
// resource and the version currently in the API server (at the time of the
// function call).
func (c *CreateCmd) createResourceDiffSet(ctx context.Context, kongCtx *kong.Context, config *rest.Config, changes []spacesv1alpha1.SimulationChange) ([]diff.ResourceDiff, error) { //nolint:gocyclo // TODO: simplify this
	lookup, err := kube.NewDiscoveryResourceLookup(config)
	if err != nil {
		return []diff.ResourceDiff{}, errors.Wrap(err, "unable to create resource lookup client")
	}

	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return []diff.ResourceDiff{}, errors.Wrap(err, "unable to create dynamic client")
	}

	diffSet := make([]diff.ResourceDiff, 0, len(changes))

	c.debugPrintf(kongCtx.Stderr, "iterating over %d changes\n", len(changes))

	// stores a list of resources that we want to filter in the diff, that
	// aren't being filtered in the reconciler
	trimKind := []schema.GroupVersionKind{
		{Group: "apiextensions.crossplane.io", Version: "v1", Kind: "CompositionRevision"},
		{Group: "pkg.crossplane.io", Version: "v1beta1", Kind: "DeploymentRuntimeConfig"},
	}

	for _, change := range changes {
		gvk := schema.FromAPIVersionAndKind(change.ObjectReference.APIVersion, change.ObjectReference.Kind)

		// todo(redbackthomson): Remove this logic once we have done a better
		// job of filtering in the reconciler
		if slices.Contains(trimKind, gvk) {
			c.debugPrintf(kongCtx.Stderr, "skipping gvk %+v\n", gvk)
			continue
		}

		rs, err := lookup.Get(gvk)
		if err != nil {
			c.debugPrintf(kongCtx.Stderr, "unable to find gvk from lookup %q\n", gvk)
			return []diff.ResourceDiff{}, err
		}

		switch change.Change { //nolint:exhaustive // Proceed with the rest of the loop if other.
		case spacesv1alpha1.SimulationChangeTypeCreate:
			diffSet = append(diffSet, diff.ResourceDiff{
				SimulationChange: change,
			})
			c.debugPrintf(kongCtx.Stderr, "appended create to diff set for %v\n", change.ObjectReference)
			continue
		case spacesv1alpha1.SimulationChangeTypeDelete:
			diffSet = append(diffSet, diff.ResourceDiff{
				SimulationChange: change,
			})
			c.debugPrintf(kongCtx.Stderr, "appended delete to diff set for %v\n", change.ObjectReference)
			continue
		}

		var cl dynamic.ResourceInterface
		ncl := dyn.Resource(schema.GroupVersionResource{
			Group:    rs.Group,
			Version:  rs.Version,
			Resource: rs.Name,
		})
		if change.ObjectReference.Namespace != nil {
			cl = ncl.Namespace(*change.ObjectReference.Namespace)
		} else {
			cl = ncl
		}

		after, err := cl.Get(ctx, change.ObjectReference.Name, metav1.GetOptions{})
		if err != nil {
			return []diff.ResourceDiff{}, errors.Wrap(err, "unable to get object from simulated control plane")
		}

		beforeRaw, ok := after.GetAnnotations()[annotationKeyClonedState]
		if !ok {
			c.debugPrintf(kongCtx.Stderr, "object %v is missing the previous cloned state annotation\n", change.ObjectReference)
			continue
		}
		beforeObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, []byte(beforeRaw))
		if err != nil {
			return []diff.ResourceDiff{}, errors.Wrapf(err, "previous cloned state annotation on %v could not be decoded", change.ObjectReference)
		}

		before, ok := beforeObj.(*unstructured.Unstructured)
		if !ok {
			return []diff.ResourceDiff{}, errors.Wrap(err, "before object not unstructured")
		}
		if err := c.removeFieldsForDiff(after); err != nil {
			return []diff.ResourceDiff{}, errors.Wrapf(err, "unable to remove fields before diff")
		}

		if err := c.removeFieldsForDiff(before); err != nil {
			return []diff.ResourceDiff{}, errors.Wrapf(err, "unable to remove fields before diff")
		}

		diffd, err := diffv3.Diff(before.UnstructuredContent(), after.UnstructuredContent())
		if err != nil {
			return []diff.ResourceDiff{}, errors.Wrapf(err, "unable to calculate diff for object %v", change.ObjectReference)
		}

		// we filtered out all of the changes
		if len(diffd) == 0 {
			continue
		}

		diffSet = append(diffSet, diff.ResourceDiff{
			SimulationChange: change,
			Diff:             diffd,
		})
		c.debugPrintf(kongCtx.Stderr, "appended update to diff set for %v\n", change.ObjectReference)
	}
	return diffSet, nil
}

// outputDiff outputs the diff to the location, and in the format, specified by
// the command line arguments.
func (c *CreateCmd) outputDiff(kongCtx *kong.Context, diffSet []diff.ResourceDiff) error {
	stdout := c.Output == ""

	// todo(redbackthomson): Use a different printer for JSON or YAML output
	buf := &strings.Builder{}
	writer := diff.NewPrettyPrintWriter(buf, stdout)
	_ = writer.Write(diffSet)

	if stdout {
		if _, err := fmt.Fprintf(kongCtx.Stdout, "\n\n"); err != nil {
			return errors.Wrap(err, "failed to write output")
		}
		if _, err := fmt.Fprint(kongCtx.Stdout, buf.String()); err != nil {
			return errors.Wrap(err, "failed to write output")
		}
		return nil
	}

	return os.WriteFile(c.Output, []byte(buf.String()), 0o644) //nolint:gosec // nothing system sensitive in the file
}

func (c *CreateCmd) debugPrintf(stderr io.Writer, format string, args ...any) {
	if c.Flags.Debug > 0 {
		fmt.Fprintf(stderr, format, args...) //nolint:errcheck // Fine if debug output fails to print.
	}
}

// getControlPlaneConfig gets a REST config for a given control plane within
// the space.
func getControlPlaneConfig(ctx context.Context, upCtx *upbound.Context, ctp types.NamespacedName) (*rest.Config, error) {
	po := clientcmd.NewDefaultPathOptions()
	var err error

	conf, err := po.GetStartingConfig()
	if err != nil {
		return nil, err
	}
	state, err := upctx.DeriveState(ctx, upCtx, conf, profile.GetIngressHost)
	if err != nil {
		return nil, err
	}

	var ok bool
	var space *upctx.Space

	if space, ok = state.(*upctx.Space); !ok {
		if group, ok := state.(*upctx.Group); ok {
			space = &group.Space
		} else if ctp, ok := state.(*upctx.ControlPlane); ok {
			space = &ctp.Group.Space
		} else {
			return nil, errors.New("current kubeconfig is not pointed at a space cluster")
		}
	}

	spaceClient, err := space.BuildClient(upCtx, ctp)
	if err != nil {
		return nil, err
	}

	return spaceClient.ClientConfig()
}

// loadResources builds a list of resources from the given path.
func loadResources(getter resource.RESTClientGetter, paths []string, recursive bool) ([]*resource.Info, error) {
	return resource.NewBuilder(getter).
		Unstructured().
		Path(recursive, paths...).
		Flatten().
		Do().
		Infos()
}

// applyOneObject applies objects to whichever client was used to build the
// resource. Uses server side apply with the force flag set to true.
func applyOneObject(info *resource.Info) error {
	helper := resource.NewHelper(info.Client, info.Mapping).
		WithFieldManager(fieldManagerName).
		WithFieldValidation("Strict")

	data, err := runtime.Encode(unstructured.UnstructuredJSONScheme, info.Object)
	if err != nil {
		return errors.Wrap(err, "unable to decode object")
	}

	options := metav1.PatchOptions{
		Force: ptr.To(true),
	}
	obj, err := helper.Patch(info.Namespace, info.Name, types.ApplyPatchType, data, &options)
	if err != nil {
		return err
	}

	_ = info.Refresh(obj, true)
	return nil
}
