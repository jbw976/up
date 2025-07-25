// Copyright 2025 Upbound Inc.
// All rights reserved

// Package manager implements a schema manager for use in control plane
// projects.
package manager

import (
	"context"
	"encoding/json"
	"io/fs"
	"path/filepath"
	"sync"

	"github.com/invopop/jsonschema"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/schemas/generator"
	"github.com/upbound/up/internal/schemas/runner"
)

// Manager is a schema manager. It manages a directory of schemas, generating
// new schemas only when necessary.
type Manager struct {
	fs         afero.Fs
	generators []generator.Interface
	runner     runner.SchemaRunner

	lockMu sync.RWMutex
}

// Add ensures schemas for resources in the given source are present in the
// managed directory.
func (m *Manager) Add(ctx context.Context, source Source) error {
	version, err := source.Version(ctx)
	if err != nil {
		return err
	}

	existing, err := m.currentVersion(source.ID())
	if err != nil {
		return err
	}
	if existing == version {
		// Current version schemas are already present, no need to regenerate.
		return nil
	}

	_, err = m.Generate(ctx, source)
	return err
}

// Generate generates and returns schemas using the manager's generators, and
// adds them to the manager. Unlike Add, Generate will always generate schemas,
// regardless of whether they're already present in the manager. Callers should
// prefer to use Add unless they require the schemas to be returned.
func (m *Manager) Generate(ctx context.Context, source Source) (map[string]afero.Fs, error) {
	version, err := source.Version(ctx)
	if err != nil {
		return nil, err
	}

	// If we have pre-generated schemas, use them. Note that we never generate
	// schemas for sources that *could* have pre-generated schemas, even if they
	// do not have schemas packaged. This is intentional, since we don't want to
	// enable client-side schema generation for xpkgs.
	schemas := make(map[string]afero.Fs)
	if ps, ok := source.(PackagedSource); ok {
		schemas, err = ps.Schemas()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get packaged schemas")
		}
	} else {
		fromFS, err := source.Resources(ctx)
		if err != nil {
			return nil, err
		}

		eg, egCtx := errgroup.WithContext(ctx)
		sourceType := source.Type()
		for _, gen := range m.generators {
			eg.Go(func() error {
				var schemaFS afero.Fs
				var err error

				switch sourceType {
				case SourceTypeCRD:
					schemaFS, err = gen.GenerateFromCRD(egCtx, fromFS, m.runner)
				case SourceTypeOpenAPI:
					schemaFS, err = gen.GenerateFromOpenAPI(egCtx, fromFS, m.runner)
				default:
					return errors.Errorf("unsupported source type %q", sourceType)
				}
				if err != nil {
					return err
				}

				if schemaFS != nil {
					schemas[gen.Language()] = schemaFS
				}

				return nil
			})
		}
		if err := eg.Wait(); err != nil {
			return nil, err
		}
	}

	// Copy the generated or retrieved schemas into our schema repository.
	for lang, fs := range schemas {
		langFS := afero.NewBasePathFs(m.fs, lang)
		if err := filesystem.CopyFilesBetweenFs(fs, langFS); err != nil {
			return nil, err
		}

		if err := postProcessModelsForLanguage(lang, langFS); err != nil {
			return nil, err
		}
	}

	return schemas, m.updateVersion(source.ID(), version)
}

// processModelsForLanguage does any language-specific work after adding models
// to the manager's model cache.
func postProcessModelsForLanguage(language string, langFS afero.Fs) error {
	switch language {
	case "json":
		// For JSON, create and write the index schema, an anyOf of all the
		// specific schemas we've collected from any source.
		schemas, err := afero.Glob(langFS, "models/*.schema.json")
		if err != nil {
			return err
		}

		metaFile := filepath.Join("models", "index.schema.json")
		var metaSchema jsonschema.Schema
		for _, schema := range schemas {
			if schema == metaFile {
				continue
			}
			metaSchema.AnyOf = append(metaSchema.AnyOf, &jsonschema.Schema{
				Ref: filepath.Base(schema),
			})
		}
		bs, err := json.Marshal(metaSchema)
		if err != nil {
			return err
		}

		return afero.WriteFile(langFS, metaFile, bs, 0o644)

	default:
		return nil
	}
}

func (m *Manager) currentVersion(id string) (string, error) {
	m.lockMu.RLock()
	defer m.lockMu.RUnlock()

	l, err := m.getLock()
	if err != nil {
		return "", err
	}

	return l.Packages[id], nil
}

func (m *Manager) updateVersion(id, version string) error {
	m.lockMu.Lock()
	defer m.lockMu.Unlock()

	l, err := m.getLock()
	if err != nil {
		return err
	}

	l.Packages[id] = version

	return m.updateLock(l)
}

// getLock should be called only when holding the lock mutex.
func (m *Manager) getLock() (*lock, error) {
	lf, err := m.fs.Open(lockFileName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return newLock(), nil
		}
		return nil, err
	}
	defer func() { _ = lf.Close() }()

	var l lock
	if err := json.NewDecoder(lf).Decode(&l); err != nil {
		return nil, err
	}

	return &l, nil
}

// updateLock should be called only when holding the lock mutex for writing.
func (m *Manager) updateLock(l *lock) error {
	// This looks weird, but afero will happily create a BasePathFs for a
	// nonexistent directory. Creating / in it makes sure we're able to write
	// the lock file.
	if err := m.fs.MkdirAll("/", 0o750); err != nil {
		return errors.Wrap(err, "failed to ensure schema directory exists")
	}

	bs, err := json.Marshal(l)
	if err != nil {
		return errors.Wrap(err, "failed to serialize schema lock")
	}

	if err := afero.WriteFile(m.fs, lockFileName, bs, 0o600); err != nil {
		return errors.Wrap(err, "failed to write schema lock file")
	}

	return nil
}

// New returns an initialized manager.
func New(fs afero.Fs, gens []generator.Interface, r runner.SchemaRunner) *Manager {
	return &Manager{
		fs:         fs,
		generators: gens,
		runner:     r,
	}
}
