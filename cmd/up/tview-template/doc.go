// Copyright 2025 Upbound Inc.
// All rights reserved

// Package template is meant to be a template for creation text ui commands. For
// a new command, copy this folder to cmd/up/<your-copy> and adapt it to your
// needs.
//
// This template intentionally follows a certain structure to make it easier to
// build maintainable text uis. Especially the use of a model and views rendering
// the model is highly encouraged.
//
// See https://github.com/rivo/tview for more information on tview.
//
// There is also the internal/tview package which means to serve as a collection
// of shared code between different tview commands. Please consider moving
// reusable components there.

package template
