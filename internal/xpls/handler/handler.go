// Copyright 2025 Upbound Inc.
// All rights reserved

package handler

import (
	"context"

	"github.com/sourcegraph/jsonrpc2"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/upbound/up/internal/xpls/dispatcher"
	"github.com/upbound/up/internal/xpls/server"
)

// A Handler handles LSP requests.
type Handler struct {
	log        logging.Logger
	dispatcher *dispatcher.Dispatcher
	server     *server.Server
}

// New constructs a new LSP handler,.
func New(opts ...Option) (*Handler, error) {
	h := &Handler{
		log: logging.NewNopLogger(),
	}

	server, err := server.New(server.WithLogger(h.log))
	if err != nil {
		return nil, err
	}

	h.server = server

	h.dispatcher = dispatcher.New(dispatcher.WithLogger(h.log))

	for _, o := range opts {
		o(h)
	}

	return h, nil
}

// Option modifies a handler.
type Option func(h *Handler)

// WithLogger sets the logger for the handler.
func WithLogger(l logging.Logger) Option {
	return func(h *Handler) {
		h.log = l
	}
}

// Handle handles LSP requests. It panics if we cannot initialize the workspace.
func (h *Handler) Handle(ctx context.Context, conn *jsonrpc2.Conn, r *jsonrpc2.Request) { //nolint:gocyclo
	h.dispatcher.Dispatch(ctx, h.server, conn, r)
}
