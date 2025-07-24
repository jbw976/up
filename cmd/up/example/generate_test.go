// Copyright 2025 Upbound Inc.
// All rights reserved

package example

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v2 "github.com/crossplane/crossplane/apis/apiextensions/v2"

	_ "embed"
)

// Embed XRD YAML files
//
//go:embed testdata/xeks-xrd-definition.yaml
var xeksXRDYAML []byte

// Embed XRDv2 YAML files
//
//go:embed testdata/xeks-xrd2-namespaces-definition.yaml
var v2XRDNamespacedYAML []byte

// Embed XRDv2 YAML files
//
//go:embed testdata/xeks-xrd2-cluster-definition.yaml
var v2XRDClusterYAML []byte

func TestCreateResource(t *testing.T) {
	type want struct {
		res resource
		err bool
	}

	cases := map[string]struct {
		resourceType  string
		compositeName string
		apiGroup      string
		apiVersion    string
		name          string
		namespace     string
		want          want
	}{
		"ValidXRCResource": {
			resourceType:  "xrc",
			compositeName: "Cluster",
			apiGroup:      "customer.upbound.io",
			apiVersion:    "v1alpha1",
			name:          "cluster",
			namespace:     "default",
			want: want{
				res: resource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "customer.upbound.io/v1alpha1",
						Kind:       "Cluster",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster",
						Namespace: "default",
					},
					Spec: map[string]interface{}{},
				},
			},
		},
		"ValidXRResource": {
			resourceType:  "xr",
			compositeName: "XCluster",
			apiGroup:      "customer.upbound.io",
			apiVersion:    "v1alpha1",
			name:          "cluster",
			namespace:     "",
			want: want{
				res: resource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "customer.upbound.io/v1alpha1",
						Kind:       "XCluster",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: map[string]interface{}{},
				},
			},
		},
		"ValidNamespacedXRResource": {
			resourceType:  "xr",
			compositeName: "XCluster",
			apiGroup:      "customer.upbound.io",
			apiVersion:    "v1alpha1",
			name:          "cluster",
			namespace:     "test-namespace",
			want: want{
				res: resource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "customer.upbound.io/v1alpha1",
						Kind:       "XCluster",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster",
						Namespace: "test-namespace",
					},
					Spec: map[string]interface{}{},
				},
			},
		},
		"ValidClusterXRResource": {
			resourceType:  "xr",
			compositeName: "XCluster",
			apiGroup:      "customer.upbound.io",
			apiVersion:    "v1alpha1",
			name:          "cluster",
			want: want{
				res: resource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "customer.upbound.io/v1alpha1",
						Kind:       "XCluster",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: map[string]interface{}{},
				},
			},
		},
		"EmptyCompositeName": {
			resourceType:  "xrc",
			compositeName: "",
			apiGroup:      "customer.upbound.io",
			apiVersion:    "v1alpha1",
			name:          "cluster",
			namespace:     "default",
			want: want{
				res: resource{},
				err: true,
			},
		},
		"EmptyAPIGroup": {
			resourceType:  "xrc",
			compositeName: "Cluster",
			apiGroup:      "",
			apiVersion:    "v1alpha1",
			name:          "cluster",
			namespace:     "default",
			want: want{
				res: resource{},
				err: true,
			},
		},
		"EmptyResourceType": {
			resourceType:  "",
			compositeName: "Cluster",
			apiGroup:      "customer.upbound.io",
			apiVersion:    "v1alpha1",
			name:          "cluster",
			namespace:     "default",
			want: want{
				res: resource{},
				err: true,
			},
		},
		"InvalidAPIVersion": {
			resourceType:  "xrc",
			compositeName: "Cluster",
			apiGroup:      "customer.upbound.io",
			apiVersion:    "invalid-version",
			name:          "cluster",
			namespace:     "default",
			want: want{
				res: resource{},
				err: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cmd := &generateCmd{}
			got, err := cmd.createResource(tc.resourceType, tc.compositeName, tc.apiGroup, tc.apiVersion, tc.name, tc.namespace)

			// Check if an error was expected and occurred
			if tc.want.err {
				if err == nil {
					t.Errorf("Expected an error but got none for test case %s", name)
				}
				return // Skip further checks if we expected an error
			}

			// Ensure no unexpected error occurred
			if err != nil {
				t.Errorf("Unexpected error for test case %s: %v", name, err)
			}

			// Compare the output resource
			if diff := cmp.Diff(got, tc.want.res); diff != "" {
				t.Errorf("createResource() -got, +want:\n%s", diff)
			}
		})
	}
}

func TestCreateCRDAndGenerateResource(t *testing.T) {
	type want struct {
		res resource
		err string
	}

	// Unmarshal the embedded XRD YAML into a CompositeResourceDefinition object
	var v1XRD v1.CompositeResourceDefinition
	err := yaml.Unmarshal(xeksXRDYAML, &v1XRD)
	assert.NilError(t, err, "Failed to unmarshal v1 sample XRD")

	// Unmarshal the v2 Namespaced XRD
	var v2XRDNamespaced v2.CompositeResourceDefinition
	err = yaml.Unmarshal(v2XRDNamespacedYAML, &v2XRDNamespaced)
	assert.NilError(t, err, "Failed to unmarshal v2 sample XRD")

	var v2XRDCluster v2.CompositeResourceDefinition
	err = yaml.Unmarshal(v2XRDClusterYAML, &v2XRDCluster)
	assert.NilError(t, err, "Failed to unmarshal v2 sample XRD")

	cases := map[string]struct {
		xrd          interface{}
		resourceType string
		want         want
	}{
		"V1XRCGeneration": {
			xrd:          v1XRD,
			resourceType: "xrc",
			want: want{
				err: "cannot derive composite CRD from v1 XRD",
			},
		},
		"V1XRGeneration": {
			xrd:          v1XRD,
			resourceType: "xr",
			want: want{
				res: resource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "aws.platform.upbound.io/v1alpha1",
						Kind:       "XEKS",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xeks",
					},
					Spec: map[string]interface{}{
						"parameters": map[string]interface{}{
							"deletionPolicy": "Delete",
							"id":             "string",
							"nodes": map[string]interface{}{
								"count":        float64(1),
								"instanceType": "t3.small",
							},
							"providerConfigName": "default",
							"region":             "string",
						},
					},
				},
				err: "",
			},
		},
		"V2XRCGeneration": {
			xrd:          v2XRDNamespaced,
			resourceType: "xrc",
			want: want{
				err: "v2 XRDs only support Composite Resources",
			},
		},
		"V2XRNamespacedGeneration": {
			xrd:          v2XRDNamespaced,
			resourceType: "xr",
			want: want{
				res: resource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "aws.platform.upbound.io/v1alpha1",
						Kind:       "XEKS",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "xeks",
						Namespace: "default", // Should be namespace-scoped based on XRD scope
					},
					Spec: map[string]interface{}{
						"parameters": map[string]interface{}{
							"region": "string",
							"nodes": map[string]interface{}{
								"count":        float64(1),
								"instanceType": "t3.small",
							},
						},
					},
				},
				err: "",
			},
		},
		"V2XRClusterGeneration": {
			xrd:          v2XRDCluster,
			resourceType: "xr",
			want: want{
				res: resource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "aws.platform.upbound.io/v1alpha1",
						Kind:       "XEKS",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xeks",
					},
					Spec: map[string]interface{}{
						"parameters": map[string]interface{}{
							"region": "string",
							"nodes": map[string]interface{}{
								"count":        float64(1),
								"instanceType": "t3.small",
							},
						},
					},
				},
				err: "",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cmd := &generateCmd{Type: tc.resourceType}

			gotCRD, err := cmd.createCRDFromXRD(tc.xrd)

			if tc.want.err != "" {
				assert.ErrorContains(t, err, tc.want.err)
				return
			}

			assert.NilError(t, err, "Failed to create CRD from XRD")

			gotRes, err := cmd.generateResourceFromCRD(gotCRD)
			assert.NilError(t, err, "Failed to generate resource from CRD")

			assert.DeepEqual(t, gotRes, tc.want.res)
		})
	}
}
