// Copyright 2025 Upbound Inc.
// All rights reserved

// Package common provides shared utilities for project commands.
package common

import (
	"github.com/spf13/afero"

	"github.com/upbound/up/internal/git"
	"github.com/upbound/up/internal/project"
)

// BuildManagerOptions creates project.ManagerOption slice with cache FS and optional git auth.
// The gitUsername parameter allows configuring the username for HTTPS authentication.
// For GitHub and GitLab, "x-access-token" (the default) works. For Bitbucket, use your actual username.
func BuildManagerOptions(cacheFS afero.Fs, gitToken, gitUsername string) []project.ManagerOption {
	opts := []project.ManagerOption{project.WithCacheFS(cacheFS)}
	if gitToken != "" {
		username := gitUsername
		if username == "" {
			username = "x-access-token"
		}
		opts = append(opts, project.WithGitAuthProvider(&git.HTTPSAuthProvider{
			Username: username,
			Password: gitToken,
		}))
	}
	return opts
}
