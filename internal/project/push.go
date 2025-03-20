// Copyright 2025 Upbound Inc.
// All rights reserved

// Package project contains types for working with Upbound development projects.
package project

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/sync/errgroup"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	sdkerrs "github.com/upbound/up-sdk-go/errors"
	"github.com/upbound/up-sdk-go/service/repositories"
	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/pkg/apis/project/v1alpha1"
)

// Pusher is able to push a set of packages built from a project to a registry.
type Pusher interface {
	// Push pushes a set of packages built from a project to a registry and
	// returns the tag to which the configuration package was pushed.
	Push(ctx context.Context, project *v1alpha1.Project, imgMap ImageTagMap, opts ...PushOption) (name.Tag, error)
}

// PusherOption configures a pusher.
type PusherOption func(p *realPusher)

// PushWithTransport sets the HTTP transport to be used by the pusher.
func PushWithTransport(t http.RoundTripper) PusherOption {
	return func(p *realPusher) {
		p.transport = t
	}
}

// PushWithMaxConcurrency sets the maximum concurrency for pushing packages.
func PushWithMaxConcurrency(n uint) PusherOption {
	return func(b *realPusher) {
		b.maxConcurrency = n
	}
}

// PushWithUpboundContext provides an Upbound context to be used during the
// push. It is used to determine whether repository names are in the Upbound
// registry or not, and for Upbound API credentials. Registry credentials are
// provided separately, using `WithAuthKeychain`.
func PushWithUpboundContext(upCtx *upbound.Context) PusherOption {
	return func(p *realPusher) {
		p.upCtx = upCtx
	}
}

// PushWithAuthKeychain provides a registry credentail source to be used by the
// push.
func PushWithAuthKeychain(kc authn.Keychain) PusherOption {
	return func(p *realPusher) {
		p.keychain = kc
	}
}

// PushOption configures a build.
type PushOption func(o *pushOptions)

type pushOptions struct {
	eventChan                async.EventChannel
	tag                      string
	createPublicRepositories bool
}

// PushWithEventChannel provides a channel to which progress updates will be
// written during the push. It is the caller's responsibility to manage the
// lifecycle of this channel.
func PushWithEventChannel(ch async.EventChannel) PushOption {
	return func(o *pushOptions) {
		o.eventChan = ch
	}
}

// PushWithTag sets the tag to be used for the pushed packages.
func PushWithTag(tag string) PushOption {
	return func(o *pushOptions) {
		o.tag = tag
	}
}

// PushWithCreatePublicRepositories configures whether to create any new
// repositories with public visibility. Private is the default.
func PushWithCreatePublicRepositories(public bool) PushOption {
	return func(o *pushOptions) {
		o.createPublicRepositories = public
	}
}

type realPusher struct {
	upCtx          *upbound.Context
	keychain       authn.Keychain
	transport      http.RoundTripper
	maxConcurrency uint
}

// Push implements the Pusher interface.
func (p *realPusher) Push(ctx context.Context, project *v1alpha1.Project, imgMap ImageTagMap, opts ...PushOption) (name.Tag, error) { //nolint:gocyclo // This isn't too complex.
	os := &pushOptions{
		// TODO(adamwg): Consider smarter tag generation using git metadata if
		// the project lives in a git repository, or the package digest.
		tag: fmt.Sprintf("v0.0.0-%d", time.Now().Unix()),
	}

	for _, opt := range opts {
		opt(os)
	}

	imgTag, err := name.NewTag(fmt.Sprintf("%s:%s", project.Spec.Repository, os.tag))
	if err != nil {
		return imgTag, errors.Wrap(err, "failed to construct image tag")
	}

	cfgImage, fnImages, err := sortImages(imgMap, project.Spec.Repository)
	if err != nil {
		return imgTag, err
	}

	if isUpboundRepository(p.upCtx, imgTag.Repository) && p.upCtx.Profile.TokenType != profile.TokenTypeRobot {
		stage := "Ensuring repository exists"
		os.eventChan.SendEvent(stage, async.EventStatusStarted)
		err = p.createRepository(ctx, imgTag.Repository, os.createPublicRepositories)
		if err != nil {
			os.eventChan.SendEvent(stage, async.EventStatusFailure)
			return imgTag, err
		}
		os.eventChan.SendEvent(stage, async.EventStatusSuccess)
	}

	// Push all the function packages in parallel.
	eg, egCtx := errgroup.WithContext(ctx)
	// Semaphore to limit the number of functions we push in parallel.
	sem := make(chan struct{}, p.maxConcurrency)
	for repo, images := range fnImages {
		eg.Go(func() error {
			sem <- struct{}{}
			defer func() {
				<-sem
			}()

			stage := fmt.Sprintf("Pushing function package %s", repo)
			os.eventChan.SendEvent(stage, async.EventStatusStarted)
			// Create the subrepository if needed. We can only do this for the
			// Upbound registry; assume other registries will create on push.
			if isUpboundRepository(p.upCtx, repo) && p.upCtx.Profile.TokenType != profile.TokenTypeRobot {
				err := p.createRepository(egCtx, repo, os.createPublicRepositories)
				if err != nil {
					os.eventChan.SendEvent(stage, async.EventStatusFailure)
					return errors.Wrapf(err, "failed to create repository for function %q", repo)
				}
			}

			tag := repo.Tag(os.tag)
			err := p.pushIndex(egCtx, tag, images...)
			if err != nil {
				os.eventChan.SendEvent(stage, async.EventStatusFailure)
				return errors.Wrapf(err, "failed to push function %q", repo)
			}
			os.eventChan.SendEvent(stage, async.EventStatusSuccess)
			return nil
		})
	}

	err = eg.Wait()
	if err != nil {
		return imgTag, err
	}

	// Once the functions are pushed, push the configuration package.
	stage := fmt.Sprintf("Pushing configuration image %s", imgTag)
	os.eventChan.SendEvent(stage, async.EventStatusStarted)
	err = p.pushImage(ctx, imgTag, cfgImage)
	if err != nil {
		os.eventChan.SendEvent(stage, async.EventStatusFailure)
		return imgTag, errors.Wrap(err, "failed to push configuration package")
	}
	os.eventChan.SendEvent(stage, async.EventStatusSuccess)

	return imgTag, nil
}

func (p *realPusher) createRepository(ctx context.Context, repo name.Repository, public bool) error {
	org, repoName, ok := strings.Cut(repo.RepositoryStr(), "/")
	if !ok {
		return errors.New("invalid repository: must be of the form <organization>/<name>")
	}
	cfg, err := p.upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	client := repositories.NewClient(cfg)

	// Check if the repository exists
	existingRepo, err := client.Get(ctx, org, repoName)
	if err != nil && !sdkerrs.IsNotFound(err) {
		return errors.Wrap(err, "failed to search repository")
	}

	if existingRepo != nil {
		return nil
	}

	visibility := repositories.WithPrivate()
	if public {
		visibility = repositories.WithPublic()
	}
	if err := client.CreateOrUpdateWithOptions(ctx, org, repoName, visibility); err != nil {
		return errors.Wrap(err, "failed to create repository")
	}

	return nil
}

func (p *realPusher) pushIndex(ctx context.Context, tag name.Tag, imgs ...v1.Image) error {
	// Build an index. This is a little superfluous if there's only one image
	// (single architecture), but we generate configuration dependencies on
	// embedded functions assuming there's an index, so we push an index
	// regardless of whether we really need one.
	idx, imgs, err := xpkg.BuildIndex(imgs...)
	if err != nil {
		return err
	}

	// Push the images by digest.
	repo := tag.Repository
	for _, img := range imgs {
		dgst, err := img.Digest()
		if err != nil {
			return err
		}
		err = p.pushImage(ctx, repo.Digest(dgst.String()), img)
		if err != nil {
			return err
		}
	}

	// Tag the function the same as the configuration. The configuration depends
	// on it by digest, so this isn't necessary for things to work correctly,
	// but it makes the Marketplace experience more intuitive for the user.
	return remote.WriteIndex(tag, idx,
		remote.WithAuthFromKeychain(p.keychain),
		remote.WithContext(ctx),
		remote.WithTransport(p.transport),
	)
}

func (p *realPusher) pushImage(ctx context.Context, ref name.Reference, img v1.Image) error {
	img, err := xpkg.AnnotateImage(img)
	if err != nil {
		return err
	}

	return remote.Write(ref, img,
		remote.WithAuthFromKeychain(p.keychain),
		remote.WithContext(ctx),
		remote.WithTransport(p.transport),
	)
}

func isUpboundRepository(upCtx *upbound.Context, tag name.Repository) bool {
	if upCtx == nil {
		return false
	}
	return strings.HasPrefix(tag.RegistryStr(), upCtx.RegistryEndpoint.Hostname())
}

func sortImages(imgMap ImageTagMap, repo string) (cfgImage v1.Image, fnImages map[name.Repository][]v1.Image, err error) {
	cfgTag, err := name.NewTag(fmt.Sprintf("%s:%s", repo, ConfigurationTag))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to construct configuration tag")
	}

	fnImages = make(map[name.Repository][]v1.Image)
	for tag, image := range imgMap {
		if tag == cfgTag {
			cfgImage = image
			continue
		}

		fnImages[tag.Repository] = append(fnImages[tag.Repository], image)
	}

	if cfgImage == nil {
		return nil, nil, errors.New("failed to find configuration image")
	}

	return cfgImage, fnImages, nil
}

// NewPusher returns a new project Pusher.
func NewPusher(opts ...PusherOption) Pusher {
	p := &realPusher{
		transport:      http.DefaultTransport,
		maxConcurrency: 8,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}
