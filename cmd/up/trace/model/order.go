// Copyright 2025 Upbound Inc.
// All rights reserved

package model

import (
	"github.com/rivo/tview"
)

type ObjectsOrder []*tview.TreeNode

func (o ObjectsOrder) Len() int { return len(o) }
func (o ObjectsOrder) Less(i, j int) bool {
	oi := o[i].GetReference().(*Object)
	oj := o[i].GetReference().(*Object)

	if a, b := oi.Group, oj.Group; a != b {
		return a < b
	}
	if a, b := oi.Kind, oj.Kind; a != b {
		return a < b
	}
	if a, b := oi.ControlPlane.Namespace, oj.ControlPlane.Namespace; a != b {
		return a < b
	}
	if a, b := oi.ControlPlane.Name, oj.ControlPlane.Name; a != b {
		return a < b
	}
	if a, b := oi.Namespace, oj.Namespace; a != b {
		return a < b
	}
	if a, b := oi.Name, oj.Name; a != b {
		return a < b
	}

	return false
}
func (o ObjectsOrder) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

type EventsOrder []Event

func (e EventsOrder) Len() int { return len(e) }
func (e EventsOrder) Less(i, j int) bool {
	return e[i].LastTimestamp.Before(&e[j].LastTimestamp)
}
func (e EventsOrder) Swap(i, j int) { e[i], e[j] = e[j], e[i] }
