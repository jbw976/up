// Copyright 2025 Upbound Inc.
// All rights reserved

package query

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func TestNativeResources(t *testing.T) {
	scheme := kruntime.NewScheme()
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add apiextensionsv1 to scheme: %v", err)
	}
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add Kubernetes types to scheme: %v", err)
	}

	missingKinds := sets.New[string]()
	special := sets.New[string](
		"WatchEvent",
		"Scale",
		"Status",
		"APIGroup",
		"APIVersion",
		"APIVersions",
	)
	for gvk := range scheme.AllKnownTypes() {
		if strings.HasSuffix(gvk.Kind, "List") || special.Has(gvk.Kind) || strings.HasSuffix(gvk.Kind, "Options") {
			continue
		}
		if _, found := nativeResources[gvk.Kind]; !found {
			missingKinds.Insert(fmt.Sprintf(`"%s":"",\n`, gvk.Kind))
		}
	}
	if len(missingKinds) > 0 {
		l := missingKinds.UnsortedList()
		sort.Strings(l)
		t.Errorf("missing native kinds")
		t.Log(strings.Join(l, ""))
	}

	if len(nativeResources) != len(nativeKinds) {
		t.Errorf("nativeKinds map does not contain all known types or vice-versa: len(nativeResources) = %d != %d = len(nativeKinds)", len(nativeResources), len(nativeKinds))
	}
}
