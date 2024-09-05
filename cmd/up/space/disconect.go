// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package space

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/google/uuid"
	"github.com/pterm/pterm"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	upboundv1alpha1 "github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
	sdkerrs "github.com/upbound/up-sdk-go/errors"
	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up-sdk-go/service/spaces"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

type disconnectCmd struct {
	Upbound upbound.Flags     `embed:""`
	Kube    upbound.KubeFlags `embed:""`

	Space string `arg:"" optional:"" help:"Name of the Upbound Space. If name is not a supplied, it will be determined from the connection info in the space."`
}

func (c *disconnectCmd) AfterApply(kongCtx *kong.Context) error {
	registryURL, err := url.Parse(agentRegistry)
	if err != nil {
		return err
	}

	needsKube := true
	if err := c.Kube.AfterApply(); err != nil {
		if c.Space == "" {
			return errors.Wrap(err, "failed to get kube config")
		}
		needsKube = false
	}

	// NOTE(tnthornton) we currently only have support for stylized output.
	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	upCtx, err := upbound.NewFromFlags(c.Upbound)
	if err != nil {
		return err
	}
	upCtx.SetupLogging()

	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	ctrlCfg, err := upCtx.BuildControllerClientConfig()
	if err != nil {
		return err
	}

	kongCtx.Bind(upCtx)
	kongCtx.Bind(ctrlCfg)
	kongCtx.Bind(robots.NewClient(cfg))
	kongCtx.Bind(spaces.NewClient(cfg))
	kongCtx.Bind(accounts.NewClient(cfg))
	kongCtx.Bind(organizations.NewClient(cfg))

	// bind nils as k8s client and helm manager
	if !needsKube {
		kongCtx.Bind((*kubernetes.Clientset)(nil))
		kongCtx.Bind((*helm.Installer)(nil))
		return nil
	}
	kubeconfig := c.Kube.GetConfig()

	kClient, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return errors.Wrap(err, "failed to create kube client")
	}
	kongCtx.Bind(kClient)

	with := []helm.InstallerModifierFn{
		helm.WithNamespace(agentNs),
		helm.IsOCI(),
	}

	mgr, err := helm.NewManager(kubeconfig,
		agentChart,
		registryURL,
		with...,
	)
	if err != nil {
		return err
	}
	kongCtx.Bind(mgr)

	return nil
}

// Run executes the disconnect command.
func (c *disconnectCmd) Run(ctx context.Context, upCtx *upbound.Context, ac *accounts.Client, oc *organizations.Client, kClient *kubernetes.Clientset, mgr *helm.Installer, rc *robots.Client, rest *rest.Config) (rErr error) {
	msg := "Disconnecting Space from Upbound Console..."
	if c.Space != "" {
		msg = fmt.Sprintf("Disconnecting Space %q from Upbound Console...", c.Space)
	}
	disconnectSpinner, err := upterm.CheckmarkSuccessSpinner.Start(msg)
	if err != nil {
		return err
	}
	defer func() {
		if rErr != nil {
			disconnectSpinner.Fail(rErr)
		}
	}()
	sc, err := client.New(rest, client.Options{})
	if err != nil {
		return err
	}

	if err := c.disconnectSpace(ctx, disconnectSpinner, upCtx, ac, oc, kClient, mgr, rc, sc); err != nil {
		return err
	}
	msg = "Space has been successfully disconnected from Upbound Console"
	if c.Space != "" {
		msg = fmt.Sprintf(`Space "%s/%s" has been successfully disconnected from Upbound Console`, upCtx.Account, c.Space)
	}
	disconnectSpinner.Success(msg)
	return nil
}

func (c *disconnectCmd) disconnectSpace(ctx context.Context, disconnectSpinner *pterm.SpinnerPrinter, upCtx *upbound.Context, ac *accounts.Client, oc *organizations.Client, kClient *kubernetes.Clientset, mgr *helm.Installer, rc *robots.Client, sc client.Client) error {
	if kClient == nil {
		if err := warnAndConfirmWithSpinner(disconnectSpinner,
			`Not connected to a Space cluster, would you like to only remove the Space "%s/%s" from the Upbound Console?`+"\n\n"+
				"  If the other Space cluster still exists, the Upbound agent will be left running and you will need to delete it manually.\n",
			upCtx.Account, c.Space,
		); err != nil {
			return err
		}
		return disconnectSpace(ctx, disconnectSpinner, ac, oc, rc, sc, upCtx.Account, c.Space)
	}
	if err := c.deleteResources(ctx, kClient, disconnectSpinner, upCtx, ac, oc, rc, sc); err != nil {
		return err
	}
	disconnectSpinner.UpdateText("Uninstalling connect agent...")
	disconnectSpinner.InfoPrinter.Printfln(`Uninstalling Chart "%s/%s"`, agentNs, agentChart)
	if err := mgr.Uninstall(); err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		return errors.Wrapf(err, `failed to uninstall Chart "%s/%s"`, agentNs, agentChart)
	}
	disconnectSpinner.InfoPrinter.Printfln(`Chart "%s/%s" uninstalled`, agentNs, agentChart)
	if err := deleteTokenSecret(ctx, disconnectSpinner.InfoPrinter, kClient, agentNs, agentSecret); err != nil {
		return err
	}
	return nil
}

func disconnectSpace(ctx context.Context, progressSpinner *pterm.SpinnerPrinter, ac *accounts.Client, oc *organizations.Client, rc *robots.Client, sc client.Client, namespace, name string) error {
	progressSpinner.UpdateText(fmt.Sprintf(`Disconnecting Space "%s/%s" from Upbound Console...`, namespace, name))
	a, err := upbound.GetOrganization(ctx, ac, namespace)
	if err != nil {
		return err
	}
	if err := deleteSpaceRobot(ctx, progressSpinner.InfoPrinter, oc, rc, a, name); err != nil {
		return err
	}
	if err := deleteSpace(ctx, progressSpinner.InfoPrinter, sc, namespace, name); err != nil {
		return err
	}
	return nil
}

func deleteSpace(ctx context.Context, p pterm.TextPrinter, sc client.Client, namespace, name string) error {
	p.Printfln(`Deleting Space "%s/%s"`, namespace, name)
	if err := sc.Delete(ctx, &upboundv1alpha1.Space{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}); err == nil && !kerrors.IsNotFound(err) {
		return errors.Wrapf(err, `failed to delete Space "%s/%s"`, namespace, name)
	}
	p.Printfln(`Space "%s/%s" deleted`, namespace, name)
	return nil
}

func deleteSpaceRobot(ctx context.Context, p pterm.TextPrinter, oc *organizations.Client, rc *robots.Client, ar *accounts.AccountResponse, space string) error {
	p.Printfln(`Looking for Robot "%s/%s"`, ar.Organization.Name, space)
	rr, err := oc.ListRobots(ctx, ar.Organization.ID)
	if err != nil {
		return errors.Wrap(err, "failed to list Robots")
	}
	for _, r := range rr {
		if r.Name != space {
			continue
		}
		p.Printfln(`Deleting Robot "%s/%s"`, ar.Organization.Name, space)
		if err := rc.Delete(ctx, r.ID); err != nil && !sdkerrs.IsNotFound(err) {
			return errors.Wrapf(err, `failed to delete Robot "%s/%s"`, ar.Organization.Name, space)
		}
		p.Printfln(`Robot "%s/%s" deleted`, ar.Organization.Name, space)
		return nil
	}
	p.Printfln(`Robot "%s/%s" not found`, ar.Organization.Name, space)
	return nil
}

func (c *disconnectCmd) deleteResources(ctx context.Context, kClient *kubernetes.Clientset, disconnectSpinner *pterm.SpinnerPrinter, upCtx *upbound.Context, ac *accounts.Client, oc *organizations.Client, rc *robots.Client, sc client.Client) error {
	cm, err := getConnectConfigmap(ctx, kClient, agentNs, connConfMap)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			return errors.Wrapf(err, `failed to get ConfigMap "%s/%s"`, agentNs, agentSecret)
		}
		disconnectSpinner.InfoPrinter.Printfln(`ConfigMap "%s/%s" not found`, agentNs, agentSecret)
		if c.Space == "" {
			return errors.New("failed to find Space to disconnect from Upbound Console")
		}
		if err := warnAndConfirmWithSpinner(disconnectSpinner,
			`We're unable to confirm if the Space "%s/%s" is currently connected to Upbound Console. Would you like to delete it anyway?`+"\n\n"+
				"  If the other Space cluster still exists, the Upbound agent will be left running and you will need to delete it manually.\n",
			upCtx.Account, c.Space,
		); err != nil {
			return err
		}
		return disconnectSpace(ctx, disconnectSpinner, ac, oc, rc, sc, upCtx.Account, c.Space)
	}

	disconnectSpinner.InfoPrinter.Printfln(`ConfigMap "%s/%s" found`, agentNs, agentSecret)
	disconnectSpinner.UpdateText("Deleting Space in the Upbound Console...")
	if err := c.deleteGeneratedSpace(ctx, disconnectSpinner, kClient, upCtx, sc, &cm); err != nil {
		return err
	}
	if err := c.deleteAgentRobot(ctx, disconnectSpinner.InfoPrinter, kClient, rc, &cm); err != nil {
		return err
	}
	if err := deleteConnectConfigmap(ctx, disconnectSpinner.InfoPrinter, kClient, agentNs, connConfMap); err != nil {
		return err
	}
	return nil
}

func (c *disconnectCmd) deleteAgentRobot(ctx context.Context, p pterm.TextPrinter, kClient *kubernetes.Clientset, rc *robots.Client, cmr **corev1.ConfigMap) error {
	cm := *cmr
	v, ok := cm.Data[keyRobotID]
	if !ok {
		return nil
	}
	rid, err := uuid.Parse(v)
	if err != nil {
		return errors.Wrapf(err, "invalid robot id %q", v)
	}
	p.Printfln("Deleting Robot %q", rid)
	if err := rc.Delete(ctx, rid); err != nil && !sdkerrs.IsNotFound(err) {
		return errors.Wrapf(err, "failed to delete Robot %q", rid)
	}
	p.Printfln("Robot %q deleted", rid)
	delete(cm.Data, keyRobotID)
	delete(cm.Data, keyTokenID)
	cm, err = kClient.CoreV1().ConfigMaps(agentNs).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrapf(err, `failed to update ConfigMap "%s/%s"`, agentNs, connConfMap)
	}
	*cmr = cm
	return nil
}

func (c *disconnectCmd) deleteGeneratedSpace(ctx context.Context, disconnectSpinner *pterm.SpinnerPrinter, kClient *kubernetes.Clientset, upCtx *upbound.Context, sc client.Client, cmr **corev1.ConfigMap) error { //nolint:gocyclo
	cm := *cmr
	v, ok := cm.Data[keySpace]
	if !ok {
		return nil
	}
	parts := strings.Split(v, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid space %q", v)
	}
	ns, name := parts[0], parts[1]
	disconnectSpinner.InfoPrinter.Printfln(`Space "%s/%s" is currently connected`, ns, name)
	if (c.Space != "" && c.Space != name) || ns != upCtx.Account {
		return fmt.Errorf(`cannot disconnect Space "%s/%s", currently connected to Space "%s/%s"`, upCtx.Account, c.Space, ns, name)
	}
	disconnectSpinner.UpdateText(fmt.Sprintf(`Deleting Space "%s/%s" in the Upbound Console...`, ns, name))
	c.Space = name
	disconnectSpinner.InfoPrinter.Printfln(`Deleting Space "%s/%s"`, ns, name)

	if err := sc.Delete(ctx, &upboundv1alpha1.Space{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}); err != nil && !kerrors.IsNotFound(err) {
		return errors.Wrapf(err, `failed to delete Space "%s/%s"`, ns, name)
	}
	disconnectSpinner.InfoPrinter.Printfln(`Space "%s/%s" deleted`, ns, name)
	delete(cm.Data, keySpace)
	cm, err := kClient.CoreV1().ConfigMaps(agentNs).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrapf(err, `failed to update ConfigMap "%s/%s"`, agentNs, connConfMap)
	}
	*cmr = cm
	return nil
}
