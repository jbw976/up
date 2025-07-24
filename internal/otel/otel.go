// Copyright 2025 Upbound Inc.
// All rights reserved

// Package otel provides OpenTelemetry functionality.
package otel

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc/grpclog"
)

// Config holds configuration for OpenTelemetry.
type Config struct {
	ServiceName string
	Endpoint    string
	Headers     map[string]string
	Insecure    bool
	Disabled    bool
	Debug       bool
	Key         string
}

// Client wraps OpenTelemetry functionality.
type Client struct {
	config         Config
	tracer         trace.Tracer
	tracerProvider *sdktrace.TracerProvider
}

// NewClient creates a new OpenTelemetry client.
func NewClient(config Config) (*Client, error) {
	client := &Client{
		config: config,
	}

	if config.Disabled {
		client.tracer = noop.NewTracerProvider().Tracer(config.ServiceName)
		return client, nil
	}

	if err := client.initTracing(); err != nil {
		return nil, fmt.Errorf("failed to initialize tracing: %w", err)
	}

	return client, nil
}

// initTracing initializes OpenTelemetry tracing.
func (c *Client) initTracing() error {
	ctx := context.Background()

	headers := c.config.Headers
	if headers == nil {
		headers = make(map[string]string)
	}
	if c.config.Key != "" {
		headers["x-upbound-authorization"] = fmt.Sprintf("Bearer %s", c.config.Key)
	}

	// Create OTLP exporter options
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(c.config.Endpoint),
		otlptracegrpc.WithHeaders(headers),
		// TODO(mjudeikis): We need dynamic detection if gzip is installed
		// otlptracegrpc.WithCompressor("gzip"),
	}

	// Configure custom gRPC logger to suppress connection errors
	// and silence other log messages.
	if !c.config.Debug {
		grpclog.SetLoggerV2(&quietGRPCLogger{})
	} else {
		grpclog.SetLoggerV2(grpclog.NewLoggerV2(os.Stdout, os.Stdout, os.Stderr))
	}

	if c.config.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(c.config.ServiceName),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	c.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global tracer provider
	otel.SetTracerProvider(c.tracerProvider)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create tracer
	c.tracer = c.tracerProvider.Tracer(c.config.ServiceName)

	return nil
}

// Tracer returns the OpenTelemetry tracer.
func (c *Client) Tracer() trace.Tracer {
	return c.tracer
}

// Shutdown gracefully shuts down the OpenTelemetry client.
func (c *Client) Shutdown(ctx context.Context) error {
	if c.config.Disabled || c.tracerProvider == nil {
		return nil
	}

	return c.tracerProvider.Shutdown(ctx)
}
