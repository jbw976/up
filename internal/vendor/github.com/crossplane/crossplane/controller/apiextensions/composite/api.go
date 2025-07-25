// Copyright 2025 Upbound Inc.
// All rights reserved

package composite

import (
	"context"
	"math/rand"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

// Error strings.
const (
	errApplySecret = "cannot apply connection secret"

	errNoCompatibleComposition         = "no compatible Compositions found"
	errNoCompatibleCompositionRevision = "no compatible CompositionRevisions found"
	errGetComposition                  = "cannot get Composition"
	errGetCompositionRevision          = "cannot get CompositionRevision"
	errListCompositions                = "cannot list Compositions"
	errListCompositionRevisions        = "cannot list CompositionRevisions"
	errUpdateComposite                 = "cannot update composite resource"
	errCompositionNotCompatible        = "referenced composition is not compatible with this composite resource"
	errGetXRD                          = "cannot get composite resource definition"
	errFetchCompositionRevision        = "cannot fetch composition revision"
)

// Event reasons.
const (
	reasonCompositionSelection event.Reason = "CompositionSelection"
)

// APIFilteredSecretPublisher publishes ConnectionDetails content after filtering
// it through a set of permitted keys.
type APIFilteredSecretPublisher struct {
	client resource.Applicator
	filter []string
}

// NewAPIFilteredSecretPublisher returns a ConnectionPublisher that only
// publishes connection secret keys that are included in the supplied filter.
func NewAPIFilteredSecretPublisher(c client.Client, filter []string) *APIFilteredSecretPublisher {
	return &APIFilteredSecretPublisher{client: resource.NewAPIPatchingApplicator(c), filter: filter}
}

// PublishConnection publishes the supplied ConnectionDetails to the Secret
// referenced in the resource.
func (a *APIFilteredSecretPublisher) PublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (bool, error) {
	// This resource does not want to expose a connection secret.
	if o.GetWriteConnectionSecretToReference() == nil {
		return false, nil
	}

	s := resource.ConnectionSecretFor(o, o.GetObjectKind().GroupVersionKind())
	m := map[string]bool{}
	// TODO(muvaf): Should empty filter allow all keys?
	for _, key := range a.filter {
		m[key] = true
	}
	for key, val := range c {
		if _, ok := m[key]; ok {
			s.Data[key] = val
		}
	}

	err := a.client.Apply(ctx, s,
		resource.ConnectionSecretMustBeControllableBy(o.GetUID()),
		resource.AllowUpdateIf(func(current, desired runtime.Object) bool {
			// We consider the update to be a no-op and don't allow it if the
			// current and existing secret data are identical.
			return !cmp.Equal(current.(*corev1.Secret).Data, desired.(*corev1.Secret).Data, cmpopts.EquateEmpty())
		}),
	)
	if resource.IsNotAllowed(err) {
		// The update was not allowed because it was a no-op.
		return false, nil
	}
	if err != nil {
		return false, errors.Wrap(err, errApplySecret)
	}

	return true, nil
}

// UnpublishConnection is no-op since PublishConnection only creates resources
// that will be garbage collected by Kubernetes when the managed resource is
// deleted.
func (a *APIFilteredSecretPublisher) UnpublishConnection(_ context.Context, _ resource.ConnectionSecretOwner, _ managed.ConnectionDetails) error {
	return nil
}

// An APICompositionFetcher fetches the Composition referenced by a composite
// resource.
type APICompositionFetcher struct {
	reader client.Reader
}

// NewAPICompositionFetcher returns a CompositionFetcher that fetches the
// Composition referenced by a composite resource.
func NewAPICompositionFetcher(r client.Reader) *APICompositionFetcher {
	return &APICompositionFetcher{reader: r}
}

// Fetch the Composition referenced by the supplied composite resource. Panics
// if the composite resource's composition reference is nil.
func (f *APICompositionFetcher) Fetch(ctx context.Context, cr resource.Composite) (*v1.Composition, error) {
	comp := &v1.Composition{}
	err := f.reader.Get(ctx, meta.NamespacedNameOf(cr.GetCompositionReference()), comp)
	return comp, errors.Wrap(err, errGetComposition)
}

// An APIRevisionFetcher selects the appropriate CompositionRevision for a
// composite resource, fetches it, and returns it as a Composition. This is done
// for compatibility with existing Composition logic while CompositionRevisions
// are in alpha.
type APIRevisionFetcher struct {
	ca resource.ClientApplicator
}

// NewAPIRevisionFetcher returns a RevisionFetcher that fetches the
// Revision referenced by a composite resource.
func NewAPIRevisionFetcher(ca resource.ClientApplicator) *APIRevisionFetcher {
	return &APIRevisionFetcher{ca: ca}
}

// Fetch the appropriate CompositionRevision for the supplied composite
// resource and convert it to a Composition. Panics if the composite resource's
// composition reference is nil, but handles setting the composition revision
// reference.
func (f *APIRevisionFetcher) Fetch(ctx context.Context, cr resource.Composite) (*v1.Composition, error) {
	current := cr.GetCompositionRevisionReference()
	pol := cr.GetCompositionUpdatePolicy()

	// We've already selected a revision, and our update policy is manual.
	// Just fetch and return the selected revision.
	if current != nil && pol != nil && *pol == xpv1.UpdateManual {
		rev := &v1.CompositionRevision{}
		err := f.ca.Get(ctx, types.NamespacedName{Name: current.Name}, rev)
		return AsComposition(rev), errors.Wrap(err, errGetCompositionRevision)
	}

	// We either haven't yet selected a revision, or our update policy is
	// automatic. Either way we need to determine the latest revision.

	comp := &v1.Composition{}
	if err := f.ca.Get(ctx, meta.NamespacedNameOf(cr.GetCompositionReference()), comp); err != nil {
		return nil, errors.Wrap(err, errGetComposition)
	}

	rl, err := f.getCompositionRevisionList(ctx, cr, comp)
	if err != nil {
		return nil, errors.Wrap(err, errFetchCompositionRevision)
	}

	latest := v1.LatestRevision(comp, rl.Items)
	if current == nil {
		return nil, errors.New(errNoCompatibleCompositionRevision)
	}

	if current == nil || current.Name != latest.GetName() {
		cr.SetCompositionRevisionReference(&corev1.LocalObjectReference{Name: latest.GetName()})
		if err := f.ca.Apply(ctx, cr); err != nil {
			return nil, errors.Wrap(err, errUpdate)
		}
	}

	return AsComposition(latest), nil
}

// AsComposition creates a new composition from the supplied revision. It
// exists only as a temporary translation layer to allow us to introduce the
// alpha CompositionRevision type with minimal changes to the XR reconciler.
// Once CompositionRevision leaves alpha this code should be removed and the XR
// reconciler should operate on CompositionRevisions instead.
func AsComposition(cr *v1.CompositionRevision) *v1.Composition {
	conv := &v1.GeneratedRevisionSpecConverter{}
	return &v1.Composition{Spec: conv.FromRevisionSpec(cr.Spec)}
}

func (f *APIRevisionFetcher) getCompositionRevisionList(ctx context.Context, cr resource.Composite, comp *v1.Composition) (*v1.CompositionRevisionList, error) {
	rl := &v1.CompositionRevisionList{}
	ml := client.MatchingLabels{}

	if cr.GetCompositionUpdatePolicy() != nil && *cr.GetCompositionUpdatePolicy() == xpv1.UpdateAutomatic &&
		cr.GetCompositionRevisionSelector() != nil {
		ml = cr.GetCompositionRevisionSelector().MatchLabels
	}

	ml[v1beta1.LabelCompositionName] = comp.GetName()
	if err := f.ca.List(ctx, rl, ml); err != nil {
		return nil, errors.Wrap(err, errListCompositionRevisions)
	}
	return rl, nil
}

// NewCompositionSelectorChain returns a new CompositionSelectorChain.
func NewCompositionSelectorChain(list ...CompositionSelector) *CompositionSelectorChain {
	return &CompositionSelectorChain{list: list}
}

// CompositionSelectorChain calls the given list of CompositionSelectors in order.
type CompositionSelectorChain struct {
	list []CompositionSelector
}

// SelectComposition calls all SelectComposition functions of CompositionSelectors
// in the list.
func (r *CompositionSelectorChain) SelectComposition(ctx context.Context, cp resource.Composite) error {
	for _, cs := range r.list {
		if err := cs.SelectComposition(ctx, cp); err != nil {
			return err
		}
	}
	return nil
}

// NewAPILabelSelectorResolver returns a SelectorResolver for composite resource.
func NewAPILabelSelectorResolver(c client.Client) *APILabelSelectorResolver {
	return &APILabelSelectorResolver{client: c}
}

// APILabelSelectorResolver is used to resolve the composition selector on the instance
// to composition reference.
type APILabelSelectorResolver struct {
	client client.Client
}

// SelectComposition resolves selector to a reference if it doesn't exist.
func (r *APILabelSelectorResolver) SelectComposition(ctx context.Context, cp resource.Composite) error {
	// TODO(muvaf): need to block the deletion of composition via finalizer once
	// it's selected since it's integral to this resource.
	// TODO(muvaf): We don't rely on UID in practice. It should not be there
	// because it will make confusion if the resource is backed up and restored
	// to another cluster
	if cp.GetCompositionReference() != nil {
		return nil
	}
	labels := map[string]string{}
	sel := cp.GetCompositionSelector()
	if sel != nil {
		labels = sel.MatchLabels
	}
	list := &v1.CompositionList{}
	if err := r.client.List(ctx, list, client.MatchingLabels(labels)); err != nil {
		return errors.Wrap(err, errListCompositions)
	}

	candidates := make([]string, 0, len(list.Items))
	v, k := cp.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()

	for _, comp := range list.Items {
		if comp.Spec.CompositeTypeRef.APIVersion == v && comp.Spec.CompositeTypeRef.Kind == k {
			// This composition is compatible with our composite resource.
			candidates = append(candidates, comp.Name)
		}
	}

	if len(candidates) == 0 {
		return errors.New(errNoCompatibleComposition)
	}

	// We don't need this choice to be cryptographically random.
	random := rand.New(rand.NewSource(time.Now().UnixNano())) // nolint:gosec
	selected := candidates[random.Intn(len(candidates))]
	cp.SetCompositionReference(&corev1.ObjectReference{Name: selected})
	return errors.Wrap(r.client.Update(ctx, cp), errUpdateComposite)
}

// NewAPIDefaultCompositionSelector returns a APIDefaultCompositionSelector.
func NewAPIDefaultCompositionSelector(c client.Client, ref corev1.ObjectReference, r event.Recorder) *APIDefaultCompositionSelector {
	return &APIDefaultCompositionSelector{client: c, defRef: ref, recorder: r}
}

// APIDefaultCompositionSelector selects the default composition referenced in
// the definition of the resource if neither a reference nor selector is given
// in composite resource.
type APIDefaultCompositionSelector struct {
	client   client.Client
	defRef   corev1.ObjectReference
	recorder event.Recorder
}

// SelectComposition selects the default compositionif neither a reference nor
// selector is given in composite resource.
func (s *APIDefaultCompositionSelector) SelectComposition(ctx context.Context, cp resource.Composite) error {
	if cp.GetCompositionReference() != nil || cp.GetCompositionSelector() != nil {
		return nil
	}
	def := &v1.CompositeResourceDefinition{}
	if err := s.client.Get(ctx, meta.NamespacedNameOf(&s.defRef), def); err != nil {
		return errors.Wrap(err, errGetXRD)
	}
	if def.Spec.DefaultCompositionRef == nil {
		return nil
	}
	cp.SetCompositionReference(&corev1.ObjectReference{Name: def.Spec.DefaultCompositionRef.Name})
	s.recorder.Event(cp, event.Normal(reasonCompositionSelection, "Default composition has been selected"))
	return nil
}

// NewEnforcedCompositionSelector returns a EnforcedCompositionSelector.
func NewEnforcedCompositionSelector(def v1.CompositeResourceDefinition, r event.Recorder) *EnforcedCompositionSelector {
	return &EnforcedCompositionSelector{def: def, recorder: r}
}

// EnforcedCompositionSelector , if it's given, selects the enforced composition
// on the definition for all composite instances.
type EnforcedCompositionSelector struct {
	def      v1.CompositeResourceDefinition
	recorder event.Recorder
}

// SelectComposition selects the enforced composition if it's given in definition.
func (s *EnforcedCompositionSelector) SelectComposition(_ context.Context, cp resource.Composite) error {
	// We don't need to fetch the CompositeResourceDefinition at every reconcile
	// because enforced composition ref is immutable as opposed to default
	// composition ref.
	if s.def.Spec.EnforcedCompositionRef == nil {
		return nil
	}
	// If the composition is already chosen, we don't need to check for compatibility
	// as its target type reference is immutable.
	if cp.GetCompositionReference() != nil && cp.GetCompositionReference().Name == s.def.Spec.EnforcedCompositionRef.Name {
		return nil
	}
	cp.SetCompositionReference(&corev1.ObjectReference{Name: s.def.Spec.EnforcedCompositionRef.Name})
	s.recorder.Event(cp, event.Normal(reasonCompositionSelection, "Enforced composition has been selected"))
	return nil
}

// NewConfiguratorChain returns a new *ConfiguratorChain.
func NewConfiguratorChain(l ...Configurator) *ConfiguratorChain {
	return &ConfiguratorChain{list: l}
}

// ConfiguratorChain executes the Configurators in given order.
type ConfiguratorChain struct {
	list []Configurator
}

// Configure calls Configure function of every Configurator in the list.
func (cc *ConfiguratorChain) Configure(ctx context.Context, cp resource.Composite, comp *v1.Composition) error {
	for _, c := range cc.list {
		if err := c.Configure(ctx, cp, comp); err != nil {
			return err
		}
	}
	return nil
}

// NewAPIConfigurator returns a Configurator that configures a
// composite resource using its composition.
func NewAPIConfigurator() *APIConfigurator {
	return &APIConfigurator{}
}

// An APIConfigurator configures a composite resource using its
// composition.
type APIConfigurator struct{}

// Configure any required fields that were omitted from the composite resource
// by copying them from its composition.
func (c *APIConfigurator) Configure(ctx context.Context, cp resource.Composite, comp *v1.Composition) error {
	apiVersion, kind := cp.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	if comp.Spec.CompositeTypeRef.APIVersion != apiVersion || comp.Spec.CompositeTypeRef.Kind != kind {
		return errors.New(errCompositionNotCompatible)
	}

	// NOTE (@tnthornton) this is custom logic for validation with xpls. Given
	// that we aren't working with a "real" extant resource.Composite, we're
	// filling in these fields in order for them to exist if the upstream patch
	// is looking for these details as a configuration source.
	cp.SetWriteConnectionSecretToReference(&xpv1.SecretReference{
		Name:      string(cp.GetUID()),
		Namespace: PlaceholderNamespace,
	})

	return nil
}

// NewAPINamingConfigurator returns a Configurator that sets the root name prefixKu
// to its own name if it is not already set.
func NewAPINamingConfigurator() *APINamingConfigurator {
	return &APINamingConfigurator{}
}

// An APINamingConfigurator sets the root name prefix to its own name if it is not
// already set.
type APINamingConfigurator struct{}

// Configure the supplied composite resource's root name prefix.
func (c *APINamingConfigurator) Configure(ctx context.Context, cp resource.Composite, _ *v1.Composition) error {
	if cp.GetLabels()[xcrd.LabelKeyNamePrefixForComposed] != "" {
		return nil
	}
	meta.AddLabels(cp, map[string]string{xcrd.LabelKeyNamePrefixForComposed: cp.GetName()})
	return nil
}
