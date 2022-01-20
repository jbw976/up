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

package validator

import (
	"k8s.io/kube-openapi/pkg/validation/validate"
)

// Nop is used for no-op validator results.
var Nop = &validate.Result{}

// A Validator validates data and returns a validation result.
type Validator interface {
	Validate(data interface{}) *validate.Result
}

// MetaValidation represents a failure of a meta file condition.
type MetaValidation struct {
	code    int32
	Message string
	Name    string
}

// Code returns the code corresponding to the MetaValidation.
func (e *MetaValidation) Code() int32 {
	return e.code
}

func (e *MetaValidation) Error() string {
	return e.Message
}

// ObjectValidator provides a mechanism for grouping various validators for a
// given object type.
type ObjectValidator struct {
	validators []Validator
}

// New returns a new ObjectValidator.
func New(schemaValidator Validator, others ...Validator) *ObjectValidator {
	validators := []Validator{schemaValidator}

	return &ObjectValidator{
		validators: append(validators, others...),
	}
}

// Validate implements the validator.Validator interface, providing a way to
// validate more than just the strict schema for a given runtime.Object.
func (o *ObjectValidator) Validate(data interface{}) *validate.Result {
	errs := make([]error, 0)

	for _, v := range o.validators {
		result := v.Validate(data)
		errs = append(errs, result.Errors...)
	}

	return &validate.Result{
		Errors: errs,
	}
}