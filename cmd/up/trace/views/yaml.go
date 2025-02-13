// Copyright 2025 Upbound Inc.
// All rights reserved

package views

import (
	"fmt"

	"github.com/rivo/tview"
	"k8s.io/apimachinery/pkg/types"

	"github.com/upbound/up/cmd/up/trace/model"
)

type YAML struct {
	*tview.TextView
}

func NewYAML(o *model.Object, s string) *YAML {
	y := &YAML{
		TextView: tview.NewTextView().
			SetDynamicColors(true).
			SetText(s),
	}
	y.TextView.SetBorder(true).
		SetTitle(fmt.Sprintf(" [::b]%s[::-] %s [darkgray]in ControlPlane[-] %s/%s ", o.Kind, types.NamespacedName{Namespace: o.Namespace, Name: o.Name}, o.ControlPlane.Namespace, o.ControlPlane.Name))

	return y
}
