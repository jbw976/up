// Copyright 2025 Upbound Inc.
// All rights reserved

// Package trace contains the `up alpha trace` command.
package trace

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/upbound/up-sdk-go/apis/common"
	queryv1alpha2 "github.com/upbound/up-sdk-go/apis/query/v1alpha2"
	"github.com/upbound/up/cmd/up/query"
	"github.com/upbound/up/cmd/up/query/resource"
	"github.com/upbound/up/internal/upbound"

	_ "embed"
)

// Cmd is the `up alpha trace` command.
type Cmd struct {
	upbound.RequiresContextAllowMissingProfile

	ControlPlane string `description:"Controlplane to query"                                      env:"UPBOUND_CONTROLPLANE" long:"controlplane" short:"c"`
	Group        string `description:"Group to query"                                             env:"UPBOUND_GROUP"        long:"group"        short:"g"`
	Namespace    string `description:"Namespace of objects to query (defaults to all namespaces)" env:"UPBOUND_NAMESPACE"    long:"namespace"    short:"n"`
	AllGroups    bool   `help:"Query in all groups."                                              name:"all-groups"          short:"A"`

	// positional arguments
	Resources []string `arg:"" help:"Type(s) (resource, singular or plural, category, short-name) and names: TYPE[.GROUP][,TYPE[.GROUP]...] [NAME ...] | TYPE[.GROUP]/NAME .... If no resource is specified, all resources are queried, but --all-resources must be specified."`
}

//go:embed help/trace.md
var traceHelp string

// Help returns help for the trace command.
func (c *Cmd) Help() string {
	return traceHelp
}

// Run is the implementation of the command.
func (c *Cmd) Run(ctx context.Context, upCtx *upbound.Context) error { //nolint:gocognit // TODO: split up
	// create client
	kubeconfig, err := upCtx.GetKubeconfig()
	if err != nil {
		return err
	}

	_, ctp, isSpace := upCtx.GetCurrentSpaceContextScope()

	if c.Group == "" && !c.AllGroups {
		if isSpace && ctp.Namespace != "" {
			c.Group = ctp.Namespace
		}
	}
	kc, err := client.New(kubeconfig, client.Options{Scheme: queryScheme})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// parse positional arguments
	tgns, errs := query.ParseTypesAndNames(c.Resources...)
	if len(errs) > 0 {
		return kerrors.NewAggregate(errs)
	}
	gkNames, categoryNames := query.SplitGroupKindAndCategories(tgns)

	// create query template depending on the scope
	var queryObject resource.QueryObject
	switch {
	case c.Group != "" && c.ControlPlane != "":
		queryObject = &resource.Query{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: c.Group,
				Name:      c.ControlPlane,
			},
		}
	case c.Group != "":
		queryObject = &resource.GroupQuery{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: c.Group,
			},
		}
	default:
		queryObject = &resource.SpaceQuery{}
	}

	poll := func(gkns query.GroupKindNames, cns query.CategoryNames) ([]queryv1alpha2.QueryResponseObject, error) {
		var querySpecs []*queryv1alpha2.QuerySpec
		for gk, names := range gkns {
			if len(names) == 0 {
				query := createQuerySpec(types.NamespacedName{Namespace: c.Namespace}, gk, nil)
				querySpecs = append(querySpecs, query)
				continue
			}
			for _, name := range names {
				query := createQuerySpec(types.NamespacedName{Namespace: c.Namespace, Name: name}, gk, nil)
				querySpecs = append(querySpecs, query)
			}
		}
		for cat, names := range cns {
			catList := []string{cat}
			if cat == query.AllCategory {
				catList = nil
			}
			if len(names) == 0 {
				query := createQuerySpec(types.NamespacedName{Namespace: c.Namespace}, metav1.GroupKind{}, catList)
				querySpecs = append(querySpecs, query)
				continue
			}
			for _, name := range names {
				query := createQuerySpec(types.NamespacedName{Namespace: c.Namespace, Name: name}, metav1.GroupKind{}, catList)
				querySpecs = append(querySpecs, query)
			}
		}

		var objs []queryv1alpha2.QueryResponseObject
		for _, spec := range querySpecs {
			var cursor string
			var page int
			for {
				spec := spec.DeepCopy()
				spec.QueryTopLevelResources.QueryResources.Page.Cursor = cursor
				query := queryObject.DeepCopyQueryObject().SetSpec(spec)

				if err := kc.Create(ctx, query); err != nil {
					return nil, fmt.Errorf("%T request failed: %w", query, err)
				}
				resp := query.GetResponse()
				objs = append(objs, resp.Objects...)

				// do paging
				cursor = resp.Cursor.Next
				page++
				if cursor == "" {
					break
				}
			}
		}

		return objs, nil
	}

	fetch := func(id string) (*unstructured.Unstructured, error) {
		query := queryObject.DeepCopyQueryObject().SetSpec(&queryv1alpha2.QuerySpec{
			QueryTopLevelResources: queryv1alpha2.QueryTopLevelResources{
				Filter: queryv1alpha2.QueryTopLevelFilter{
					Objects: []queryv1alpha2.QueryFilter{
						{
							ID: id,
						},
					},
				},
				QueryResources: queryv1alpha2.QueryResources{
					Objects: &queryv1alpha2.QueryObjects{
						ID:           true,
						ControlPlane: true,
						Object: &common.JSON{
							Object: true,
						},
					},
				},
			},
		})

		if err := kc.Create(ctx, query); err != nil {
			return nil, fmt.Errorf("failed to SpaceQuery request: %w", err)
		}

		if len(query.GetResponse().Objects) == 0 {
			return nil, fmt.Errorf("not found Object: %s", id)
		}

		return &unstructured.Unstructured{Object: query.GetResponse().Objects[0].Object.Object}, nil
	}

	upCtx.HideLogging()
	app := NewApp("upbound trace", c.Resources, gkNames, categoryNames, poll, fetch)
	return app.Run(ctx)
}

func createQuerySpec(obj types.NamespacedName, gk metav1.GroupKind, categories []string) *queryv1alpha2.QuerySpec {
	return &queryv1alpha2.QuerySpec{
		QueryTopLevelResources: queryv1alpha2.QueryTopLevelResources{
			Filter: queryv1alpha2.QueryTopLevelFilter{
				Objects: []queryv1alpha2.QueryFilter{
					{
						GroupKind: queryv1alpha2.QueryGroupKind{
							APIGroup: gk.Group,
							Kind:     gk.Kind,
						},
						Namespace:  obj.Namespace,
						Name:       obj.Name,
						Categories: categories,
					},
				},
			},
			QueryResources: queryv1alpha2.QueryResources{
				Limit:  500,
				Cursor: true,
				Objects: &queryv1alpha2.QueryObjects{
					ID:           true,
					ControlPlane: true,
					Object: &common.JSON{
						Object: map[string]interface{}{
							"kind":       true,
							"apiVersion": true,
							"metadata": map[string]interface{}{
								"creationTimestamp": true,
								"deletionTimestamp": true,
								"name":              true,
								"namespace":         true,
							},
							"status": map[string]interface{}{
								"conditions": true,
							},
						},
					},
					Relations: map[string]queryv1alpha2.QueryRelation{
						"events": {
							QueryNestedResources: queryv1alpha2.QueryNestedResources{
								QueryResources: queryv1alpha2.QueryResources{
									Objects: &queryv1alpha2.QueryObjects{
										ID:           true,
										ControlPlane: true,
										Object: &common.JSON{
											Object: map[string]interface{}{
												"lastTimestamp": true,
												"message":       true,
												"count":         true,
												"type":          true,
											},
										},
									},
								},
							},
						},
						"resources+": {
							QueryNestedResources: queryv1alpha2.QueryNestedResources{
								QueryResources: queryv1alpha2.QueryResources{
									Limit: 10000,
									Objects: &queryv1alpha2.QueryObjects{
										ID:           true,
										ControlPlane: true,
										Object: &common.JSON{
											Object: map[string]interface{}{
												"kind":       true,
												"apiVersion": true,
												"metadata": map[string]interface{}{
													"creationTimestamp": true,
													"deletionTimestamp": true,
													"name":              true,
													"namespace":         true,
												},
												"status": map[string]interface{}{
													"conditions": true,
												},
											},
										},
										Relations: map[string]queryv1alpha2.QueryRelation{
											"events": {
												QueryNestedResources: queryv1alpha2.QueryNestedResources{
													QueryResources: queryv1alpha2.QueryResources{
														Objects: &queryv1alpha2.QueryObjects{
															ID:           true,
															ControlPlane: true,
															Object: &common.JSON{
																Object: map[string]interface{}{
																	"lastTimestamp": true,
																	"message":       true,
																	"count":         true,
																	"type":          true,
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
