// Copyright 2025 Upbound Inc.
// All rights reserved

package simulation

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient/wait"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/diff"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/simulation"
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
	quiet config.QuietFlag
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
func (c *CreateCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context, quiet config.QuietFlag) error {
	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	if c.Group == "" {
		ns, err := upCtx.GetCurrentContextNamespace()
		if err != nil {
			return err
		}
		c.Group = ns
	}

	c.debugPrintf(kongCtx.Stderr, "debug logging enabled\n")
	c.quiet = quiet
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

	run, err := c.startRun(ctx, kongCtx, spacesClient)
	if err != nil {
		return errors.Wrap(err, "error starting simulation")
	}

	p.Printfln("Simulation %q created", run.Simulation().Name)

	// wait for simulated ctp to be able to accept changes
	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter("Waiting for simulated control plane to start", 1, totalSteps),
		upterm.CheckmarkSuccessSpinner,
		waitForConditionStep(ctx, spacesClient, run, simulation.AcceptingChanges(), wait.WithTimeout(controlPlaneReadyTimeout)),
		c.quiet,
	); err != nil {
		return err
	}

	simConfig, err := run.RESTConfig(ctx, upCtx)
	if err != nil {
		return err
	}

	// apply changeset
	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter("Applying the changeset to the simulation control plane", 2, totalSteps),
		stepSpinner,
		c.applyChangesetStep(simConfig),
		c.quiet,
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
		waitForConditionStep(ctx, spacesClient, run, simulation.Complete()),
		c.quiet,
	); err != nil {
		return err
	}

	// compute + print diff
	s, _ := stepSpinner.Start(upterm.StepCounter("Computing simulated differences", 4, totalSteps))

	c.debugPrintf(kongCtx.Stderr, "total changes on the Simulation object: %d\n", len(run.Simulation().Status.Changes))

	diffSet, err := run.DiffSet(ctx, upCtx, []schema.GroupKind{})
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
			func() error {
				return run.Terminate(ctx, spacesClient)
			},
			c.quiet,
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

// startRun starts a simulation using the configuration defined in the command
// options.
func (c *CreateCmd) startRun(ctx context.Context, kongCtx *kong.Context, spacesClient client.Client) (*simulation.Run, error) {
	runOpts := []simulation.Option{
		simulation.WithDebugPrintfFunc(func(format string, args ...any) {
			c.debugPrintf(kongCtx.Stderr, format, args)
		}),
		simulation.WithCompleteAfter(1 * time.Minute),
	}

	if c.CompleteAfter != nil {
		runOpts = append(runOpts, simulation.WithCompleteAfter(*c.CompleteAfter))
	}

	if c.SimulationName != "" {
		runOpts = append(runOpts, simulation.WithName(c.SimulationName))
	}

	return simulation.Start(ctx, spacesClient, types.NamespacedName{
		Name:      c.SourceName,
		Namespace: c.Group,
	}, runOpts...)
}

// waitForConditionStep defines a step to poll until a specific wait condition
// is met.
func waitForConditionStep(ctx context.Context, spacesClient client.Client, run *simulation.Run, cond simulation.WaitConditionFunc, opts ...wait.Option) func() error {
	return func() error {
		return run.WaitForCondition(ctx, spacesClient, cond, opts...)
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

// debugPrintf defines a printer for writing debug logs from internal methods.
func (c *CreateCmd) debugPrintf(stderr io.Writer, format string, args ...any) {
	if c.Flags.Debug > 0 {
		fmt.Fprintf(stderr, format, args...) //nolint:errcheck // Fine if debug output fails to print.
	}
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
