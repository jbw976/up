// Copyright 2025 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schemagenerator

import (
	"context"
	"embed"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"golang.org/x/mod/modfile"
	"gotest.tools/v3/assert"
)

//go:embed testdata/*.yaml
var testdataFS embed.FS

func TestGenerateGo(t *testing.T) {
	inputFS := afero.NewBasePathFs(afero.FromIOFS{FS: testdataFS}, "testdata")
	schemaFS, err := GenerateSchemaGo(context.Background(), inputFS, nil, nil)
	assert.NilError(t, err)

	expectedFiles := []string{
		"models/go.mod",
		"models/io/k8s/meta/v1/meta.go",
		"models/co/acme/platform/v1alpha1/accountscaffold.go",
		"models/co/acme/platform/v1alpha1/xaccountscaffold.go",
	}

	files := token.NewFileSet()
	for _, path := range expectedFiles {
		exists, err := afero.Exists(schemaFS, path)
		assert.NilError(t, err)
		assert.Assert(t, exists, "expected model file %s does not exist", path)

		contents, err := afero.ReadFile(schemaFS, path)
		assert.NilError(t, err)

		// Basic validation of file contents - we're not going to make sure they
		// contain exactly the right stuff, just that they're syntactially OK
		// and have the right module and package names.
		switch filepath.Ext(path) {
		case ".go":
			f, err := parser.ParseFile(files, path, contents, parser.ParseComments)
			assert.NilError(t, err)
			expectedPackage := filepath.Base(filepath.Dir(path))
			assert.Equal(t, f.Name.Name, expectedPackage)

		case ".mod":
			mod, err := modfile.Parse(path, contents, nil)
			assert.NilError(t, err)
			assert.Equal(t, mod.Module.Mod.Path, "dev.upbound.io/models")
		}
	}
}
