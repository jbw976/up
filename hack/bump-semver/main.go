// bump-semver is a development utility that creates an appropriate version
// number for the releases we publish from main and release branches. Given a
// `git describe` (which will be based on the last release tagged on the
// branch), it increments the appropriate version component: minor when run on
// the main branch, or patch when run on a release branch.
//
// For example, assuming `git describe` returns "v0.41.0-1-gc2c5b728":
//
// - When run on the main branch, return "v0.42.0-1-gc2c5b728"
// - When run on a release branch, return "v0.41.1-1-gc2c5b728"
//
//nolint:forbidigo // This is a command-line utility, we can print.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/blang/semver/v4"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: bump-semver <current version> <branch>")
		os.Exit(1)
	}

	current := os.Args[1]
	branch := os.Args[2]
	next, err := incr(current, branch)
	if err != nil {
		fmt.Printf("Failed to increment version: %s\n", err)
		os.Exit(1)
	}

	fmt.Println(next)
}

func incr(current, branch string) (string, error) {
	current = strings.TrimPrefix(current, "v")

	sv, err := semver.Parse(current)
	if err != nil {
		return "", err
	}

	switch {
	case branch == "main":
		if err := sv.IncrementMinor(); err != nil {
			return "", err
		}

	case strings.HasPrefix(branch, "release-"):
		if err := sv.IncrementPatch(); err != nil {
			return "", err
		}

	default:
		return "", errors.New("unknown branch type: does not match main or release-*")
	}

	return "v" + sv.String(), nil
}
