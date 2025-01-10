// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ctx

import (
	"fmt"
	"io/fs"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/upbound"
)

type kubeContextWriter interface {
	Write(config *clientcmdapi.Config) error
}

type printWriter struct{}

var _ kubeContextWriter = &printWriter{}

// Write implements kubeContextWriter.Write.
func (p *printWriter) Write(config *clientcmdapi.Config) error {
	b, err := clientcmd.Write(*config)
	if err != nil {
		return err
	}

	fmt.Print(string(b)) //nolint:forbidigo // The printWriter is allowed to print.
	return nil
}

// fileWriter writes kubeconfigs by merging the active context of the passed
// kubeconfig into an existing kubeconfig then writing it to a file. If a
// context exists with the merged context's name, it will be renamed with the
// suffix "-previous". If the "-previous" context also exists it will be
// overwritten.
type fileWriter struct {
	upCtx *upbound.Context
	// fileOverride is the path to the existing kubeconfig to update. If empty
	// the default loading rules are used.
	fileOverride string
	// kubeContext overrides the name of the context to be merged into the
	// kubeconfig. If empty the merged context retains its name.
	kubeContext string

	// writeLastContext is called with the name of the previously active context
	// from the existing kubeconfig after the merged kubeconfig is written.
	writeLastContext func(string) error
	// verify is called to validate that the merged kubeconfig is valid before
	// writing it.
	verify func(cfg *clientcmdapi.Config) error
	// modifyConfig is called to write the merged kubeconfig.
	modifyConfig func(configAccess clientcmd.ConfigAccess, newConfig clientcmdapi.Config, relativizePaths bool) error
}

var _ kubeContextWriter = &fileWriter{}

// Write implements kubeContextWriter.Write.
func (f *fileWriter) Write(config *clientcmdapi.Config) error {
	outConfig, err := f.loadOutputKubeconfig()
	if err != nil {
		return err
	}

	updatedConf, prevContext, err := f.mergeConfigs(outConfig, config)
	if err != nil {
		return err
	}

	if err := f.verify(updatedConf); err != nil {
		return err
	}

	pathOptions := clientcmd.NewDefaultPathOptions()
	if f.fileOverride != "" {
		pathOptions = &clientcmd.PathOptions{
			GlobalFile:   f.fileOverride,
			LoadingRules: &clientcmd.ClientConfigLoadingRules{},
		}
	}

	if err := f.modifyConfig(pathOptions, *updatedConf, false); err != nil {
		return err
	}

	_ = f.writeLastContext(prevContext) // ignore error because now everything has happened already.
	return nil
}

// loadOutputKubeconfig loads the Kubeconfig that will be overwritten by the
// action, either loading it from the file override or defaulting back to the
// current kubeconfig.
func (f *fileWriter) loadOutputKubeconfig() (config *clientcmdapi.Config, err error) {
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

func (f *fileWriter) mergeConfigs(outConfig *clientcmdapi.Config, inConfig *clientcmdapi.Config) (*clientcmdapi.Config, string, error) {
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
		previousContextName = mergeContextName + upboundPreviousContextSuffix
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
