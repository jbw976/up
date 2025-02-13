// Copyright 2025 Upbound Inc.
// All rights reserved

package xpls

import (
	"context"

	"github.com/sourcegraph/jsonrpc2"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/upbound/up/internal/xpls"
	"github.com/upbound/up/internal/xpls/handler"
)

// serveCmd starts the language server.
type serveCmd struct {
	// TODO(@tnthornton) cache dir doesn't seem to be the responsibility of the
	// serve command. It seems like we can easily get into an inconsistent state
	// if someone specifies config element from the command line. We should move
	// this to the config.
	Cache   string `default:"~/.up/cache"                   help:"Directory path for dependency schema cache." type:"path"`
	Verbose bool   `help:"Run server with verbose logging."`
}

// Run runs the language server.
func (c *serveCmd) Run(ctx context.Context) error {
	// cache directory resolution should occur at this level.

	// TODO(hasheddan): move to AfterApply.
	zl := zap.New(zap.UseDevMode(c.Verbose))
	h, err := handler.New(
		handler.WithLogger(logging.NewLogrLogger(zl.WithName("xpls"))),
	)
	if err != nil {
		return err
	}

	// TODO(hasheddan): handle graceful shutdown.
	<-jsonrpc2.NewConn(ctx, jsonrpc2.NewBufferedStream(xpls.StdRWC{}, jsonrpc2.VSCodeObjectCodec{}), h).DisconnectNotify()
	return nil
}
