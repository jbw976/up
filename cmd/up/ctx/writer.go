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

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/upbound/up/internal/kube"
)

type printWriter struct{}

var _ kube.ContextWriter = &printWriter{}

// Write implements kubeContextWriter.Write.
func (p *printWriter) Write(config *clientcmdapi.Config) error {
	b, err := clientcmd.Write(*config)
	if err != nil {
		return err
	}

	fmt.Print(string(b)) //nolint:forbidigo // The printWriter is allowed to print.
	return nil
}
