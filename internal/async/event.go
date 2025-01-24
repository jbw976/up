// Copyright 2025 Upbound Inc.
// All rights reserved

package async

// Event represents an event that happened during an asynchronous operation. It
// is used to pass information back to callers that are interested in the
// operation's progress.
type Event struct {
	// Text is a description of the event. Events with the same text represent
	// updates to the status of a single sub-operation.
	Text string
	// Status is the updated status of the sub-operation.
	Status EventStatus
}

// EventStatus represents the status of an async process.
type EventStatus string

const (
	// EventStatusStarted indicates that an operation has started.
	EventStatusStarted EventStatus = "started"
	// EventStatusSuccess indicates that an operation has completed
	// successfully.
	EventStatusSuccess EventStatus = "success"
	// EventStatusFailure indicates that an operation has failed.
	EventStatusFailure EventStatus = "failure"
)

// EventChannel is a channel for sending events. We define our own type for it
// so we can attach useful functions to it.
type EventChannel chan Event

// SendEvent sends an event to an event channel. It is a no-op if the channel is
// nil. This allows event producers to produce events unconditionally, with
// callers providing an optionally nil channel.
func (ch EventChannel) SendEvent(text string, status EventStatus) {
	if ch == nil {
		return
	}
	ch <- Event{
		Text:   text,
		Status: status,
	}
}
