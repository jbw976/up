// Copyright 2025 Upbound Inc.
// All rights reserved

// Package space contains functions for handling spaces
package space

import (
	"encoding/json"
	"reflect"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	_ "embed"
)

// Embed the YAML file.
//
//go:embed mirrorconfig.yaml
var configFile []byte

type uxpVersionsPath struct {
	Controller struct {
		Crossplane struct {
			SupportedVersions []string `json:"supportedVersions"`
		} `json:"crossplane"`
	} `json:"controller"`
}

type kubeVersionPath struct {
	ControlPlanes struct {
		K8sVersion stringOrArray `json:"k8sVersion"`
	} `json:"controlPlanes"`
}

type xgqlVersionPath struct {
	ControlPlanes struct {
		Uxp struct {
			Xgql struct {
				Version stringOrArray `json:"version"`
			} `json:"xgql"`
		} `json:"uxp"`
	} `json:"controlPlanes"`
}

type imageTag struct {
	Image struct {
		Tag stringOrArray `json:"tag"`
	} `json:"image"`
}

type registerImageTag struct {
	Registration struct {
		Image struct {
			Tag stringOrArray `json:"tag"`
		} `json:"image"`
	} `json:"registration"`
}

func (j *uxpVersionsPath) Extractor() ([]string, error) {
	if len(j.Controller.Crossplane.SupportedVersions) == 0 {
		return nil, errors.New("no supported versions found in UXPVersionsPath")
	}
	return j.Controller.Crossplane.SupportedVersions, nil
}

func (k *kubeVersionPath) Extractor() ([]string, error) {
	if len(k.ControlPlanes.K8sVersion) == 0 {
		return nil, errors.New("no supported versions found in KubeVersionPath")
	}
	return k.ControlPlanes.K8sVersion, nil
}

func (k *xgqlVersionPath) Extractor() ([]string, error) {
	if len(k.ControlPlanes.Uxp.Xgql.Version) == 0 {
		return nil, errors.New("no supported versions found in XgqlVersionPath")
	}
	return k.ControlPlanes.Uxp.Xgql.Version, nil
}

func (k *imageTag) Extractor() ([]string, error) {
	if len(k.Image.Tag) == 0 {
		return nil, errors.New("no supported versions found in ImageTag")
	}
	return k.Image.Tag, nil
}

func (k *registerImageTag) Extractor() ([]string, error) {
	if len(k.Registration.Image.Tag) == 0 {
		return nil, errors.New("no supported versions found in RegisterImageTag")
	}
	return k.Registration.Image.Tag, nil
}

// init function to return byte slice and oci.PathNavigator.
func initConfig() ([]byte, map[string]reflect.Type) {
	return configFile, map[string]reflect.Type{
		"uxpVersionsPath":  reflect.TypeOf(uxpVersionsPath{}),
		"kubeVersionPath":  reflect.TypeOf(kubeVersionPath{}),
		"xgqlVersionPath":  reflect.TypeOf(xgqlVersionPath{}),
		"imageTag":         reflect.TypeOf(imageTag{}),
		"registerImageTag": reflect.TypeOf(registerImageTag{}),
	}
}

type stringOrArray []string

func (s *stringOrArray) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*s = []string{single}
		return nil
	}

	var array []string
	if err := json.Unmarshal(data, &array); err == nil {
		*s = array
		return nil
	}

	return errors.New("data is neither a string nor an array of strings")
}
