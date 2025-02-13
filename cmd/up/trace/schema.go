// Copyright 2025 Upbound Inc.
// All rights reserved

package trace

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/upbound/up/cmd/up/query/resource"
)

var queryScheme = runtime.NewScheme()

func init() {
	kruntime.Must(resource.AddToScheme(queryScheme))
	kruntime.Must(metav1.AddMetaToScheme(queryScheme))

	metav1.AddToGroupVersion(queryScheme, schema.GroupVersion{Version: "v1"})
}
