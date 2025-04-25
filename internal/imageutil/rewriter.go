// Copyright 2025 Upbound Inc.
// All rights reserved

// Package imageutil contains functions to work with all kind of images.
package imageutil

import (
	"strings"

	projectv1alpha1 "github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// RewriteImage applies imageConfig rules to rewrite the given image string.
func RewriteImage(image string, configs []projectv1alpha1.ImageConfig) string {
	var bestMatchPrefix, replacementPrefix string

	for _, config := range configs {
		for _, match := range config.MatchImages {
			if strings.HasPrefix(image, match.Prefix) {
				if len(match.Prefix) > len(bestMatchPrefix) {
					bestMatchPrefix = match.Prefix
					replacementPrefix = config.RewriteImage.Prefix
				}
			}
		}
	}

	return replacementPrefix + strings.TrimPrefix(image, bestMatchPrefix)
}
