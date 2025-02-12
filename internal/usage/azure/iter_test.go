// Copyright 2025 Upbound Inc.
// All rights reserved

package azure

import (
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	usagetime "github.com/upbound/up/internal/usage/time"
)

func TestListBlobsOptionsIterator(t *testing.T) {
	type args struct {
		account string
		tr      usagetime.Range
		window  time.Duration
	}
	type iteration struct {
		ListBlobsOptions []container.ListBlobsFlatOptions
		Window           usagetime.Range
		Err              error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   []iteration
	}{
		"3HourRange1HourWindow": {
			reason: "3h range divided into 1h windows.",
			args: args{
				account: "test-account",
				tr: usagetime.Range{
					Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
				},
				window: time.Hour,
			},
			want: []iteration{
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=03/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 4, 0, 0, 0, time.UTC),
					},
				},
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=04/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 4, 4, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 5, 0, 0, 0, time.UTC),
					},
				},
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=05/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 4, 5, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
					},
				},
			},
		},
		"3HourRange2HourWindow": {
			reason: "3h range divided into 2h windows.",
			args: args{
				account: "test-account",
				tr: usagetime.Range{
					Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
				},
				window: 2 * time.Hour,
			},
			want: []iteration{
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=03/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=04/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 5, 0, 0, 0, time.UTC),
					},
				},
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=05/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 4, 5, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
					},
				},
			},
		},
		"3HourRange4HourWindow": {
			reason: "3h range divided into 4h windows.",
			args: args{
				account: "test-account",
				tr: usagetime.Range{
					Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
				},
				window: 4 * time.Hour,
			},
			want: []iteration{
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=03/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=04/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=05/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
					},
				},
			},
		},
		"3DayRange1DayWindow": {
			reason: "3-day range divided into 1-day windows.",
			args: args{
				account: "test-account",
				tr: usagetime.Range{
					Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 7, 3, 0, 0, 0, time.UTC),
				},
				window: 24 * time.Hour,
			},
			want: []iteration{
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=03/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=04/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=05/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=06/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=07/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=08/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=09/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=10/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=11/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=12/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=13/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=14/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=15/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=16/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=17/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=18/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=19/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=20/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=21/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=22/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-04/hour=23/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=00/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=01/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=02/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 5, 3, 0, 0, 0, time.UTC),
					},
				},
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=03/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=04/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=05/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=06/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=07/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=08/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=09/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=10/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=11/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=12/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=13/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=14/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=15/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=16/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=17/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=18/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=19/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=20/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=21/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=22/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-05/hour=23/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=00/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=01/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=02/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 5, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 6, 3, 0, 0, 0, time.UTC),
					},
				},
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=03/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=04/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=05/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=06/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=07/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=08/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=09/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=10/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=11/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=12/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=13/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=14/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=15/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=16/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=17/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=18/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=19/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=20/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=21/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=22/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-06/hour=23/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-07/hour=00/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-07/hour=01/")},
						{Prefix: ptr.To("account=test-account/date=2006-05-07/hour=02/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 6, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 7, 3, 0, 0, 0, time.UTC),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			iter, err := NewListBlobsOptionsIterator(tc.args.account, tc.args.tr, tc.args.window)
			if err != nil {
				t.Fatalf("NewListBlobsOptionsIterator() error: %s", err)
			}

			got := []iteration{}
			for iter.More() {
				lbo, window, err := iter.Next()
				got = append(got, iteration{ListBlobsOptions: lbo, Window: window, Err: err})
			}

			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nListBlobsOptionsIterator output: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
