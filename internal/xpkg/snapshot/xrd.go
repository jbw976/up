// Copyright 2025 Upbound Inc.
// All rights reserved

package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kube-openapi/pkg/validation/validate"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/xcrd"

	"github.com/upbound/up/internal/xpkg/snapshot/validator"
)

var mapKeyRE = regexp.MustCompile(`(\[([a-zA-Z]*)\])`)

// XRDValidator defines a validator for xrd files.
type XRDValidator struct {
	validators []xrdValidator
}

// DefaultXRDValidators returns a new Meta validator.
func DefaultXRDValidators() (validator.Validator, error) {
	validators := []xrdValidator{
		NewXRDSchemaValidator(),
	}

	return &XRDValidator{
		validators: validators,
	}, nil
}

// Validate performs validation rules on the given data input per the rules
// defined for the Validator.
func (x *XRDValidator) Validate(ctx context.Context, data any) *validate.Result {
	xrd, err := x.Marshal(data)
	if err != nil {
		// TODO(@tnthornton) add debug logging
		return validator.Nop
	}

	errs := make([]error, 0)

	for _, v := range x.validators {
		errs = append(errs, v.validate(ctx, xrd)...)
	}

	return &validate.Result{
		Errors: errs,
	}
}

// Marshal marshals the given data object into a Pkg, errors otherwise.
func (x *XRDValidator) Marshal(data any) (*xpextv1.CompositeResourceDefinition, error) {
	u, ok := data.(*unstructured.Unstructured)
	if !ok {
		return nil, errors.New("invalid type passed in, expected Unstructured")
	}

	b, err := u.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var xrd xpextv1.CompositeResourceDefinition
	err = json.Unmarshal(b, &xrd)
	if err != nil {
		return nil, err
	}

	return &xrd, nil
}

type xrdValidator interface {
	validate(ctx context.Context, xrd *xpextv1.CompositeResourceDefinition) []error
}

// XRDSchemaValidator validates XRD schema definitions.
type XRDSchemaValidator struct{}

// NewXRDSchemaValidator returns a new XRDSchemaValidator.
func NewXRDSchemaValidator() *XRDSchemaValidator {
	return &XRDSchemaValidator{}
}

func (v *XRDSchemaValidator) validate(ctx context.Context, xrd *xpextv1.CompositeResourceDefinition) []error {
	errs := validateOpenAPIV3Schema(ctx, xrd)

	errList := []error{}

	for _, e := range errs {
		var fe *field.Error
		if errors.As(e, &fe) {
			fieldValue := fe.Field

			path := cleanFieldPath(fieldValue)
			errList = append(errList, &validator.ValidationError{
				TypeCode: validator.ErrorTypeCode,
				Name:     path,
				Message:  fmt.Sprintf("%s %s", path, fe.ErrorBody()),
			},
			)
		}
	}

	return errList
}

// validateOpenAPIV3Schema validates the spec.versions[*].schema.openAPIV3Schema
// section of the given XRD definition.
func validateOpenAPIV3Schema(ctx context.Context, xrd *xpextv1.CompositeResourceDefinition) []error {
	// we need to set UID
	// ValidateCustomResourceDefinition is checking ownerReference and UID
	// ref: https://github.com/kubernetes/apimachinery/blob/6a84120b17236460f404ba6486926f62cbf733dd/pkg/api/validation/objectmeta.go#L82-L84
	if xrd.UID == "" {
		xrd.UID = types.UID("dummy-uid")
	}
	crd, err := xcrd.ForCompositeResource(xrd)
	if err != nil {
		return nil
	}

	extv1.SetObjectDefaults_CustomResourceDefinition(crd)

	internal := &apiextensions.CustomResourceDefinition{}
	if err := extv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(crd, internal, nil); err != nil {
		return nil
	}
	// TODO(hasheddan): propagate actual context to validation.
	felist := validation.ValidateCustomResourceDefinition(ctx, internal)
	if felist != nil {
		return felist.ToAggregate().Errors()
	}
	return nil
}

func cleanFieldPath(fieldVal string) string {
	fns := []cleaner{
		replaceValidation,
		replaceMapKeys,
		trimInvalidDollarSign,
	}

	cleaned := fieldVal
	for _, f := range fns {
		cleaned = f(cleaned)
	}

	return cleaned
}

type cleaner func(string) string

// if the validations were all moved to spec.validation, update the path
// to point to spec.version[0].
func replaceValidation(fieldVal string) string {
	return strings.Replace(fieldVal, "spec.validation", "spec.versions[0].schema", 1)
}

// paths are returned from CRD validations using map[key].field notation.
func replaceMapKeys(fieldVal string) string {
	return mapKeyRE.ReplaceAllString(fieldVal, ".$2")
}

func trimInvalidDollarSign(fieldVal string) string {
	if idx := strings.Index(fieldVal, ".$"); idx != -1 {
		return fieldVal[:idx]
	}
	return fieldVal
}
