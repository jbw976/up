// Copyright 2025 Upbound Inc.
// All rights reserved

package image

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/xpkg/dep/utils"
)

const (
	// DefaultVer effectively defines latest for the semver constraints.
	DefaultVer = ">=v0.0.0"

	packageTagFmt = "%s:%s"

	errInvalidConstraint  = "invalid dependency constraint"
	errInvalidProviderRef = "invalid package reference"
	errFailedToFetchTags  = "failed to fetch tags"
	errNoMatchingVersion  = "supplied version does not match an existing version"
	errTagDoesNotExist    = "supplied tag does not exist in the registry"
)

// Resolver --.
type Resolver struct {
	f Fetcher
}

// Fetcher defines how we expect to intract with the Image repository.
type Fetcher interface {
	Fetch(ctx context.Context, ref name.Reference, secrets ...string) (v1.Image, error)
	Head(ctx context.Context, ref name.Reference, secrets ...string) (*v1.Descriptor, error)
	Tags(ctx context.Context, ref name.Reference, secrets ...string) ([]string, error)
}

// NewResolver returns a new Resolver.
func NewResolver(opts ...ResolverOption) *Resolver {
	r := &Resolver{
		f: NewLocalFetcher(),
	}

	for _, o := range opts {
		o(r)
	}
	return r
}

// ResolverOption modifies the image resolver.
type ResolverOption func(*Resolver)

// WithFetcher modifies the Resolver and adds the given fetcher.
func WithFetcher(f Fetcher) ResolverOption {
	return func(r *Resolver) {
		r.f = f
	}
}

// ResolveImage resolves the image corresponding to the given v1beta1.Dependency.
func (r *Resolver) ResolveImage(ctx context.Context, d v1beta1.Dependency) (string, v1.Image, *v1.Descriptor, error) {
	var (
		cons string
		err  error
		path string
	)

	if utils.IsDigest(&d) {
		cons, err = r.ResolveDigest(ctx, d)
		if err != nil {
			return "", nil, nil, errors.Errorf("failed to resolve %s@%s: %w", d.Package, d.Constraints, err)
		}
		path = fmt.Sprintf("%s@%s", d.Package, cons)
	} else {
		cons, err = r.ResolveTag(ctx, d)
		if err != nil {
			return "", nil, nil, errors.Errorf("failed to resolve %s:%s: %w", d.Package, d.Constraints, err)
		}
		path = FullTag(v1beta1.Dependency{
			Package:     d.Package,
			Type:        d.Type,
			Constraints: cons,
		})
	}

	remoteImageRef, err := name.ParseReference(path)
	if err != nil {
		return "", nil, nil, err
	}

	i, err := r.f.Fetch(ctx, remoteImageRef)
	if err != nil {
		return "", nil, nil, err
	}

	digest, err := r.f.Head(ctx, remoteImageRef)
	return cons, i, digest, err
}

// ResolveTag resolves the tag corresponding to the given v1beta1.Dependency.
// TODO(@tnthornton) add a test that flexes resolving constraint versions to the expected target version.
func (r *Resolver) ResolveTag(ctx context.Context, dep v1beta1.Dependency) (string, error) { //nolint:gocyclo
	// if the passed in version was blank use the default to pass
	// constraint checks and grab latest semver
	if dep.Constraints == "" {
		dep.Constraints = DefaultVer
	}

	// check up front if we already have a valid semantic version
	v, err := semver.NewVersion(dep.Constraints)
	if err != nil && !errors.Is(err, semver.ErrInvalidSemVer) {
		return "", err
	}

	if v != nil {
		// version is a valid semantic version, check if it's a real tag
		_, err := r.ResolveDigest(ctx, dep)
		if err != nil {
			return "", err
		}
		return dep.Constraints, nil
	}

	// supplied version may be a semantic version constraint
	c, err := semver.NewConstraint(dep.Constraints)
	if err != nil {
		return "", errors.Wrap(err, errInvalidConstraint)
	}

	ref, err := name.ParseReference(dep.Identifier())
	if err != nil {
		return "", errors.Wrap(err, errInvalidProviderRef)
	}

	tags, err := r.f.Tags(ctx, ref)
	if err != nil {
		return "", errors.Wrap(err, errFailedToFetchTags)
	}

	vs := []*semver.Version{}
	for _, r := range tags {
		v, err := semver.NewVersion(r)
		if err != nil {
			// Skip any tags that are not valid semantic versions.
			continue
		}
		vs = append(vs, v)
	}

	// Sort the versions in ascending order
	sort.Sort(semver.Collection(vs))

	var ver string
	for _, v := range vs {
		if c.Check(v) {
			ver = v.Original()
		}
	}

	// If no matching version was found, show the latest 3 possible versions
	if ver == "" {
		// Determine the latest 3 available versions (or fewer if there aren't 3)
		numVersionsToShow := 3
		if len(vs) < 3 {
			numVersionsToShow = len(vs)
		}

		latestVersions := vs[len(vs)-numVersionsToShow:] // Get the last `numVersionsToShow` elements
		availableVersions := []string{}
		for _, v := range latestVersions {
			availableVersions = append(availableVersions, v.Original())
		}

		return "", errors.Errorf("%s. Latest available versions: %v", errNoMatchingVersion, availableVersions)
	}

	return ver, nil
}

// ResolveDigest performs a head request to the configured registry in order to determine
// if the provided version corresponds to a real tag and what the digest of that tag is.
func (r *Resolver) ResolveDigest(ctx context.Context, d v1beta1.Dependency) (string, error) {
	ref, err := name.ParseReference(d.Identifier(), name.WithDefaultTag(d.Constraints))
	if err != nil {
		return "", errors.Wrap(err, errInvalidProviderRef)
	}

	desc, err := r.f.Head(ctx, ref)
	if err != nil {
		var e *transport.Error
		if errors.As(err, &e) {
			if e.StatusCode == http.StatusNotFound {
				// couldn't find the specified tag, it appears to be invalid
				return "", errors.New(errTagDoesNotExist)
			}
		}
		return "", err
	}
	return desc.Digest.String(), nil
}

// FullTag returns the full image tag "source:version" of the given dependency.
func FullTag(d v1beta1.Dependency) string {
	// NOTE(@tnthornton) this should ONLY be used after the version constraint
	// has been resolved for the given dependency. Using a semver range is not
	// a valid tag format and will cause lookups to this string to fail.
	return fmt.Sprintf(packageTagFmt, d.Package, d.Constraints)
}
