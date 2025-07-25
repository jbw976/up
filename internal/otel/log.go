// Copyright 2025 Upbound Inc.
// All rights reserved

package otel

import (
	"fmt"
	"log"
	"strings"
)

// shouldSuppressMessage checks if a log message should be suppressed.
func (l *quietGRPCLogger) shouldSuppressMessage(msg string) bool {
	suppressPatterns := []string{
		"connection error",
		"connection refused",
		"createTransport failed",
		"context deadline exceeded",
		"dial tcp",
		"transport: Error while dialing",
		"SubChannel",
		"addrConn.createTransport",
	}

	for _, pattern := range suppressPatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}
	return false
}

// quietGRPCLogger is a custom gRPC logger that suppresses connection errors.
type quietGRPCLogger struct{}

func (l *quietGRPCLogger) Info(_ ...interface{}) {
	// No info. We don't want to log anything.
}

func (l *quietGRPCLogger) Infoln(args ...interface{}) {
	l.Info(args...)
}

func (l *quietGRPCLogger) Infof(_ string, _ ...interface{}) {
	// No info. We don't want to log anything.
}

func (l *quietGRPCLogger) Warning(_ ...interface{}) {
	// No warnings. We don't want to log anything.
}

func (l *quietGRPCLogger) Warningln(args ...interface{}) {
	l.Warning(args...)
}

func (l *quietGRPCLogger) Warningf(_ string, _ ...interface{}) {
	// No warnings. We don't want to log anything.
}

func (l *quietGRPCLogger) Error(args ...interface{}) {
	// Still log errors but suppress connection-related ones
	msg := fmt.Sprint(args...)
	if !l.shouldSuppressMessage(msg) {
		log.Println("telemetry error:", msg)
	}
}

func (l *quietGRPCLogger) Errorln(args ...interface{}) {
	l.Error(args...)
}

func (l *quietGRPCLogger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if !l.shouldSuppressMessage(msg) {
		l.Error(msg)
	}
}

func (l *quietGRPCLogger) Fatal(args ...interface{}) {
	log.Fatal("telemetry fatal:", fmt.Sprint(args...))
}

func (l *quietGRPCLogger) Fatalln(args ...interface{}) {
	l.Fatal(args...)
}

func (l *quietGRPCLogger) Fatalf(format string, args ...interface{}) {
	log.Fatalf("telemetry fatal: "+format, args...)
}

func (l *quietGRPCLogger) V(level int) bool {
	// Only log important messages (level 0)
	return level == 0
}
