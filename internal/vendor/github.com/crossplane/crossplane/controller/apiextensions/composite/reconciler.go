// Copyright 2025 Upbound Inc.
// All rights reserved

package composite

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	composition "github.com/crossplane/crossplane/controller/apiextensions/compositions"
)

const (
	shortWait = 30 * time.Second
	longWait  = 1 * time.Minute
	timeout   = 2 * time.Minute
)

// Error strings
const (
	errGet          = "cannot get composite resource"
	errUpdate       = "cannot update composite resource"
	errUpdateStatus = "cannot update composite resource status"
	errSelectComp   = "cannot select Composition"
	errFetchComp    = "cannot fetch Composition"
	errConfigure    = "cannot configure composite resource"
	errPublish      = "cannot publish connection details"
	errRenderCD     = "cannot render composed resource"
	errValidate     = "refusing to use invalid Composition"
	errAssociate    = "cannot associate composed resources with Composition resource templates"

	errFmtRender = "cannot render composed resource from resource template at index %d"

	errFmtPatchEnvironment = "cannot apply environment patch at index %d"
)

// Event reasons.
const (
	reasonResolve event.Reason = "SelectComposition"
	reasonCompose event.Reason = "ComposeResources"
	reasonPublish event.Reason = "PublishConnectionSecret"
)

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of composite resource.
func ControllerName(name string) string {
	return "composite/" + name
}

// ConnectionSecretFilterer returns a set of allowed keys.
type ConnectionSecretFilterer interface {
	GetConnectionSecretKeys() []string
}

// A ConnectionPublisher publishes the supplied ConnectionDetails for the
// supplied resource. Publishers must handle the case in which the supplied
// ConnectionDetails are empty.
type ConnectionPublisher interface {
	// PublishConnection details for the supplied resource. Publishing must be
	// additive; i.e. if details (a, b, c) are published, subsequently
	// publishing details (b, c, d) should update (b, c) but not remove a.
	// Returns 'published' if the publish was not a no-op.
	PublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error)
}

// A ConnectionPublisherFn publishes the supplied ConnectionDetails for the
// supplied resource.
type ConnectionPublisherFn func(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error)

// PublishConnection details for the supplied resource.
func (fn ConnectionPublisherFn) PublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error) {
	return fn(ctx, o, c)
}

// A CompositionSelector selects a composition reference.
type CompositionSelector interface {
	SelectComposition(ctx context.Context, cr resource.Composite) error
}

// A CompositionSelectorFn selects a composition reference.
type CompositionSelectorFn func(ctx context.Context, cr resource.Composite) error

// SelectComposition for the supplied composite resource.
func (fn CompositionSelectorFn) SelectComposition(ctx context.Context, cr resource.Composite) error {
	return fn(ctx, cr)
}

// A CompositionFetcher fetches an appropriate Composition for the supplied
// composite resource.
type CompositionFetcher interface {
	Fetch(ctx context.Context, cr resource.Composite) (*v1.Composition, error)
}

// A CompositionFetcherFn fetches an appropriate Composition for the supplied
// composite resource.
type CompositionFetcherFn func(ctx context.Context, cr resource.Composite) (*v1.Composition, error)

// Fetch an appropriate Composition for the supplied Composite resource.
func (fn CompositionFetcherFn) Fetch(ctx context.Context, cr resource.Composite) (*v1.Composition, error) {
	return fn(ctx, cr)
}

// A Configurator configures a composite resource using its composition.
type Configurator interface {
	Configure(ctx context.Context, cr resource.Composite, cp *v1.Composition) error
}

// A ConfiguratorFn configures a composite resource using its composition.
type ConfiguratorFn func(ctx context.Context, cr resource.Composite, cp *v1.Composition) error

// Configure the supplied composite resource using its composition.
func (fn ConfiguratorFn) Configure(ctx context.Context, cr resource.Composite, cp *v1.Composition) error {
	return fn(ctx, cr, cp)
}

// A CompositionRequest is a request to compose resources.
// It should be treated as immutable.
type CompositionRequest struct {
	Revision *v1.CompositionRevision
}

// A CompositionResult is the result of the composition process.
type CompositionResult struct {
	Composed          []ComposedResource
	ConnectionDetails managed.ConnectionDetails
	Events            []event.Event
}

// A CompositionTarget is the target of a composition event or condition.
type CompositionTarget string

// Composition event and condition targets.
const (
	CompositionTargetComposite         CompositionTarget = "Composite"
	CompositionTargetCompositeAndClaim CompositionTarget = "CompositeAndClaim"
)

// A TargetedEvent represents an event produced by the composition process. It
// can target either the XR only, or both the XR and the claim.
type TargetedEvent struct {
	event.Event
	Target CompositionTarget
	// Detail about the event to be included in the composite resource event but
	// not the claim.
	Detail string
}

// AsEvent produces the base event.
func (e *TargetedEvent) AsEvent() event.Event {
	return event.Event{Type: e.Type, Reason: e.Reason, Message: e.Message, Annotations: e.Annotations}
}

// AsDetailedEvent produces an event with additional detail if available.
func (e *TargetedEvent) AsDetailedEvent() event.Event {
	if e.Detail == "" {
		return e.AsEvent()
	}
	msg := fmt.Sprintf("%s: %s", e.Detail, e.Message)
	return event.Event{Type: e.Type, Reason: e.Reason, Message: msg, Annotations: e.Annotations}
}

// A TargetedCondition represents a condition produced by the composition
// process. It can target either the XR only, or both the XR and the claim.
type TargetedCondition struct {
	xpv1.Condition
	Target CompositionTarget
}

// A Composer composes (i.e. creates, updates, or deletes) resources given the
// supplied composite resource and composition request.
type Composer interface {
	Compose(ctx context.Context, xr *composite.Unstructured, req CompositionRequest) ([]ComposedResourceState, error)
}

// A ComposerFn composes resources.
type ComposerFn func(ctx context.Context, xr *composite.Unstructured, req CompositionRequest) (CompositionResult, error)

// Compose resources.
func (fn ComposerFn) Compose(ctx context.Context, xr *composite.Unstructured, req CompositionRequest) (CompositionResult, error) {
	return fn(ctx, xr, req)
}

// A ComposerSelectorFn selects the appropriate Composer for a mode.
type ComposerSelectorFn func(*v1.CompositionMode) Composer

// Compose calls the Composer returned by calling fn.
func (fn ComposerSelectorFn) Compose(ctx context.Context, xr *composite.Unstructured, req CompositionRequest) ([]ComposedResourceState, error) {
	return fn(req.Revision.Spec.Mode).Compose(ctx, xr, req)
}

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(log logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = log
	}
}

// WithComposer specifies how the Reconciler should compose resources.
func WithComposer(c Composer) ReconcilerOption {
	return func(r *Reconciler) {
		r.resource = c
	}
}

type revision struct {
	CompositionRevisionValidator
}

// A CompositionRevisionValidator validates the supplied CompositionRevision.
type CompositionRevisionValidator interface {
	Validate(*v1.CompositionRevision) error
}

// A CompositionRevisionValidatorFn is a function that validates a
// CompositionRevision.
type CompositionRevisionValidatorFn func(*v1.CompositionRevision) error

// Validate the supplied CompositionRevision.
func (fn CompositionRevisionValidatorFn) Validate(c *v1.CompositionRevision) error {
	return fn(c)
}

// WithConfigurator specifies how the Reconciler should configure
// composite resources using their composition.
func WithConfigurator(c Configurator) ReconcilerOption {
	return func(r *Reconciler) {
		r.composite.Configurator = c
	}
}

type compositeResource struct {
	Configurator
}

// NewReconciler returns a new Reconciler of composite resources.
func NewReconciler(of resource.CompositeKind, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		gvk: schema.GroupVersionKind(of),

		composite: compositeResource{
			Configurator: NewConfiguratorChain(NewAPINamingConfigurator(), NewAPIConfigurator()),
		},

		revision: revision{
			CompositionRevisionValidator: CompositionRevisionValidatorFn(func(rev *v1.CompositionRevision) error {
				// TODO(negz): Presumably this validation will eventually be
				// removed in favor of the new Composition validation
				// webhook.
				// This is the last remaining use of conv.FromRevisionSpec -
				// we can stop generating that once this is removed.
				conv := &v1.GeneratedRevisionSpecConverter{}
				comp := &v1.Composition{Spec: conv.FromRevisionSpec(rev.Spec)}
				_, errs := comp.Validate()
				return errs.ToAggregate()
			}),
		},

		resource: NewPTComposer(nil),

		log: logging.NewNopLogger(),
	}

	for _, f := range opts {
		f(r)
	}
	return r
}

// A Reconciler reconciles composite resources.
type Reconciler struct {
	gvk schema.GroupVersionKind

	revision  revision
	composite compositeResource

	resource Composer

	log logging.Logger
}

// composedRenderState is a wrapper around a composed resource that tracks whether
// it was successfully rendered or not, together with a list of patches defined
// on its template that have been applied (not filtered out).
type composedRenderState struct {
	resource       resource.Composed
	rendered       bool
	appliedPatches []v1.Patch
}

// Reconcile a composite resource.
func (r *Reconciler) Reconcile(ctx context.Context, comp *v1.Composition) ([]ComposedResourceState, error) { //nolint:gocyclo
	// NOTE(negz): Like most Reconcile methods, this one is over our cyclomatic
	// complexity goal. Be wary when adding branches, and look for functionality
	// that could be reasonably moved into an injected dependency.

	xr := composite.New(composite.WithGroupVersionKind(r.gvk))
	xr.SetName(PlaceholderName)
	xr.SetUID(types.UID(PlaceholderUID))

	// TODO(negz): Composition validation should be handled by a validation
	// webhook, not by this controller.
	if err := r.revision.Validate(composition.NewCompositionRevision(comp, 1)); err != nil {
		r.log.Debug(errValidate, "error", err)
		return nil, err
	}

	if err := r.composite.Configure(ctx, xr, comp); err != nil {
		r.log.Debug(errConfigure, "error", err)
		return nil, err
	}

	// Inline PatchSets from Composition Spec before composing resources

	res, err := r.resource.Compose(ctx, xr, CompositionRequest{Revision: &v1.CompositionRevision{Spec: composition.NewCompositionRevisionSpec(comp.Spec, 0)}})
	if err != nil {
		r.log.Debug(errRenderCD, "error", err)
	}

	return res, nil
}

// filterToXRPatches selects patches defined in composed templates,
// whose type is one of the XR-targeting patches
// (e.g. v1.PatchTypeToCompositeFieldPath or v1.PatchTypeCombineToComposite)
func filterToXRPatches(tas []TemplateAssociation) []v1.Patch {
	filtered := make([]v1.Patch, 0, len(tas))
	for _, ta := range tas {
		filtered = append(filtered, filterPatches(ta.Template.Patches,
			patchTypesToXR()...)...)
	}
	return filtered
}
