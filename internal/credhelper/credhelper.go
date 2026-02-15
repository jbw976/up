// Copyright 2025 Upbound Inc.
// All rights reserved

package credhelper

import (
	"strings"
	"time"

	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/golang-jwt/jwt/v5"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/profile"
)

const (
	errUnimplemented     = "operation is not implemented"
	errInitializeSource  = "unable to initialize source"
	errExtractConfig     = "unable to extract config"
	errGetDefaultProfile = "unable to get default profile in config"
	errGetProfile        = "unable to get specified profile in config"
	errUnsupportedDomain = "supplied server URL is not supported"
)

const (
	defaultDockerUser = "_token"
)

// Helper is a docker credential helper for Upbound.
type Helper struct {
	log logging.Logger

	profile string
	domain  string
	src     config.Source
}

// Opt sets a helper option.
type Opt func(h *Helper)

// WithLogger sets the helper logger.
func WithLogger(l logging.Logger) Opt {
	return func(h *Helper) {
		h.log = l
	}
}

// WithDomain sets the allowed registry domain.
func WithDomain(d string) Opt {
	return func(h *Helper) {
		h.domain = d
	}
}

// WithProfile sets the helper profile.
func WithProfile(p string) Opt {
	return func(h *Helper) {
		h.profile = p
	}
}

// WithSource sets the source for the helper config.
func WithSource(src config.Source) Opt {
	return func(h *Helper) {
		h.src = src
	}
}

// New constructs a new Docker credential helper.
func New(opts ...Opt) *Helper {
	h := &Helper{
		log: logging.NewNopLogger(),
		src: config.NewFSSource(),
	}

	for _, o := range opts {
		o(h)
	}

	return h
}

// Add adds the supplied credentials.
func (h *Helper) Add(c *credentials.Credentials) error {
	return errors.New(errUnimplemented)
}

// Delete deletes credentials for the supplied server.
func (h *Helper) Delete(serverURL string) error {
	return errors.New(errUnimplemented)
}

// List lists all the configured credentials.
func (h *Helper) List() (map[string]string, error) {
	return nil, errors.New(errUnimplemented)
}

// Get gets credentials for the supplied server.
func (h *Helper) Get(serverURL string) (string, string, error) {
	if !strings.Contains(serverURL, h.domain) {
		return "", "", errors.New(errUnsupportedDomain)
	}
	if err := h.src.Initialize(); err != nil {
		return "", "", errors.Wrap(err, errInitializeSource)
	}
	conf, err := config.Extract(h.src)
	if err != nil {
		return "", "", errors.Wrap(err, errExtractConfig)
	}
	var p profile.Profile
	if h.profile == "" {
		_, p, err = conf.GetDefaultUpboundProfile()
		if err != nil {
			return "", "", errors.Wrap(err, errGetDefaultProfile)
		}
	} else {
		p, err = conf.GetUpboundProfile(h.profile)
		if err != nil {
			return "", "", errors.Wrap(err, errGetProfile)
		}
	}
	if p.Session == "" {
		return "", "", credentials.NewErrCredentialsNotFound()
	}
	if !hasValidSession(p.Session) {
		return "", "", credentials.NewErrCredentialsNotFound()
	}
	return defaultDockerUser, p.Session, nil
}

// hasValidSession checks whether the session token is a valid, non-expired JWT.
// If the token cannot be parsed or is expired, it returns false so that callers
// can fall back to anonymous access for public artifacts.
func hasValidSession(session string) bool {
	parser := jwt.NewParser()
	token, _, err := parser.ParseUnverified(session, jwt.MapClaims{})
	if err != nil {
		return false
	}
	exp, err := token.Claims.GetExpirationTime()
	if err != nil {
		return false
	}
	if exp == nil {
		return true
	}
	// Treat the token as expired if it expires within the next 30 seconds.
	return time.Now().Add(30 * time.Second).Before(exp.Time)
}
