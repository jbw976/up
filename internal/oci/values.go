// Copyright 2025 Upbound Inc.
// All rights reserved

// Package oci contains functions for handling remote oci artifacts
package oci

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
)

// PathNavigator is an interface for navigating and extracting paths in charts.
type PathNavigator interface {
	Extractor() ([]string, error)
}

// LoaderFunc is a function type for loading charts.
type LoaderFunc func(name string) (*chart.Chart, error)

// PullFunc is a function type for pulling charts.
type PullFunc func(src, version, username, password string) (string, error)

// GetValuesFromChart fetches the supported versions from a Helm chart specified by the chart and version parameters.
func GetValuesFromChart[T PathNavigator](chart, version string, pathNavigator T, username, password string) ([]string, error) {
	return getValuesFromChartWithLoaderAndPull(chart, version, pathNavigator, loader.Load, defaultPullFunc, username, password)
}

// getValuesFromChartWithLoaderAndPull is a helper function that fetches the supported versions
// from a Helm chart specified by the chart and version parameters. It allows custom loader and
// pull functions to be provided for more flexible testing and use cases.
func getValuesFromChartWithLoaderAndPull[T PathNavigator](chart, version string, pathNavigator T, loaderFunc LoaderFunc, pullFunc PullFunc, username, password string) ([]string, error) {
	src := fmt.Sprintf("oci://%s", chart)
	settings := cli.New()

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(
		settings.RESTClientGetter(),
		settings.Namespace(),
		"configmap",
		log.Printf,
	); err != nil {
		return nil, fmt.Errorf("failed to initialize Helm action configuration: %w", err)
	}

	tempDir, err := pullFunc(src, version, username, password)
	if err != nil {
		return nil, fmt.Errorf("failed to pull chart: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			log.Printf("failed to remove temporary directory: %v", err)
		}
	}()

	chartPath := filepath.Join(tempDir, GetArtifactName(chart))
	loadedChart, err := loaderFunc(chartPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart: %w", err)
	}

	valuesJSON, err := json.Marshal(loadedChart.Values)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chart values: %w", err)
	}

	if err := json.Unmarshal(valuesJSON, &pathNavigator); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chart values: %w", err)
	}

	return pathNavigator.Extractor()
}

// defaultPullFunc is the default implementation of the PullFunc type for pulling charts.
func defaultPullFunc(repourl, version, username, password string) (string, error) {
	settings := cli.New()
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(
		settings.RESTClientGetter(),
		settings.Namespace(),
		"configmap",
		log.Printf,
	); err != nil {
		return "", fmt.Errorf("failed to initialize Helm action configuration: %w", err)
	}

	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(false),
		registry.ClientOptEnableCache(false),
	)
	if err != nil {
		return "nil", err
	}
	actionConfig.RegistryClient = registryClient

	if username != "" && password != "" {
		auth := registry.LoginOptBasicAuth(username, password)
		if err := actionConfig.RegistryClient.Login("xpkg.upbound.io", auth); err != nil {
			return "", err
		}
	}

	f, err := os.MkdirTemp("", "untar")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	co := action.ChartPathOptions{
		PassCredentialsAll: true,
		Version:            version,
	}

	opts := []action.PullOpt{
		action.WithConfig(actionConfig),
	}

	pull := action.NewPullWithOpts(opts...)
	pull.ChartPathOptions = co
	pull.Untar = true
	pull.Settings = settings
	pull.DestDir = f

	_, err = pull.Run(repourl)
	if err != nil {
		if removeErr := os.RemoveAll(f); removeErr != nil {
			log.Printf("failed to remove temporary directory: %v", removeErr)
		}
		return "", fmt.Errorf("failed to pull chart: %w", err)
	}

	return f, nil
}
