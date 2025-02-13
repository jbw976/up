// Copyright 2025 Upbound Inc.
// All rights reserved

package kube

import (
	"io/fs"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/upbound"
)

const (
	// UpboundPreviousContextSuffix is the suffix we attach to a context name to
	// preserve the context's contents when overwriting it.
	UpboundPreviousContextSuffix = "-previous"
)

// ContextWriter writes the current context of a kubeconfig.
type ContextWriter interface {
	Write(config *clientcmdapi.Config) error
}

// FileWriter writes kubeconfigs by merging the active context of the passed
// kubeconfig into an existing kubeconfig then writing it to a file. If a
// context exists with the merged context's name, it will be renamed with the
// suffix "-previous". If the "-previous" context also exists it will be
// overwritten.
type FileWriter struct {
	upCtx *upbound.Context
	// fileOverride is the path to the existing kubeconfig to update. If empty
	// the default loading rules are used.
	fileOverride string
	// kubeContext overrides the name of the context to be merged into the
	// kubeconfig. If empty the merged context retains its name.
	kubeContext string

	// writeLastContextFunc is called with the name of the previously active context
	// from the existing kubeconfig after the merged kubeconfig is written.
	writeLastContextFunc func(string) error
	// verifyFunc is called to validate that the merged kubeconfig is valid before
	// writing it.
	verifyFunc func(cfg *clientcmdapi.Config) error
	// modifyConfigFunc is called to write the merged kubeconfig.
	modifyConfigFunc func(configAccess clientcmd.ConfigAccess, newConfig clientcmdapi.Config, relativizePaths bool) error
}

var _ ContextWriter = &FileWriter{}

// Write implements kubeContextWriter.Write.
func (f *FileWriter) Write(config *clientcmdapi.Config) error {
	outConfig, err := f.loadOutputKubeconfig()
	if err != nil {
		return err
	}

	updatedConf, prevContext, err := f.mergeConfigs(outConfig, config)
	if err != nil {
		return err
	}

	if err := f.verifyFunc(updatedConf); err != nil {
		return err
	}

	pathOptions := clientcmd.NewDefaultPathOptions()
	if f.fileOverride != "" {
		pathOptions = &clientcmd.PathOptions{
			GlobalFile:   f.fileOverride,
			LoadingRules: &clientcmd.ClientConfigLoadingRules{},
		}
	}

	if err := f.modifyConfigFunc(pathOptions, *updatedConf, false); err != nil {
		return err
	}

	_ = f.writeLastContextFunc(prevContext) // ignore error because now everything has happened already.
	return nil
}

// loadOutputKubeconfig loads the Kubeconfig that will be overwritten by the
// action, either loading it from the file override or defaulting back to the
// current kubeconfig.
func (f *FileWriter) loadOutputKubeconfig() (config *clientcmdapi.Config, err error) {
	if f.fileOverride != "" {
		config, err = clientcmd.LoadFromFile(f.fileOverride)
		if errors.Is(err, fs.ErrNotExist) {
			return clientcmdapi.NewConfig(), nil
		} else if err != nil {
			return nil, err
		}
		return config, nil
	}

	raw, err := f.upCtx.Kubecfg.RawConfig()
	if err != nil {
		return nil, err
	}
	return &raw, nil
}

func (f *FileWriter) mergeConfigs(outConfig *clientcmdapi.Config, inConfig *clientcmdapi.Config) (*clientcmdapi.Config, string, error) {
	outConfig = outConfig.DeepCopy()

	// previousContextName is the name of the context containing the previous
	// current context's details. This is the previous current context name
	// unless we're overwriting that context.
	previousContextName := outConfig.CurrentContext
	mergeContextName := f.kubeContext
	if mergeContextName == "" {
		mergeContextName = inConfig.CurrentContext
	}
	if outConfig.CurrentContext == mergeContextName {
		previousContextName = mergeContextName + UpboundPreviousContextSuffix
	}

	// Construct the context that we'll merge into the kubeconfig.
	mergeContext, mergeCluster, mergeAuthInfo, err := copyContext(inConfig, inConfig.CurrentContext)
	if err != nil {
		return nil, "", err
	}
	mergeContext.Cluster = mergeContextName
	mergeContext.AuthInfo = mergeContextName

	// If we're overwriting the current context, construct the "previous"
	// context and add it to the config.
	if previousContextName != outConfig.CurrentContext {
		prevContext, prevCluster, prevAuthInfo, err := copyContext(outConfig, mergeContextName)
		if err != nil {
			return nil, "", err
		}

		prevContext.Cluster = previousContextName
		prevContext.AuthInfo = previousContextName

		outConfig.Contexts[previousContextName] = prevContext
		outConfig.Clusters[previousContextName] = prevCluster
		outConfig.AuthInfos[previousContextName] = prevAuthInfo
	}

	// Add the merge context to the config.
	outConfig.Contexts[mergeContextName] = mergeContext
	outConfig.Clusters[mergeContextName] = mergeCluster
	outConfig.AuthInfos[mergeContextName] = mergeAuthInfo
	outConfig.CurrentContext = mergeContextName

	return outConfig, previousContextName, nil
}

func copyContext(config *clientcmdapi.Config, name string) (*clientcmdapi.Context, *clientcmdapi.Cluster, *clientcmdapi.AuthInfo, error) {
	ctx, ok := config.Contexts[name]
	if !ok {
		return nil, nil, nil, errors.Errorf("context %q not found in kubeconfig", name)
	}

	cluster, ok := config.Clusters[ctx.Cluster]
	if !ok {
		return nil, nil, nil, errors.Errorf("cluster %q not found in kubeconfig", ctx.Cluster)
	}

	authInfo, ok := config.AuthInfos[ctx.AuthInfo]
	if !ok {
		return nil, nil, nil, errors.Errorf("authinfo %q not found in kubeconfig", ctx.AuthInfo)
	}

	return ctx.DeepCopy(), cluster.DeepCopy(), authInfo.DeepCopy(), nil
}

// NewFileWriter returns a new, ready-to-use, file writer. The zero value of the
// file writer is not usable.
func NewFileWriter(upCtx *upbound.Context, fileOverride string, kubeContext string) *FileWriter {
	return &FileWriter{
		upCtx:                upCtx,
		fileOverride:         fileOverride,
		kubeContext:          kubeContext,
		verifyFunc:           VerifyKubeConfig(),
		writeLastContextFunc: WriteLastContext,
		modifyConfigFunc:     clientcmd.ModifyConfig,
	}
}

// NopWriter doesn't actually write a kubeconfig.
type NopWriter struct{}

var _ ContextWriter = &NopWriter{}

// Write implements kubeContextWriter.Write.
func (p *NopWriter) Write(_ *clientcmdapi.Config) error {
	return nil
}
