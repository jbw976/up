// Copyright 2025 Upbound Inc.
// All rights reserved

package serve

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/mhrabovcin/troubleshoot-live/pkg/bundle"
	"github.com/mhrabovcin/troubleshoot-live/pkg/envtest"
	"github.com/mhrabovcin/troubleshoot-live/pkg/importer"
	"github.com/mhrabovcin/troubleshoot-live/pkg/kubernetes"
	"github.com/mhrabovcin/troubleshoot-live/pkg/proxy"
	"github.com/mhrabovcin/troubleshoot-live/pkg/rewriter"
	"k8s.io/klog/v2"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// Options configures the serve operation.
type Options struct {
	// BundlePath is the path to the support bundle directory or archive.
	BundlePath string

	// Host is the host and port to serve on (e.g., "localhost:8080").
	Host string

	// KubeconfigPath is where to write the kubeconfig file.
	KubeconfigPath string

	// EnvtestArch is the architecture for k8s server assets.
	EnvtestArch string

	// Debug enables debug output.
	Debug bool

	// Debugf is called to print debug messages when Debug is enabled.
	// If nil, debug messages are discarded.
	Debugf func(format string, args ...any)

	// OnServerReady is called when the server is ready to accept connections.
	OnServerReady func(proxyHTTPAddress, kubeconfigPath string)
}

// Start starts an HTTP server that serves support bundle resources as Kubernetes API responses.
// It blocks until the context is cancelled or an error occurs.
func Start(ctx context.Context, opts Options) error {
	// Always suppress this log output, it's noisy and not useful.
	originalLogWriter := log.Writer()
	originalLogFlags := log.Flags()
	log.SetOutput(io.Discard)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(originalLogWriter)
		log.SetFlags(originalLogFlags)
	}()

	// // Always suppress klog warnings from API server (these are noisy and not useful)
	fs := flag.NewFlagSet("klog", flag.ContinueOnError)
	klog.InitFlags(fs)
	// Set verbosity to 0 and logtostderr to false to suppress warnings
	_ = fs.Parse([]string{"--v=0", "--logtostderr=false", "--alsologtostderr=false"})
	klog.SetOutput(io.Discard)

	supportBundle, err := bundle.New(opts.BundlePath)
	if err != nil {
		return errors.Wrapf(err, "failed to read bundle from path %q", opts.BundlePath)
	}

	ctx, done := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer done()

	testEnv, err := startAPIServer(ctx, supportBundle, opts)
	if err != nil {
		return errors.Wrap(err, "failed to start API server")
	}

	defer func() {
		if err := testEnv.Stop(); err != nil {
			// Silently ignore stop errors
			_ = err
		}
	}()

	err = importer.ImportBundle(ctx, supportBundle, testEnv.Config, &outputAdapter{debug: opts.Debug, debugf: opts.Debugf})
	if err != nil {
		if opts.Debugf != nil {
			opts.Debugf("bundle import incomplete, some resources may be missing: %v", err)
		}
		// Continue anyway - some resources may have been imported but maybe not all.
	}

	proxyHost := fmt.Sprintf("http://%s", opts.Host)
	kubeconfigPath, err := kubernetes.WriteProxyKubeconfig(proxyHost, opts.KubeconfigPath)
	if err != nil {
		return errors.Wrap(err, "failed to create kubeconfig")
	}

	if opts.OnServerReady != nil {
		opts.OnServerReady(proxyHost, kubeconfigPath)
	}

	proxyHandler := proxy.New(testEnv.Config, supportBundle, rewriter.Default())

	s := &http.Server{
		Addr:              opts.Host,
		Handler:           proxyHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(shutdownCtx)
	}()

	return ignoreServerClosedError(s.ListenAndServe())
}

func ignoreServerClosedError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func startAPIServer(
	ctx context.Context,
	supportBundle bundle.Bundle,
	opts Options,
) (*envtest.Environment, error) {
	arch := opts.EnvtestArch
	if arch == "" {
		arch = runtime.GOARCH
	}

	testEnv, err := envtest.Prepare(ctx, supportBundle, envtest.Arch(arch))
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare Kubernetes API server environment")
	}

	// Always suppress API server output
	testEnv.ControlPlane.GetAPIServer().Out = io.Discard
	testEnv.ControlPlane.GetAPIServer().Err = io.Discard

	_, err = testEnv.Start()
	if err != nil {
		if stopErr := testEnv.Stop(); stopErr != nil {
			return nil, errors.Wrapf(err, "failed to start test environment; also failed to stop: %v", stopErr)
		}
		return nil, errors.Wrap(err, "failed to start test environment")
	}

	return testEnv, nil
}
