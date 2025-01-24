// Copyright 2025 Upbound Inc.
// All rights reserved

package azure

import (
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	clock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"

	"github.com/upbound/up/internal/usage/event"
	"github.com/upbound/up/internal/usage/event/reader"
	usagetime "github.com/upbound/up/internal/usage/time"
)

var _ event.WindowIterator = &WindowIterator{}

// WindowIterator iterates through readers for windows of usage events from an
// Azure blob storage container. Must be initialized with NewWindowIterator().
type WindowIterator struct {
	Iter   *ListBlobsOptionsIterator
	Client *container.Client
}

// NewWindowIterator returns an initialized *WindowIterator.
func NewWindowIterator(cli *container.Client, account string, tr usagetime.Range, window time.Duration) (*WindowIterator, error) {
	iter, err := NewListBlobsOptionsIterator(account, tr, window)
	if err != nil {
		return nil, err
	}
	return &WindowIterator{
		Iter:   iter,
		Client: cli,
	}, nil
}

func (i *WindowIterator) More() bool {
	return i.Iter.More()
}

func (i *WindowIterator) Next() (event.Reader, usagetime.Range, error) {
	lo, window, err := i.Iter.Next()
	if err != nil {
		return nil, usagetime.Range{}, err
	}

	readers := make([]event.Reader, len(lo))
	for j, opts := range lo {
		readers[j] = &PagerEventReader{Client: i.Client, Pager: i.Client.NewListBlobsFlatPager(&opts)}
	}

	return &reader.MultiReader{Readers: readers}, window, nil
}

// ListBlobsOptionsIterator iterates through slices of ListBlobsFlatOptions for
// each window of time in a time range. Must be initialized with
// NewListBlobOptions().
type ListBlobsOptionsIterator struct {
	Account string
	Iter    *usagetime.WindowIterator
}

// NewListBlobsOptionsIterator returns an initialized *ListBlobOptionsIterator.
func NewListBlobsOptionsIterator(account string, tr usagetime.Range, window time.Duration) (*ListBlobsOptionsIterator, error) {
	iter, err := usagetime.NewWindowIterator(tr, window)
	if err != nil {
		return nil, err
	}
	return &ListBlobsOptionsIterator{
		Account: account,
		Iter:    iter,
	}, nil
}

// More() returns true if Next() has more to return.
func (i *ListBlobsOptionsIterator) More() bool {
	return i.Iter.More()
}

// Next() returns a []container.ListBlobsFlatOptions covering the next window of
// time, as well as a time range marking the window.
func (i *ListBlobsOptionsIterator) Next() ([]container.ListBlobsFlatOptions, usagetime.Range, error) {
	window, err := i.Iter.Next()
	if err != nil {
		return nil, usagetime.Range{}, err
	}

	// Create a list options struct for each hour prefix in the window.
	lbo := []container.ListBlobsFlatOptions{}
	c := clock.SimpleIntervalClock{Time: window.Start, Duration: time.Hour}
	now := window.Start
	for {
		if now.Equal(window.End) || now.After(window.End) {
			break
		}
		lbo = append(lbo, container.ListBlobsFlatOptions{
			Prefix: ptr.To(fmt.Sprintf(
				"account=%s/date=%s/hour=%02d/",
				i.Account,
				usagetime.FormatDateUTC(now),
				now.Hour(),
			)),
		})
		now = c.Now()
	}

	return lbo, window, nil
}
