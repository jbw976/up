/*
Copyright 2020 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package composite

import (
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/names"
)

// Error strings
const (
	errGetComposed  = "cannot get composed resource"
	errGCComposed   = "cannot garbage collect composed resource"
	errFetchDetails = "cannot fetch connection details"
	errInline       = "cannot inline Composition patch sets"

	errFmtApplyComposed              = "cannot apply composed resource %q"
	errFmtParseBase                  = "cannot parse base template of composed resource %q"
	errFmtRenderFromCompositePatches = "cannot render FromComposite patches for composed resource %q"
	errFmtRenderToCompositePatches   = "cannot render ToComposite patches for composed resource %q"
	errFmtRenderMetadata             = "cannot render metadata for composed resource %q"
	errFmtGenerateName               = "cannot generate a name for composed resource %q"
	errFmtExtractDetails             = "cannot extract composite resource connection details from composed resource %q"
	errFmtCheckReadiness             = "cannot check whether composed resource %q is ready"
)

// TODO(negz): Move P&T Composition logic into its own package?

// A PTComposerOption is used to configure a PTComposer.
type PTComposerOption func(*PTComposer)

// WithTemplateAssociator configures how a PatchAndTransformComposer associates
// templates with extant composed resources.
func WithTemplateAssociator(a CompositionTemplateAssociator) PTComposerOption {
	return func(c *PTComposer) {
		c.composition = a
	}
}

// WithComposedNameGenerator configures how the PTComposer should generate names
// for unnamed composed resources.
func WithComposedNameGenerator(r names.NameGenerator) PTComposerOption {
	return func(c *PTComposer) {
		c.composed.NameGenerator = r
	}
}

// WithComposedReadinessChecker configures how a PatchAndTransformComposer
// checks composed resource readiness.
func WithComposedReadinessChecker(r ReadinessChecker) PTComposerOption {
	return func(c *PTComposer) {
		c.composed.ReadinessChecker = r
	}
}

// WithComposedConnectionDetailsFetcher configures how a
// PatchAndTransformComposer fetches composed resource connection details.
func WithComposedConnectionDetailsFetcher(f managed.ConnectionDetailsFetcher) PTComposerOption {
	return func(c *PTComposer) {
		c.composed.ConnectionDetailsFetcher = f
	}
}

// WithComposedConnectionDetailsExtractor configures how a
// PatchAndTransformComposer extracts XR connection details from a composed
// resource.
func WithComposedConnectionDetailsExtractor(e ConnectionDetailsExtractor) PTComposerOption {
	return func(c *PTComposer) {
		c.composed.ConnectionDetailsExtractor = e
	}
}

type composedResource struct {
	names.NameGenerator
	managed.ConnectionDetailsFetcher
	ConnectionDetailsExtractor
	ReadinessChecker
}

// A PTComposer composes resources using Patch and Transform (P&T) Composition.
// It uses a Composition's 'resources' array, which consist of 'base' resources
// along with a series of patches and transforms. It does not support Functions
// - any entries in the functions array are ignored.
type PTComposer struct {
	composition CompositionTemplateAssociator
	composed    composedResource
}

// NewPTComposer returns a Composer that composes resources using Patch and
// Transform (P&T) Composition - a Composition's bases, patches, and transforms.
func NewPTComposer(kube client.Client, o ...PTComposerOption) *PTComposer {
	c := &PTComposer{

		composition: NewGarbageCollectingAssociator(),
		composed: composedResource{
			NameGenerator:              names.NewNameGenerator(kube),
			ReadinessChecker:           ReadinessCheckerFn(IsReady),
			ConnectionDetailsFetcher:   NewSecretConnectionDetailsFetcher(kube),
			ConnectionDetailsExtractor: ConnectionDetailsExtractorFn(ExtractConnectionDetails),
		},
	}

	for _, fn := range o {
		fn(c)
	}

	return c
}

// Compose resources using the bases, patches, and transforms specified by the
// supplied Composition.
func (c *PTComposer) Compose(ctx context.Context, xr *composite.Unstructured, req CompositionRequest) ([]ComposedResourceState, error) { //nolint:gocyclo // Breaking this up doesn't seem worth yet more layers of abstraction.
	// Inline PatchSets from Composition Spec before composing resources.
	ct, err := ComposedTemplates(req.Revision.Spec.PatchSets, req.Revision.Spec.Resources)
	if err != nil {
		return nil, errors.Wrap(err, errInline)
	}

	tas, err := c.composition.AssociateTemplates(ctx, xr, ct)
	if err != nil {
		return nil, errors.Wrap(err, errAssociate)
	}

	events := make([]TargetedEvent, 0)

	// We optimistically render all composed resources that we are able to with
	// the expectation that any that we fail to render will subsequently have
	// their error corrected by manual intervention or propagation of a required
	// input. Errors are recorded, but not considered fatal to the composition
	// process.
	refs := make([]corev1.ObjectReference, len(tas))
	cds := make([]ComposedResourceState, len(tas))
	for i := range tas {
		ta := tas[i]

		// If this resource is anonymous its "name" is just its index.
		name := ptr.Deref(ta.Template.Name, strconv.Itoa(i))
		r := composed.New(composed.FromReference(ta.Reference))

		if err := RenderFromJSON(r, ta.Template.Base.Raw); err != nil {
			// We consider this a terminal error, since it indicates a broken
			// CompositionRevision that will never be valid.
			return nil, errors.Wrapf(err, errFmtParseBase, name)
		}

		// Failures to patch aren't terminal - we just emit a warning event and
		// move on. This is because patches often fail because other patches
		// need to happen first in order for them to succeed. If we returned an
		// error when a patch failed we might never reach the patch that would
		// unblock it.

		rendered := true
		if err := RenderFromCompositePatches(r, xr, ta.Template.Patches); err != nil {
			events = append(events, TargetedEvent{
				Event:  event.Warning(reasonCompose, errors.Wrapf(err, errFmtRenderFromCompositePatches, name)),
				Target: CompositionTargetComposite,
			})
			rendered = false
		}

		if err := RenderComposedResourceMetadata(r, xr, ResourceName(ptr.Deref(ta.Template.Name, ""))); err != nil {
			events = append(events, TargetedEvent{
				Event:  event.Warning(reasonCompose, errors.Wrapf(err, errFmtRenderMetadata, name)),
				Target: CompositionTargetComposite,
			})
			rendered = false
		}

		if err := c.composed.GenerateName(ctx, r); err != nil {
			events = append(events, TargetedEvent{
				Event:  event.Warning(reasonCompose, errors.Wrapf(err, errFmtGenerateName, name)),
				Target: CompositionTargetComposite,
			})
			rendered = false
		}

		// We record a reference even if we didn't render the resource because
		// if it already exists we don't want to drop our reference to it (and
		// thus not know about it next reconcile). If we're using anonymous
		// resource templates we also need to record a reference even if it's
		// empty, so that our XR's spec.resourceRefs remains the same length and
		// order as our CompositionRevisions's array of templates.
		refs[i] = *meta.ReferenceTo(r, r.GetObjectKind().GroupVersionKind())

		// We only need the composed resource if it rendered correctly.
		if rendered {
			cds[i] = ComposedResourceState{Resource: r}
		}
	}

	// We persist references to our composed resources before we create
	// them. This way we can render composed resources with
	// non-deterministic names, and also potentially recover from any errors
	// we encounter while applying composed resources without leaking them.
	xr.SetResourceReferences(refs)

	// We apply all of our composed resources before we observe them and update
	// in the loop below. This ensures that issues observing and processing one
	// composed resource won't block the application of another.

	return cds, nil
}

// toXRPatchesFromTAs selects patches defined in composed templates,
// whose type is one of the XR-targeting patches
// (e.g. v1.PatchTypeToCompositeFieldPath or v1.PatchTypeCombineToComposite)
func toXRPatchesFromTAs(tas []TemplateAssociation) []v1.Patch {
	filtered := make([]v1.Patch, 0, len(tas))
	for _, ta := range tas {
		filtered = append(filtered, filterPatches(ta.Template.Patches,
			patchTypesToXR()...)...)
	}
	return filtered
}

// filterPatches selects patches whose type belong to the list onlyTypes
func filterPatches(pas []v1.Patch, onlyTypes ...v1.PatchType) []v1.Patch {
	filtered := make([]v1.Patch, 0, len(pas))
	include := make(map[v1.PatchType]bool)
	for _, t := range onlyTypes {
		include[t] = true
	}
	for _, p := range pas {
		if include[p.Type] {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// A TemplateAssociation associates a composed resource template with a composed
// resource. If no such resource exists the reference will be empty.
type TemplateAssociation struct {
	Template  v1.ComposedTemplate
	Reference corev1.ObjectReference
}

// AssociateByOrder associates the supplied templates with the supplied resource
// references by order; i.e. by assuming template n corresponds to reference n.
// The returned array will always be of the same length as the supplied array of
// templates. Any additional references will be truncated.
func AssociateByOrder(t []v1.ComposedTemplate, r []corev1.ObjectReference) []TemplateAssociation {
	a := make([]TemplateAssociation, len(t))
	for i := range t {
		a[i] = TemplateAssociation{Template: t[i]}
	}

	j := len(t)
	if len(r) < j {
		j = len(r)
	}

	for i := 0; i < j; i++ {
		a[i].Reference = r[i]
	}

	return a
}

// A CompositionTemplateAssociator returns an array of template associations.
type CompositionTemplateAssociator interface {
	AssociateTemplates(context.Context, resource.Composite, []v1.ComposedTemplate) ([]TemplateAssociation, error)
}

// A CompositionTemplateAssociatorFn returns an array of template associations.
type CompositionTemplateAssociatorFn func(context.Context, resource.Composite, []v1.ComposedTemplate) ([]TemplateAssociation, error)

// AssociateTemplates with composed resources.
func (fn CompositionTemplateAssociatorFn) AssociateTemplates(ctx context.Context, cr resource.Composite, ct []v1.ComposedTemplate) ([]TemplateAssociation, error) {
	return fn(ctx, cr, ct)
}

// A GarbageCollectingAssociator associates a Composition's resource templates
// with (references to) composed resources. It tries to associate them by
// checking the template name annotation of each referenced resource. If any
// template or existing composed resource can't be associated by name it falls
// back to associating them by order. If it encounters a referenced resource
// that corresponds to a non-existent template the resource will be garbage
// collected (i.e. deleted).
type GarbageCollectingAssociator struct {
}

// NewGarbageCollectingAssociator returns a CompositionTemplateAssociator that
// may garbage collect composed resources.
func NewGarbageCollectingAssociator() *GarbageCollectingAssociator {
	return &GarbageCollectingAssociator{}
}

// AssociateTemplates with composed resources.
func (a *GarbageCollectingAssociator) AssociateTemplates(ctx context.Context, cr resource.Composite, ct []v1.ComposedTemplate) ([]TemplateAssociation, error) { //nolint:gocyclo // Only slightly over (13).
	templates := map[string]int{}
	for i, t := range ct {
		if t.Name == nil {
			// If our templates aren't named we fall back to assuming that the
			// existing resource reference array (if any) already matches the
			// order of our resource template array.
			return AssociateByOrder(ct, cr.GetResourceReferences()), nil
		}
		templates[*t.Name] = i
	}

	tas := make([]TemplateAssociation, len(ct))
	for i := range ct {
		tas[i] = TemplateAssociation{Template: ct[i]}
	}

	return tas, nil
}

// Observation is the result of composed reconciliation.
type Observation struct {
	Ref               corev1.ObjectReference
	ConnectionDetails managed.ConnectionDetails
	Ready             bool
}
