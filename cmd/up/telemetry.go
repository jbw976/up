// Copyright 2025 Upbound Inc.
// All rights reserved

package main

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/cmd/up/version"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/otel"
)

// initOTEL initializes OpenTelemetry client and binds it to Kong context.
func (c *cli) initOTEL(ctx *kong.Context) error {
	// Get config source
	src := config.NewFSSource()
	if err := src.Initialize(); err != nil {
		return errors.Wrap(err, "failed to initialize config")
	}

	conf, err := config.Extract(src)
	if err != nil {
		return errors.Wrap(err, "failed to read config")
	}

	values, err := conf.GetBaseConfiguration()
	if err != nil {
		return errors.Wrap(err, "failed to get base configuration")
	}

	otelDisabled, ok := values[config.ConfigurationTelemetryDisabled]
	if !ok {
		otelDisabled = "false"
	}
	otelDisabledBool, err := strconv.ParseBool(otelDisabled)
	if err != nil {
		return errors.Wrap(err, "failed to parse telemetry.disabled configuration")
	}

	otelEndpoint, ok := values[config.ConfigurationTelemetryEndpoint]
	if !ok {
		otelEndpoint = config.DefaultTelemetryEndpoint
	}

	// Parse and normalize endpoint for OTLP gRPC exporter
	parsedURL, err := url.Parse(otelEndpoint)
	if err != nil {
		return errors.Wrap(err, "failed to parse telemetry endpoint")
	}

	// For OTLP gRPC, we need just host:port format
	if parsedURL.Host != "" {
		otelEndpoint = parsedURL.Host
	}

	otelDebug, ok := values[config.ConfigurationTelemetryDebug]
	if !ok {
		otelDebug = "false"
	}
	otelDebugBool, err := strconv.ParseBool(otelDebug)
	if err != nil {
		return errors.Wrap(err, "failed to parse telemetry.debug configuration")
	}
	c.otelDebug = otelDebugBool

	otelKey, ok := values[config.ConfigurationTelemetryKey]
	if !ok {
		otelKey = config.TelemetryAuthToken
	}

	otelInsecure, ok := values[config.ConfigurationTelemetryInsecure]
	if !ok {
		otelInsecure = "false"
	}
	otelInsecureBool, err := strconv.ParseBool(otelInsecure)
	if err != nil {
		return errors.Wrap(err, "failed to parse telemetry.insecure configuration")
	}

	otelIdentity, ok := values[config.ConfigurationTelemetryIdentity]
	if !ok {
		otelIdentity = ""
	}

	// Initialize OTEL client
	otelClient, err := otel.NewClient(otel.Config{
		Identity:    otelIdentity,
		ServiceName: "up-cli",
		Endpoint:    otelEndpoint,
		Disabled:    otelDisabledBool,
		Debug:       otelDebugBool,
		Key:         otelKey,
		Insecure:    otelInsecureBool, // yes, this is expected. We overload flag for now.
	})
	if err != nil {
		return err
	}

	// Store client globally for shutdown
	globalOTELClient = otelClient

	// Bind OTEL client and tracer to Kong context
	ctx.Bind(otelClient)
	ctx.BindTo(otelClient.Tracer(), (*trace.Tracer)(nil))

	return nil
}

func (c *cli) createCommandSpans(ctx context.Context, kongCtx *kong.Context) error {
	// Skip telemetry if client is not available (disabled or failed to initialize)
	if globalOTELClient == nil {
		kongCtx.BindTo(context.Background(), (*context.Context)(nil))
		return nil
	}

	// This will build only client version info. so no client calls.
	v := version.Cmd{Client: true}
	versionInfo := v.BuildVersionInfo(ctx, kongCtx, nil)

	// Create span only for the selected command
	rootCtx := context.Background()
	_, span := createCommandSpan(rootCtx, globalOTELClient, kongCtx.Selected())

	span.AddEvent("version.build", trace.WithAttributes(
		attribute.String("version.client", versionInfo.Client.Version),
		attribute.String("version.go", versionInfo.Client.GoVersion),
		attribute.String("version.os", versionInfo.Client.OS),
		attribute.String("version.arch", versionInfo.Client.Arch),
	))

	// Store span globally for cleanup in main()
	globalCommandSpan = span

	// Bind both context and span for commands to use
	// Commands can now:
	// 1. Run(...., span trace.Span) - to create child spans
	// 2. Add events: span.AddEvent("event-name", trace.WithAttributes(...)). See version.go for example.
	kongCtx.BindTo(span, (*trace.Span)(nil))

	return nil
}

// createCommandSpan creates a single telemetry span for the executed command.
// Returns the span context and span for the command to use during execution.
func createCommandSpan(ctx context.Context, otelClient *otel.Client, node *kong.Node) (context.Context, trace.Span) {
	tracer := otelClient.Tracer()

	// Build command path for span name (e.g., "up organization list")
	var cmdPath []string
	current := node
	for current != nil {
		if current.Name != "" {
			cmdPath = append([]string{current.Name}, cmdPath...) // prepend to get correct order
		}
		current = current.Parent
	}

	spanName := strings.Join(cmdPath, "-") // e.g. "up-organization-list"

	// Extract command metadata
	var attrs []trace.SpanStartOption

	// Add basic command info
	attrs = append(attrs, trace.WithAttributes(
		attribute.String("command.name", node.Name),
	))

	// Extract flags that were actually set (without sensitive values)
	if node.Flags != nil {
		var setFlags []string
		for _, flag := range node.Flags {
			if flag.Set && flag.Target.IsValid() {
				setFlags = append(setFlags, flag.Name)
			}
		}

		if len(setFlags) > 0 {
			attrs = append(attrs, trace.WithAttributes(
				attribute.StringSlice("flags.set", setFlags),
				attribute.Int("flags.count", len(setFlags)),
			))
		}
	}

	// Extract positional arguments info (without sensitive values)
	if len(node.Positional) > 0 {
		argCount := 0
		for _, arg := range node.Positional {
			if arg.Target.IsValid() {
				argValue := arg.Target.Interface()
				if argValue != nil {
					argCount++
				}
			}
		}

		if argCount > 0 {
			attrs = append(attrs, trace.WithAttributes(
				attribute.Int("args.count", argCount),
			))
		}
	}

	// Create span that will remain open during command execution
	// Note: This span will be ended in main() after ctx.Run()
	//nolint:spancheck // Span lifecycle managed globally
	return tracer.Start(ctx, spanName, attrs...)
}
