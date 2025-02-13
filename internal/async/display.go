// Copyright 2025 Upbound Inc.
// All rights reserved

// Package async contains utilities for running functions asynchronously and
// displaying status updates from them.
package async

import (
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/upterm"
)

// WrapperFunc is a function that can be wrapped around functions that emit
// events for asynchronous display.
type WrapperFunc func(fn func(ch EventChannel) error) error

// IgnoreEvents is a wrapper function that runs the given function and ignores
// any events it produces.
func IgnoreEvents(fn func(ch EventChannel) error) error {
	return fn(nil)
}

// WrapWithSuccessSpinners runs a given function in a separate goroutine,
// consuming events from its event channel and using them to display a set of
// spinners on the terminal. One spinner will be generated for each unique event
// text received. A checkmark will be displayed on success.
func WrapWithSuccessSpinners(fn func(ch EventChannel) error) error {
	var (
		updateChan = make(EventChannel, 10)
		doneChan   = make(chan error, 1)
	)

	go func() {
		err := fn(updateChan)
		close(updateChan)
		doneChan <- err
	}()

	multi := &pterm.DefaultMultiPrinter
	multi, _ = multi.Start()
	spinners := make(map[string]*pterm.SpinnerPrinter)
	for update := range updateChan {
		spinner, ok := spinners[update.Text]
		if !ok {
			spinner, _ = upterm.NewCheckmarkSuccessSpinner(multi.NewWriter()).Start(update.Text)
			spinners[update.Text] = spinner
		}
		switch update.Status {
		case EventStatusStarted:
			// Spinner should already be running.
		case EventStatusSuccess:
			spinner.Success(update.Text)
		case EventStatusFailure:
			spinner.Fail(update.Text)
		}
	}
	err := <-doneChan
	_, _ = multi.Stop()

	return err
}
