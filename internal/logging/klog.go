// Copyright 2025 Upbound Inc.
// All rights reserved

package logging

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
)

// SetKlogLogger sets log as the logger backend of klog, with debugLevel+3 as the
// klog verbosity level.
func SetKlogLogger(debugLevel int, log logr.Logger) {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	klog.InitFlags(fs)
	_ = fs.Parse([]string{fmt.Sprintf("--v=%d", debugLevel+3)})

	klogr := logr.New(&klogFilter{LogSink: log.GetSink()})
	klog.SetLogger(klogr)
}

type klogFilter struct {
	logr.LogSink
}

func (l *klogFilter) Info(level int, msg string, keysAndValues ...interface{}) {
	l.LogSink.Info(klogToLogrLevel(level), msg, keysAndValues...)
}

func (l *klogFilter) Enabled(_ int) bool {
	return true
}

func klogToLogrLevel(klogLvl int) int {
	if klogLvl > 3 {
		return 1
	}
	return 0
}

func (l *klogFilter) WithCallDepth(depth int) logr.LogSink {
	if delegate, ok := l.LogSink.(logr.CallDepthLogSink); ok {
		return &klogFilter{LogSink: delegate.WithCallDepth(depth)}
	}

	return l
}
