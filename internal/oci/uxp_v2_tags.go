// Copyright 2025 Upbound Inc.
// All rights reserved

package oci

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

// uxpV2ChartValues reflects image tags from the UXP v2 Helm chart (crossplane) values.yaml.
type uxpV2ChartValues struct {
	Image struct {
		Tag string `json:"tag"`
	} `json:"image"`
	Upbound struct {
		Manager struct {
			Image struct {
				Tag string `json:"tag"`
			} `json:"image"`
		} `json:"manager"`
	} `json:"upbound"`
}

// GetUxpV2RuntimeTags pulls the UXP v2 Helm chart and returns crossplane and controller-manager
// image tags for mirroring. Tags follow the same leading-v convention as [normalizeMirrorTag].
// Crossplane uses image.tag then Chart.yaml appVersion. Controller-manager uses an explicit
// upbound.manager.image.tag when set; otherwise appVersion when it matches Crossplane's semver
// major.minor.patch (different chart vs runtime up.* suffixes), else the Crossplane runtime tag
// when the chart appVersion core version differs (e.g. 2.0.1 chart shipping 2.1.0 runtime).
//
// Note: This performs a Helm pull of the chart; the mirror command may pull the same chart again
// when mirroring the OCI artifact with crane.
func GetUxpV2RuntimeTags(chart, version, username, password string) (crossplaneTag, controllerManagerTag string, err error) {
	return getUxpV2RuntimeTagsWithDeps(chart, version, username, password, loader.Load, defaultPullFunc)
}

func getUxpV2RuntimeTagsWithDeps(chart, version, username, password string, loaderFunc LoaderFunc, pullFunc PullFunc) (string, string, error) {
	loadedChart, err := loadChart(chart, version, loaderFunc, pullFunc, username, password)
	if err != nil {
		return "", "", err
	}
	return parseUxpV2RuntimeTags(chart, version, loadedChart)
}

func parseUxpV2RuntimeTags(chart, version string, loadedChart *chart.Chart) (string, string, error) {
	appVersion := ""
	if loadedChart.Metadata != nil {
		appVersion = loadedChart.Metadata.AppVersion
	}

	valuesJSON, err := json.Marshal(loadedChart.Values)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal chart values: %w", err)
	}

	var v uxpV2ChartValues
	if err := json.Unmarshal(valuesJSON, &v); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal chart values: %w", err)
	}

	cx := resolveUxpImageTag(v.Image.Tag, appVersion)
	cm := resolveUxpControllerManagerTag(v.Upbound.Manager.Image.Tag, cx, appVersion)
	if cx == "" {
		return "", "", fmt.Errorf("failed to resolve crossplane image tag for chart %s:%s (empty image.tag and appVersion)", chart, version)
	}
	if cm == "" {
		return "", "", fmt.Errorf("failed to resolve controller-manager image tag for chart %s:%s (empty upbound.manager.image.tag and appVersion)", chart, version)
	}
	return cx, cm, nil
}

// resolveUxpControllerManagerTag mirrors Helm defaults: explicit upbound.manager.image.tag, else when
// the Crossplane image shares the same semver major.minor.patch as Chart appVersion use appVersion
// (e.g. chart 2.2.0-up.3 with Crossplane v2.2.0-up.1 → controller-manager v2.2.0-up.3). When the core
// version differs, published controller-manager artifacts often track the Crossplane runtime tag
// (e.g. chart 2.0.1-up.2 with Crossplane v2.1.0-up.1 → controller-manager v2.1.0-up.1).
func resolveUxpControllerManagerTag(managerValuesTag, crossplaneTag, appVersion string) string {
	if t := strings.TrimSpace(managerValuesTag); t != "" {
		return normalizeMirrorTag(t)
	}
	app := strings.TrimSpace(appVersion)
	if app == "" {
		return ""
	}
	if sameMajorMinorPatch(strings.TrimPrefix(strings.TrimSpace(crossplaneTag), "v"), strings.TrimPrefix(app, "v")) {
		return normalizeMirrorTag(app)
	}
	return crossplaneTag
}

func sameMajorMinorPatch(a, b string) bool {
	va, errA := semver.NewVersion(a)
	vb, errB := semver.NewVersion(b)
	if errA != nil || errB != nil {
		return false
	}
	return va.Major() == vb.Major() && va.Minor() == vb.Minor() && va.Patch() == vb.Patch()
}

func resolveUxpImageTag(valuesTag, appVersion string) string {
	if t := strings.TrimSpace(valuesTag); t != "" {
		return normalizeMirrorTag(t)
	}
	if t := strings.TrimSpace(appVersion); t != "" {
		return normalizeMirrorTag(t)
	}
	return ""
}

// normalizeMirrorTag ensures a leading v for mirror tags when absent, matching cmd/up/space mirror
// behavior for UXP v1 images.
func normalizeMirrorTag(tag string) string {
	if tag == "" {
		return ""
	}
	if strings.HasPrefix(tag, "v") {
		return tag
	}
	return "v" + tag
}
