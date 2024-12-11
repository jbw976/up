// Copyright 2024 Upbound Inc
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

package upbound

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// ParseRepository parse a repository and normalize it.
func ParseRepository(repository string, defaultRegistry string) (registry, org, repoName string, err error) {
	ref, err := name.NewRepository(repository, name.WithDefaultRegistry(defaultRegistry))
	if err != nil {
		return "", "", "", errors.Wrap(err, "failed to parse repository")
	}
	reg := ref.Registry.String()
	repo := ref.RepositoryStr()
	repoParts := strings.SplitN(repo, "/", 2)

	// Ensure that repoParts contains at least two elements
	if len(repoParts) < 2 {
		return "", "", "", fmt.Errorf("invalid repository format: %q, expected format 'org/repo' or 'registry/org/repo'", repo)
	}

	return reg, repoParts[0], repoParts[1], nil
}
