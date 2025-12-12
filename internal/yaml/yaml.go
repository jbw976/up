// Copyright 2025 Upbound Inc.
// All rights reserved

// Package yaml is an opinionated drop-in replacement for the k8s yaml library,
// offering some additional functionality.
package yaml

import (
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

// Marshal uses the Kubernetes yaml library to marshal the given object to YAML,
// first removing the metadata.creationTimestamp field if it is present and
// null. Additional fields may be removed by passing options.
func Marshal(obj any, opts ...MarshalOption) ([]byte, error) {
	cfg := marshalOptions{
		removeNilFields: []fieldPath{
			{"metadata", "creationTimestamp"},
		},
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	typ := reflect.TypeOf(obj)
	switch typ.Kind() { //nolint:exhaustive // We have a perfectly good default case.
	case reflect.Pointer:
		// Don't need to do anything.
	case reflect.Array, reflect.Slice, reflect.Map:
		// ToUnstructured doesn't work on these.
		return yaml.Marshal(obj)
	default:
		// Take a pointer so we can convert to unstructured. We have to use
		// ptr.To here instead of just taking the address because obj is a stack
		// variable.
		obj = ptr.To(obj)
	}

	unst, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	for _, field := range cfg.removeFields {
		unstructured.RemoveNestedField(unst, field...)
	}
	for _, field := range cfg.removeNilFields {
		val, found, err := unstructured.NestedFieldNoCopy(unst, field...)
		if err != nil {
			return nil, err
		}
		if found && val == nil {
			unstructured.RemoveNestedField(unst, field...)
		}
	}

	return yaml.Marshal(unst)
}

// MarshalOption is an option to influence the behavior of Marshal.
type MarshalOption func(*marshalOptions)

type marshalOptions struct {
	removeFields    []fieldPath
	removeNilFields []fieldPath
}

type fieldPath []string

// RemoveField causes Marshal to remove the given field from the object before
// marshaling. Field paths are dot-separated without a leading dot.
func RemoveField(path string) MarshalOption {
	return func(opts *marshalOptions) {
		fieldPath := strings.Split(path, ".")
		opts.removeFields = append(opts.removeFields, fieldPath)
	}
}

// RemoveFieldIfNil causes Marshal to remove the given field from the object
// before marshaling if its value is nil. Field paths are dot-separated without
// a leading dot.
func RemoveFieldIfNil(path string) MarshalOption {
	return func(opts *marshalOptions) {
		fieldPath := strings.Split(path, ".")
		opts.removeNilFields = append(opts.removeNilFields, fieldPath)
	}
}

// Unmarshal wraps the Kubernetes yaml package's Unmarshal, allowing this
// package to serve as a drop-in replacement for the upstream package in most
// cases.
func Unmarshal(y []byte, obj any, opts ...yaml.JSONOpt) error {
	return yaml.Unmarshal(y, obj, opts...)
}

// YAMLToJSON wraps the Kubernetes yaml package's YAMLToJSON, allowing this
// package to serve as a drop-in replacement for the upstream package in most
// cases.
//
//nolint:revive // Name matches the upstream package.
func YAMLToJSON(y []byte) ([]byte, error) {
	return yaml.YAMLToJSON(y)
}
