// Copyright 2025 Upbound Inc.
// All rights reserved

package composition

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/meta"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
)

// NewCompositionRevision creates a new revision of the supplied Composition.
func NewCompositionRevision(c *v1.Composition, revision int64) *v1.CompositionRevision {
	hash := c.Hash()

	cr := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", c.GetName(), hash[0:7]),
			Labels: map[string]string{
				v1beta1.LabelCompositionName: c.GetName(),
				// We cannot have a label value longer than 63 chars
				// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
				v1beta1.LabelCompositionHash: hash[0:63],
			},
		},
		Spec: NewCompositionRevisionSpec(c.Spec, revision),
	}

	ref := meta.TypedReferenceTo(c, v1.CompositionGroupVersionKind)
	meta.AddOwnerReference(cr, meta.AsController(ref))

	for k, v := range c.GetLabels() {
		cr.ObjectMeta.Labels[k] = v
	}

	return cr
}

// NewCompositionRevisionSpec translates a composition's spec to a composition
// revision spec.
func NewCompositionRevisionSpec(cs v1.CompositionSpec, revision int64) v1.CompositionRevisionSpec {
	conv := v1.GeneratedRevisionSpecConverter{}
	rs := conv.ToRevisionSpec(cs)
	rs.Revision = revision
	return rs
}
