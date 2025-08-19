// Copyright 2025 Upbound Inc.
// All rights reserved

package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/otel"
	"github.com/upbound/up/internal/version"
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

	otelDisabled := conf.GetBaseConfigurationValue(config.ConfigurationTelemetryDisabled, "false")
	otelDisabledBool, err := strconv.ParseBool(otelDisabled)
	if err != nil {
		return errors.Wrap(err, "failed to parse telemetry.disabled configuration")
	}

	otelEndpoint := conf.GetBaseConfigurationValue(config.ConfigurationTelemetryEndpoint, config.DefaultTelemetryEndpoint)
	// Parse and normalize endpoint for OTLP gRPC exporter
	parsedURL, err := url.Parse(otelEndpoint)
	if err != nil {
		return errors.Wrap(err, "failed to parse telemetry endpoint")
	}

	// For OTLP gRPC, we need just host:port format
	if parsedURL.Host != "" {
		otelEndpoint = parsedURL.Host
	}

	otelDebug := conf.GetBaseConfigurationValue(config.ConfigurationTelemetryDebug, "false")
	otelDebugBool, err := strconv.ParseBool(otelDebug)
	if err != nil {
		return errors.Wrap(err, "failed to parse telemetry.debug configuration")
	}
	c.otelDebug = otelDebugBool

	otelInsecure := conf.GetBaseConfigurationValue(config.ConfigurationTelemetryInsecure, "false")
	otelInsecureBool, err := strconv.ParseBool(otelInsecure)
	if err != nil {
		return errors.Wrap(err, "failed to parse telemetry.insecure configuration")
	}

	// Initialize OTEL client
	otelClient, err := otel.NewClient(otel.Config{
		Identity:    conf.GetBaseConfigurationValue(config.ConfigurationTelemetryIdentity, ""),
		ServiceName: "up-cli",
		Endpoint:    otelEndpoint,
		Disabled:    otelDisabledBool,
		Debug:       otelDebugBool,
		Key:         conf.GetBaseConfigurationValue(config.ConfigurationTelemetryKey, config.TelemetryAuthToken),
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

func createCommandSpans(kongCtx *kong.Context) trace.Span {
	// Skip telemetry if client is not available (disabled or failed to initialize)
	if globalOTELClient == nil {
		return nil
	}

	// Create span only for the selected command
	rootCtx := context.Background()
	_, span := createCommandSpan(rootCtx, globalOTELClient, kongCtx.Selected())

	span.AddEvent("version.build", trace.WithAttributes(
		attribute.String("version.client", version.Version()),
		attribute.String("version.go", runtime.Version()),
		attribute.String("version.os", runtime.GOOS),
		attribute.String("version.arch", runtime.GOARCH),
	))

	return span
}

// createCommandSpan creates a single telemetry span for the executed command.
// Returns the span context and span for the command to use during execution.
func createCommandSpan(ctx context.Context, otelClient *otel.Client, node *kong.Node) (context.Context, trace.Span) {
	tracer := otelClient.Tracer()

	// The span name will be the command path (e.g., 'up organization list')
	// with spaces replaced by hyphens (e.g., 'up-organization-list'). We build
	// the path manually instead of using `node.FullPath` because we don't want
	// parenthesized aliases in our path (e.g., `up controlplane (ctp) list`).
	var cmdPath []string
	current := node
	for current != nil {
		if current.Name != "" {
			cmdPath = append([]string{current.Name}, cmdPath...) // prepend to get correct order
		}
		current = current.Parent
	}

	spanName := strings.Join(cmdPath, "-")

	// Extract command metadata
	var attrs []trace.SpanStartOption

	// Add basic command info
	attrs = append(attrs, trace.WithAttributes(
		attribute.String("command.full", strings.Join(cmdPath, " ")),
		attribute.String("command.name", node.Name),
	))

	// Add CI info if we detect we're running in CI.
	attrs = append(attrs, ciAttrs()...)

	// Extract flags that were actually set (without sensitive values)
	if node.Flags != nil {
		attrs = append(attrs, flagAttrs(node)...)
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

func flagAttrs(node *kong.Node) []trace.SpanStartOption {
	var (
		setFlags   []string
		flagValues = map[string]string{}
	)

	for _, flag := range node.Flags {
		if flag.Set && flag.Target.IsValid() {
			setFlags = append(setFlags, flag.Name)
			if flag.Tag != nil && parseBool(flag.Tag.Get("telemetry")) {
				flagValues[flag.Name] = flag.Value.Target.String()
			}
		}
	}

	attrs := make([]trace.SpanStartOption, 0, len(flagValues)+2)
	if len(setFlags) > 0 {
		attrs = append(attrs, trace.WithAttributes(
			attribute.StringSlice("flags.set", setFlags),
			attribute.Int("flags.count", len(setFlags)),
		))
	}

	for name, value := range flagValues {
		attrs = append(attrs, trace.WithAttributes(
			attribute.String(fmt.Sprintf("flags.values.%s", name), value),
		))
	}

	return attrs
}

func ciAttrs() []trace.SpanStartOption {
	// Environment variables used to detect particular CI environments, from
	// https://learn.microsoft.com/en-us/dotnet/core/tools/telemetry#continuous-integration-detection.
	ciEnvs := map[string]func() bool{
		"GitHub Actions": func() bool {
			return parseBool(os.Getenv("GITHUB_ACTIONS"))
		},
		"Azure Pipelines": func() bool {
			return parseBool(os.Getenv("TF_BUILD"))
		},
		"Appveyor": func() bool {
			return parseBool(os.Getenv("APPVEYOR"))
		},
		"Travis CI": func() bool {
			return parseBool(os.Getenv("TRAVIS"))
		},
		"Circle CI": func() bool {
			return parseBool(os.Getenv("CIRCLECI"))
		},
		"AWS CodeBuild": func() bool {
			return os.Getenv("CODEBUILD_BUILD_ID") != "" && os.Getenv("AWS_REGION") != ""
		},
		"Jenkins": func() bool {
			return os.Getenv("BUILD_ID") != "" && os.Getenv("BUILD_URL") != ""
		},
		"Google Cloud Build": func() bool {
			return os.Getenv("BUILD_ID") != "" && os.Getenv("PROJECT_ID") != ""
		},
		"TeamCity": func() bool {
			return os.Getenv("TEAMCITY_VERSION") != ""
		},
		"JetBrains Space": func() bool {
			return os.Getenv("JB_SPACE_API_URL") != ""
		},
	}

	attrs := make([]trace.SpanStartOption, 0, 2)

	// First, check the generic CI variable. If it's non-empty, we're most
	// likely in a CI environment, so set the ci.env attr to true.
	if os.Getenv("CI") != "" {
		attrs = append(attrs, trace.WithAttributes(attribute.Bool("ci.env", true)))
	}

	// Now, check for individual providers and fill in ci.provider if possible.
	for provider, fn := range ciEnvs {
		if fn() {
			attrs = append(attrs, trace.WithAttributes(attribute.String("ci.provider", provider)))
			break
		}
	}

	return attrs
}

// parseBool parses a boolean value from s, returning false if the string is not
// a bool.
func parseBool(s string) bool {
	b, err := strconv.ParseBool(s)
	return b && err == nil
}
