// Copyright 2025 Upbound Inc.
// All rights reserved

// Package schemagenerator generates schemas for languages
package schemagenerator

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xpv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	xcrd "github.com/upbound/up/internal/crd"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/xpkg/schemarunner"
)

const (
	kclSchemaFolder         = "schemas"
	kclModelsFolder         = "models"
	kclAdoptModelsStructure = "sorted"
	kclImage                = "xpkg.upbound.io/upbound/kcl:v0.10.6"
)

// GenerateSchemaKcl generates KCL schema files from the XRDs and CRDs fromFS.
func GenerateSchemaKcl(ctx context.Context, fromFS afero.Fs, exclude []string, generator schemarunner.SchemaRunner) (afero.Fs, error) { //nolint:gocognit // generate kcl schemas
	crdFS := afero.NewMemMapFs()
	schemaFS := afero.NewMemMapFs()
	baseFolder := "workdir"

	if err := crdFS.MkdirAll(baseFolder, 0o755); err != nil {
		return nil, err
	}

	var crdPaths []string

	// Walk the virtual filesystem to find and process target files
	if err := afero.Walk(fromFS, "", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded files
		if !info.IsDir() {
			for _, excl := range exclude {
				if info.Name() == excl {
					return nil // Skip this file
				}
			}
		}

		// Skip excluded paths
		for _, excl := range exclude {
			if strings.HasPrefix(path, excl) {
				return filepath.SkipDir
			}
		}

		if info.IsDir() {
			return nil
		}
		// Ignore files without yaml extensions.
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		var u metav1.TypeMeta
		bs, err := afero.ReadFile(fromFS, path)
		if err != nil {
			return errors.Wrapf(err, "failed to read file %q", path)
		}
		err = yaml.Unmarshal(bs, &u)
		if err != nil {
			return errors.Wrapf(err, "failed to parse file %q", path)
		}

		switch u.GroupVersionKind().Kind {
		case xpv1.CompositeResourceDefinitionKind:
			// Process the XRD and get the paths
			xrPath, claimPath, err := xcrd.ProcessXRD(crdFS, bs, path, baseFolder)
			if err != nil {
				return err
			}

			// Append paths if they are returned
			if xrPath != "" {
				crdPaths = append(crdPaths, xrPath)
			}
			if claimPath != "" {
				crdPaths = append(crdPaths, claimPath)
			}

		case "CustomResourceDefinition":
			// Write CRD file
			if err := afero.WriteFile(crdFS, filepath.Join(baseFolder, path), bs, 0o644); err != nil {
				return err
			}
			crdPaths = append(crdPaths, filepath.Join(baseFolder, path))
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if len(crdPaths) == 0 {
		// Return nil if no files were generated
		return nil, nil
	}

	if err := generator.Generate(
		ctx,
		crdFS,
		baseFolder,
		"",
		kclImage,
		[]string{
			"sh", "-c",
			`find . -name "*.yaml" -exec kcl import -m crd -s {} \;`,
		},
	); err != nil {
		return nil, err
	}

	// we need to transform the folder structure to avoid the same resource kinds across multiple providers
	if err := transformStructureKcl(crdFS, kclModelsFolder, kclAdoptModelsStructure); err != nil {
		return nil, err
	}

	// Copy only the files from kclAdoptModelsStructure into the schemaFs
	if err := filesystem.CopyFilesBetweenFs(afero.NewBasePathFs(crdFS, kclAdoptModelsStructure), afero.NewBasePathFs(schemaFS, kclModelsFolder)); err != nil {
		return nil, err
	}

	return schemaFS, nil
}

// transformStructureKcl modifies the existing fs by moving files from sourceDir to targetDir
// in the reversed and segmented structure with the version appended at the end,
// and it copies the k8s directory and specific files (kcl.mod and kcl.mod.lock) to the targetDir.
func transformStructureKcl(fs afero.Fs, sourceDir, targetDir string) error { //nolint:gocognit // tansform kcl schemas
	// Copy kcl.mod and kcl.mod.lock files if they exist
	if err := filesystem.CopyFileIfExists(fs, filepath.Join(sourceDir, "kcl.mod"), filepath.Join(targetDir, "kcl.mod")); err != nil {
		return errors.Wrap(err, "failed to copy kcl.mod")
	}

	if err := filesystem.CopyFileIfExists(fs, filepath.Join(sourceDir, "kcl.mod.lock"), filepath.Join(targetDir, "kcl.mod.lock")); err != nil {
		return errors.Wrap(err, "failed to copy kcl.mod.lock")
	}

	// Modify files in the sourceDir before copying
	objectMetaPath := filepath.Join(sourceDir, "k8s", "apimachinery", "pkg", "apis", "meta", "v1", "object_meta.k")
	managedFieldsEntryPath := filepath.Join(sourceDir, "k8s", "apimachinery", "pkg", "apis", "meta", "v1", "managed_fields_entry.k")

	// Replace `managedFields?: [ManagedFieldsEntry]` with `managedFields?: any` in `object_meta.k`
	if _, err := fs.Stat(objectMetaPath); err == nil { // Check if the file exists
		content, err := afero.ReadFile(fs, objectMetaPath)
		if err != nil {
			return errors.Wrapf(err, "failed to read %s", objectMetaPath)
		}

		updatedContent := strings.ReplaceAll(string(content), "managedFields?: [ManagedFieldsEntry]", "managedFields?: any")

		if err := afero.WriteFile(fs, objectMetaPath, []byte(updatedContent), 0o644); err != nil {
			return errors.Wrapf(err, "failed to update %s", objectMetaPath)
		}
	}

	// Remove `managed_fields_entry.k` if it exists
	if _, err := fs.Stat(managedFieldsEntryPath); err == nil {
		if err := fs.Remove(managedFieldsEntryPath); err != nil {
			return errors.Wrapf(err, "failed to remove %s", managedFieldsEntryPath)
		}
	}

	// Copy the modified `k8s` directory to the targetDir
	k8sSourcePath := filepath.Join(sourceDir, "k8s")
	if err := filesystem.CopyFolder(fs, k8sSourcePath, filepath.Join(targetDir, "k8s")); err != nil {
		return errors.Wrap(err, "failed to copy k8s directory")
	}

	// Additional transformations remain the same, working on other files
	if err := afero.Walk(fs, sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || strings.HasPrefix(path, filepath.Join(sourceDir, "k8s")) {
			return nil
		}

		filename := info.Name()
		parts := strings.Split(filename, "_")

		// Identify the index of the known API version in the filename
		var versionIndex int
		foundVersion := false

		for i, part := range parts {
			if xcrd.IsKnownAPIVersion(part) {
				versionIndex = i
				foundVersion = true
				break
			}
		}

		if !foundVersion || versionIndex == 0 {
			return nil
		}

		// Take the segments before the version, reverse them, and append the version
		reversedParts := parts[:versionIndex]
		slices.Reverse(reversedParts)
		reversedParts = append(reversedParts, parts[versionIndex])

		// Construct the new directory structure by joining reversed parts
		newDir := filepath.Join(targetDir, filepath.Join(reversedParts...))

		// Ensure the new directory structure exists
		if err := fs.MkdirAll(newDir, 0o755); err != nil {
			return errors.Wrapf(err, "failed to create directory %s", newDir)
		}

		// Transform the filename after the version: remove underscores
		transformedName := strings.ReplaceAll(strings.Join(parts[versionIndex+1:], ""), "_", "")
		transformedName = strings.ReplaceAll(transformedName, "swagger", "")

		// Construct the new file path in the target directory with the transformed name
		newFilePath := filepath.Join(newDir, transformedName)

		// Copy the file to the new location
		srcFile, err := fs.Open(path)
		if err != nil {
			return errors.Wrapf(err, "failed to open source file %s", path)
		}

		destFile, err := fs.Create(newFilePath)
		if err != nil {
			return errors.Wrapf(err, "failed to create destination file %s", newFilePath)
		}

		_, err = io.Copy(destFile, srcFile)
		if err != nil {
			return errors.Wrapf(err, "failed to copy file from %s to %s", path, newFilePath)
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "error processing directory")
	}

	return nil
}
