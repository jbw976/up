// Copyright 2025 Upbound Inc.
// All rights reserved

package report

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/up/internal/usage/model"
	usagetesting "github.com/upbound/up/internal/usage/testing"
	usagetime "github.com/upbound/up/internal/usage/time"
)

func TestMaxResourceCountPerGVKPerMXP(t *testing.T) {
	type args struct {
		iter   *usagetesting.MockWindowIterator
		writer *usagetesting.MockWriter
	}
	type want struct {
		writer *usagetesting.MockWriter
		err    error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyIterator": {
			reason: "A report from an iterator with no windows has no events.",
			args: args{
				iter:   &usagetesting.MockWindowIterator{},
				writer: &usagetesting.MockWriter{},
			},
			want: want{
				writer: &usagetesting.MockWriter{},
			},
		},
		"EmptyReaders": {
			reason: "A report from an iterator with windows with no events has no events.",
			args: args{
				iter: &usagetesting.MockWindowIterator{Windows: []usagetesting.Window{
					{Reader: &usagetesting.MockReader{}},
					{Reader: &usagetesting.MockReader{}},
					{Reader: &usagetesting.MockReader{}},
				}},
				writer: &usagetesting.MockWriter{},
			},
			want: want{
				writer: &usagetesting.MockWriter{},
			},
		},
		"ReadError": {
			reason: "Read errors are returned.",
			args: args{
				iter: &usagetesting.MockWindowIterator{Windows: []usagetesting.Window{
					{Reader: &usagetesting.MockReader{Reads: []usagetesting.ReadResult{
						{Event: model.MXPGVKEvent{
							Name:  "kube_managedresource_uid",
							Value: float64(1),
							Tags: model.MXPGVKEventTags{
								Group:   "example.com",
								Version: "v1",
								Kind:    "Thing",
								MXPID:   "mxp1",
							},
						}},
						{Err: fmt.Errorf("boom")},
					}}},
				}},
				writer: &usagetesting.MockWriter{},
			},
			want: want{
				err:    fmt.Errorf("boom"),
				writer: &usagetesting.MockWriter{},
			},
		},
		"NonpositiveValuesIgnored": {
			reason: "Events with values <= 0 are not considered when aggregating.",
			args: args{
				iter: &usagetesting.MockWindowIterator{Windows: []usagetesting.Window{
					{Reader: &usagetesting.MockReader{Reads: []usagetesting.ReadResult{
						{Event: model.MXPGVKEvent{
							Name:  "kube_managedresource_uid",
							Value: float64(-1),
							Tags: model.MXPGVKEventTags{
								Group:   "example.com",
								Version: "v1",
								Kind:    "Thing",
								MXPID:   "mxp1",
							},
						}},
						{Event: model.MXPGVKEvent{
							Name:  "kube_managedresource_uid",
							Value: float64(-2),
							Tags: model.MXPGVKEventTags{
								Group:   "example.com",
								Version: "v1",
								Kind:    "Thing",
								MXPID:   "mxp2",
							},
						}},
					}}},
					{Reader: &usagetesting.MockReader{Reads: []usagetesting.ReadResult{
						{Event: model.MXPGVKEvent{
							Name:  "kube_managedresource_uid",
							Value: float64(0),
							Tags: model.MXPGVKEventTags{
								Group:   "example.com",
								Version: "v1",
								Kind:    "Thing",
								MXPID:   "mxp1",
							},
						}},
						{Event: model.MXPGVKEvent{
							Name:  "kube_managedresource_uid",
							Value: float64(-2),
							Tags: model.MXPGVKEventTags{
								Group:   "example.com",
								Version: "v1",
								Kind:    "Thing",
								MXPID:   "mxp1",
							},
						}},
					}}},
				}},
				writer: &usagetesting.MockWriter{},
			},
			want: want{
				writer: &usagetesting.MockWriter{},
			},
		},
		"PopulatedIterator": {
			reason: "A report from a populated iterator has the expected aggregated events.",
			args: args{
				iter: &usagetesting.MockWindowIterator{Windows: []usagetesting.Window{
					// This window's events have the same GVK and mxpID, so they
					// should be aggregated into one event.
					{
						Reader: &usagetesting.MockReader{Reads: []usagetesting.ReadResult{
							{Event: model.MXPGVKEvent{
								Name:  "kube_managedresource_uid",
								Value: float64(4),
								Tags: model.MXPGVKEventTags{
									Group:   "example.com",
									Version: "v1",
									Kind:    "Thing",
									MXPID:   "mxp1",
								},
							}},
							{Event: model.MXPGVKEvent{
								Name:  "kube_managedresource_uid",
								Value: float64(7),
								Tags: model.MXPGVKEventTags{
									Group:   "example.com",
									Version: "v1",
									Kind:    "Thing",
									MXPID:   "mxp1",
								},
							}},
							{Event: model.MXPGVKEvent{
								Name:  "kube_managedresource_uid",
								Value: float64(3),
								Tags: model.MXPGVKEventTags{
									Group:   "example.com",
									Version: "v1",
									Kind:    "Thing",
									MXPID:   "mxp1",
								},
							}},
						}},
						Window: usagetime.Range{
							Start: time.Date(2006, 0o5, 0o4, 0o3, 0, 0, 0, time.UTC),
							End:   time.Date(2006, 0o5, 0o4, 0o4, 0, 0, 0, time.UTC),
						},
					},
					// This window's events each have a different combination of
					// mxp and GVK, so they should be aggregated into separate
					// events.
					{
						Reader: &usagetesting.MockReader{Reads: []usagetesting.ReadResult{
							{Event: model.MXPGVKEvent{
								Name:  "kube_managedresource_uid",
								Value: float64(3),
								Tags: model.MXPGVKEventTags{
									Group:   "example.com",
									Version: "v1",
									Kind:    "Thing",
									MXPID:   "mxp1",
								},
							}},
							{Event: model.MXPGVKEvent{
								Name:  "kube_managedresource_uid",
								Value: float64(3),
								Tags: model.MXPGVKEventTags{
									Group:   "foo.example.com",
									Version: "v1",
									Kind:    "Thing",
									MXPID:   "mxp1",
								},
							}},
							{Event: model.MXPGVKEvent{
								Name:  "kube_managedresource_uid",
								Value: float64(3),
								Tags: model.MXPGVKEventTags{
									Group:   "example.com",
									Version: "v1alpha1",
									Kind:    "Thing",
									MXPID:   "mxp1",
								},
							}},
							{Event: model.MXPGVKEvent{
								Name:  "kube_managedresource_uid",
								Value: float64(3),
								Tags: model.MXPGVKEventTags{
									Group:   "example.com",
									Version: "v1",
									Kind:    "OtherThing",
									MXPID:   "mxp1",
								},
							}},
							{Event: model.MXPGVKEvent{
								Name:  "kube_managedresource_uid",
								Value: float64(3),
								Tags: model.MXPGVKEventTags{
									Group:   "example.com",
									Version: "v1",
									Kind:    "Thing",
									MXPID:   "mxp2",
								},
							}},
						}},
						Window: usagetime.Range{
							Start: time.Date(2006, 0o5, 0o4, 0o4, 0, 0, 0, time.UTC),
							End:   time.Date(2006, 0o5, 0o4, 0o5, 0, 0, 0, time.UTC),
						},
					},
					// This window's events have the same GVK and mxpID as
					// events in the first window, but they're in a different
					// window, so they should be aggregated into a different
					// event.
					{
						Reader: &usagetesting.MockReader{Reads: []usagetesting.ReadResult{
							{Event: model.MXPGVKEvent{
								Name:  "kube_managedresource_uid",
								Value: float64(1),
								Tags: model.MXPGVKEventTags{
									Group:   "example.com",
									Version: "v1",
									Kind:    "Thing",
									MXPID:   "mxp1",
								},
							}},
							{Event: model.MXPGVKEvent{
								Name:  "kube_managedresource_uid",
								Value: float64(50),
								Tags: model.MXPGVKEventTags{
									Group:   "example.com",
									Version: "v1",
									Kind:    "Thing",
									MXPID:   "mxp1",
								},
							}},
						}},
						Window: usagetime.Range{
							Start: time.Date(2006, 0o5, 0o4, 0o5, 0, 0, 0, time.UTC),
							End:   time.Date(2006, 0o5, 0o4, 0o6, 0, 0, 0, time.UTC),
						},
					},
				}},
				writer: &usagetesting.MockWriter{},
			},
			want: want{
				writer: &usagetesting.MockWriter{Events: []model.MXPGVKEvent{
					{
						Name:         "max_resource_count_per_gvk_per_mxp",
						Value:        float64(7),
						Timestamp:    time.Date(2006, 0o5, 0o4, 0o3, 0, 0, 0, time.UTC),
						TimestampEnd: time.Date(2006, 0o5, 0o4, 0o4, 0, 0, 0, time.UTC),
						Tags: model.MXPGVKEventTags{
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
							MXPID:   "mxp1",
						},
					},
					{
						Name:         "max_resource_count_per_gvk_per_mxp",
						Value:        float64(3),
						Timestamp:    time.Date(2006, 0o5, 0o4, 0o4, 0, 0, 0, time.UTC),
						TimestampEnd: time.Date(2006, 0o5, 0o4, 0o5, 0, 0, 0, time.UTC),
						Tags: model.MXPGVKEventTags{
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
							MXPID:   "mxp1",
						},
					},
					{
						Name:         "max_resource_count_per_gvk_per_mxp",
						Value:        float64(3),
						Timestamp:    time.Date(2006, 0o5, 0o4, 0o4, 0, 0, 0, time.UTC),
						TimestampEnd: time.Date(2006, 0o5, 0o4, 0o5, 0, 0, 0, time.UTC),
						Tags: model.MXPGVKEventTags{
							Group:   "foo.example.com",
							Version: "v1",
							Kind:    "Thing",
							MXPID:   "mxp1",
						},
					},
					{
						Name:         "max_resource_count_per_gvk_per_mxp",
						Value:        float64(3),
						Timestamp:    time.Date(2006, 0o5, 0o4, 0o4, 0, 0, 0, time.UTC),
						TimestampEnd: time.Date(2006, 0o5, 0o4, 0o5, 0, 0, 0, time.UTC),
						Tags: model.MXPGVKEventTags{
							Group:   "example.com",
							Version: "v1alpha1",
							Kind:    "Thing",
							MXPID:   "mxp1",
						},
					},
					{
						Name:         "max_resource_count_per_gvk_per_mxp",
						Value:        float64(3),
						Timestamp:    time.Date(2006, 0o5, 0o4, 0o4, 0, 0, 0, time.UTC),
						TimestampEnd: time.Date(2006, 0o5, 0o4, 0o5, 0, 0, 0, time.UTC),
						Tags: model.MXPGVKEventTags{
							Group:   "example.com",
							Version: "v1",
							Kind:    "OtherThing",
							MXPID:   "mxp1",
						},
					},
					{
						Name:         "max_resource_count_per_gvk_per_mxp",
						Value:        float64(3),
						Timestamp:    time.Date(2006, 0o5, 0o4, 0o4, 0, 0, 0, time.UTC),
						TimestampEnd: time.Date(2006, 0o5, 0o4, 0o5, 0, 0, 0, time.UTC),
						Tags: model.MXPGVKEventTags{
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
							MXPID:   "mxp2",
						},
					},
					{
						Name:         "max_resource_count_per_gvk_per_mxp",
						Value:        float64(50),
						Timestamp:    time.Date(2006, 0o5, 0o4, 0o5, 0, 0, 0, time.UTC),
						TimestampEnd: time.Date(2006, 0o5, 0o4, 0o6, 0, 0, 0, time.UTC),
						Tags: model.MXPGVKEventTags{
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
							MXPID:   "mxp1",
						},
					},
				}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := MaxResourceCountPerGVKPerMXP(context.Background(), tc.args.iter, tc.args.writer)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nMaxResourceCountPerGVKPerMXP: -want err, +got err:\n%s", tc.reason, diff)
			}
			got := tc.args.writer

			// Sort for stability.
			usagetesting.SortEvents(got.Events)
			usagetesting.SortEvents(tc.want.writer.Events)

			if diff := cmp.Diff(tc.want.writer, got); diff != "" {
				t.Errorf("\n%s\nMaxResourceCountPerGVKPerMXP: -want writer, +got writer:\n%s", tc.reason, diff)
			}
		})
	}
}
