// Copyright 2025 Upbound Inc.
// All rights reserved

package xpkg

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// JSONStreamFile is the name of the file in local Crossplane package
	// that contains the JSON stream representation of the Crossplane package.
	JSONStreamFile string = "package.ndjson"

	// MetaFile is the name of a Crossplane package metadata file.
	MetaFile string = "crossplane.yaml"

	// StreamFile is the name of the file in a Crossplane package image that
	// contains its YAML stream.
	StreamFile string = "package.yaml"

	// StreamFileMode determines the permissions on the stream file.
	StreamFileMode os.FileMode = 0o644

	// XpkgExtension is the extension for compiled Crossplane packages.
	XpkgExtension string = ".xpkg"

	// XpkgMatchPattern is the match pattern for identifying compiled Crossplane
	// packages.
	XpkgMatchPattern string = "*" + XpkgExtension

	// XpkgExamplesFile is the name of the file in a Crossplane package image
	// that contains the examples YAML stream.
	XpkgExamplesFile string = ".up/examples.yaml"

	// XpkgHelmChartFile is the name of the file in an Upbound Controller
	// package image that contains the helm chart.
	XpkgHelmChartFile = "helm/chart.tgz"
	
	// AnnotationKey is the key value for xpkg annotations.
	AnnotationKey string = "io.crossplane.xpkg"
	// PackageAnnotation is the annotation value used for the package.yaml
	// layer.
	PackageAnnotation string = "base"
	// ExamplesAnnotation is the annotation value used for the examples.yaml
	// layer.
	ExamplesAnnotation string = "upbound"
	// SchemaKclAnnotation is the annotation value used for the kcl schema
	// layer.
	SchemaKclAnnotation string = "schema.kcl"
	// SchemaKclModFile is the name of the kcl mod file in a Crossplane package image
	// that contains the kcl mod.
	SchemaKclModFile string = "models/kcl.mod"
	// SchemaPythonAnnotation is the annotation value used for the python schema
	// layer.
	SchemaPythonAnnotation string = "schema.python"
	// SchemaGoAnnotation is the annotation value used for the go schema layer.
	SchemaGoAnnotation string = "schema.go"
	// HelmChartAnnotation is the annotation value used for the helm chart layer.
	HelmChartAnnotation = "helm"
)

func truncate(str string, num int) string {
	t := str
	if len(str) > num {
		t = str[0:num]
	}
	return t
}

// FriendlyID builds a valid DNS label string made up of the name of a package
// and its image digest.
func FriendlyID(name, hash string) string {
	return ToDNSLabel(strings.Join([]string{truncate(name, 50), truncate(hash, 12)}, "-"))
}

// ToDNSLabel converts the string to a valid DNS label.
func ToDNSLabel(s string) string {
	var cut strings.Builder
	for i := range s {
		b := s[i]
		if ('a' <= b && b <= 'z') || ('0' <= b && b <= '9') {
			cut.WriteByte(b)
		}
		if (b == '.' || b == '/' || b == ':' || b == '-') && (i != 0 && i != 62 && i != len(s)-1) {
			cut.WriteByte('-')
		}
		if i == 62 {
			break
		}
	}
	return strings.Trim(cut.String(), "-")
}

// BuildPath builds a path for a compiled Crossplane package. If file name has
// extension it will be replaced.
func BuildPath(path, name string) string {
	full := filepath.Join(path, name)
	return ReplaceExt(full, XpkgExtension)
}

// ReplaceExt replaces the file extension of the given path.
func ReplaceExt(path, ext string) string {
	old := filepath.Ext(path)
	return path[0:len(path)-len(old)] + ext
}
