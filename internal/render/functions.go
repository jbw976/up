// Copyright 2025 Upbound Inc.
// All rights reserved

// Package render contains functions for composition rendering
package render

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/name"
	v1cache "github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	pkgmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"
	pkgv1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"

	"github.com/upbound/up/cmd/up/project/common"
	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/imageutil"
	"github.com/upbound/up/internal/oci/cache"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	projectv2alpha1 "github.com/upbound/up/pkg/apis/project/v2alpha1"
)

// LoadFunctions loads functions from a project's DependsOn list.
func LoadFunctions(ctx context.Context, proj *projectv2alpha1.Project, r manager.ImageResolver) ([]pkgv1.Function, error) {
	functions := make([]pkgv1.Function, 0, len(proj.Spec.DependsOn))

	for _, dep := range proj.Spec.DependsOn {
		dep, err := project.NormalizeDependency(dep)
		if err != nil {
			return nil, errors.Wrap(err, "invalid dependency")
		}
		if !isFunction(dep) {
			continue
		}

		// Convert function dependency
		convertedDep, ok := manager.ConvertToV1beta1(dep)
		if !ok {
			return nil, errors.Errorf("failed to convert dependency in %s", *dep.Package)
		}

		// Resolve tag for function
		version, err := r.ResolveTag(ctx, convertedDep)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve tag for function %s", *dep.Package)
		}

		// Parse function name
		functionRepo, err := name.NewRepository(*dep.Package, name.StrictValidation)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse function name reference for %s", *dep.Package)
		}

		// Create function package manifest
		f := pkgv1.Function{
			ObjectMeta: metav1.ObjectMeta{
				Name: xpkg.ToDNSLabel(functionRepo.RepositoryStr()),
			},
			Spec: pkgv1.FunctionSpec{
				PackageSpec: pkgv1.PackageSpec{
					Package: fmt.Sprintf("%s:%s", imageutil.RewriteImage(*dep.Package, proj.Spec.ImageConfig), version),
				},
			},
		}
		functions = append(functions, f)
	}

	return functions, nil
}

func isFunction(dep pkgmetav1.Dependency) bool {
	apiVersion := ptr.Deref(dep.APIVersion, "")
	kind := ptr.Deref(dep.Kind, "")

	// Support both v1 and v1beta1 functions
	isV1Function := apiVersion == pkgv1.FunctionGroupVersionKind.GroupVersion().String() && kind == pkgv1.FunctionKind
	isV1Beta1Function := apiVersion == pkgv1beta1.FunctionGroupVersionKind.GroupVersion().String() && kind == pkgv1beta1.FunctionKind

	return isV1Function || isV1Beta1Function
}

// embeddedFunctionsToDaemon loads each compatible image in the ImageTagMap into the Docker daemon.
func embeddedFunctionsToDaemon(ctx context.Context, imageMap project.ImageTagMap) ([]pkgv1.Function, error) {
	functions := make([]pkgv1.Function, 0, len(imageMap))

	// Detect the target daemon's architecture
	targetArch := getDockerDaemonArchitecture(ctx)

	for tag, img := range imageMap {
		platformInfo, err := img.ConfigFile()
		if err != nil {
			return nil, errors.Wrapf(err, "error getting platform info for image %s", tag)
		}

		if platformInfo.Architecture != targetArch {
			continue
		}

		// Push the image directly to the daemon
		if _, err := daemon.Write(tag, img); err != nil {
			return nil, errors.Wrapf(err, "error pushing image %s to daemon", tag)
		}

		f := pkgv1.Function{
			ObjectMeta: metav1.ObjectMeta{
				// align name with functionRef.name in composition
				Name: xpkg.ToDNSLabel(tag.Context().RepositoryStr()),
			},
			Spec: pkgv1.FunctionSpec{
				PackageSpec: pkgv1.PackageSpec{
					// set correct local image with tag
					Package: tag.Name(),
				},
			},
		}

		functions = append(functions, f)
	}

	return functions, nil
}

// BuildEmbeddedFunctionsLocalDaemon build and push to local deamon.
func BuildEmbeddedFunctionsLocalDaemon(ctx context.Context, upCtx *upbound.Context, opts FunctionOptions) ([]pkgv1.Function, error) {
	b := project.NewBuilder(
		project.BuildWithMaxConcurrency(opts.Concurrency),
		project.BuildWithFunctionIdentifier(opts.FunctionIdentifier),
	)

	imgMap, err := b.Build(ctx, upCtx, opts.Project, opts.ProjFS,
		project.BuildWithEventChannel(opts.EventChannel),
		project.BuildWithImageLabels(common.ImageLabels(opts)),
		project.BuildWithDependencyManager(opts.DependencyManager),
	)
	if err != nil {
		return nil, err
	}

	if !opts.NoBuildCache {
		cch := cache.NewValidatingCache(v1cache.NewFilesystemCache(opts.BuildCacheDir))
		for tag, img := range imgMap {
			imgMap[tag] = v1cache.Image(img, cch)
		}
	}

	stage := "Pushing embedded functions to local daemon"
	opts.EventChannel.SendEvent(stage, async.EventStatusStarted)
	efns, err := embeddedFunctionsToDaemon(ctx, imgMap)
	if err != nil {
		opts.EventChannel.SendEvent(stage, async.EventStatusFailure)
		return nil, errors.Wrap(err, "unable to push to local docker daemon")
	}
	opts.EventChannel.SendEvent(stage, async.EventStatusSuccess)

	return efns, nil
}

// getDockerDaemonArchitecture detects the Docker daemon's architecture.
// If DOCKER_HOST is set, it queries the daemon for its architecture.
// Otherwise, it returns the local runtime architecture.
func getDockerDaemonArchitecture(ctx context.Context) string {
	// Check if DOCKER_HOST is set, indicating a potentially remote daemon
	dockerHost := os.Getenv("DOCKER_HOST")

	// If DOCKER_HOST is not set or is a unix socket, use runtime.GOARCH
	// Unix sockets indicate local daemon, so runtime.GOARCH is appropriate
	if dockerHost == "" || strings.HasPrefix(dockerHost, "unix://") {
		return runtime.GOARCH
	}

	// For TCP/SSH connections, query the daemon for its architecture
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.FromEnv)
	if err != nil {
		// Fall back to runtime.GOARCH if we can't create a client
		return runtime.GOARCH
	}
	defer cli.Close() //nolint:errcheck // we don't care about the error

	// Get daemon info to determine architecture
	info, err := cli.Info(ctx)
	if err != nil {
		// Fall back to runtime.GOARCH if we can't get daemon info
		return runtime.GOARCH
	}

	// Map Docker's architecture naming to Go's GOARCH format
	arch := normalizeArchitecture(info.Architecture)
	return arch
}

// normalizeArchitecture converts Docker's architecture naming to Go's GOARCH format.
func normalizeArchitecture(dockerArch string) string {
	switch dockerArch {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	default:
		return dockerArch
	}
}
