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
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	upboundv1alpha1 "github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/spaces"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/version"
)

var (
	//nolint:gochecknoglobals // We'd make these consts if we could.
	upboundBrandColor = lipgloss.AdaptiveColor{Light: "#5e3ba5", Dark: "#af7efd"}
	//nolint:gochecknoglobals // We'd make these consts if we could.
	neutralColor = lipgloss.AdaptiveColor{Light: "#4e5165", Dark: "#9a9ca7"}
	//nolint:gochecknoglobals // We'd make these consts if we could.
	dimColor = lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"}
)

//nolint:gochecknoglobals // We'd make these consts if we could.
var upboundRootStyle = lipgloss.NewStyle().Foreground(upboundBrandColor)

// TODO(adamwg): All the nav states here should probably be unexported. Today we
// do use them a bit outside of the ctx command, but that's a bit of a smell -
// those bits should probably be factored out into an internal package. For now
// we have a bunch of nolints because we mix exported and unexported types.

// NavigationState is a model state that provides a list of items for a navigation node.
type NavigationState interface {
	Items(ctx context.Context, upCtx *upbound.Context, navCtx *navContext) ([]list.Item, error)
	Breadcrumbs() Breadcrumbs
}

// Breadcrumbs represents the path through the tree of contexts for a given
// navigation state.
type Breadcrumbs []string

// String returns the canonical slash-separated string representation of the
// breadcrumbs.
func (b Breadcrumbs) String() string {
	return strings.Join(b, "/")
}

// styledString returns a pretty string version of the breadcrumbs.
func (b Breadcrumbs) styledString() string {
	pathInactiveSegmentStyle := lipgloss.NewStyle().Foreground(neutralColor)
	pathSegmentStyle := lipgloss.NewStyle()

	switch len(b) {
	case 0:
		return ""
	case 1:
		return pathSegmentStyle.Render(b[0])
	default:
		inactive := strings.Join(b[:len(b)-1], "/")
		inactive += "/"
		return pathInactiveSegmentStyle.Render(inactive) + pathSegmentStyle.Render(b[len(b)-1])
	}
}

// Accepting is a model state that provides a method to accept a navigation node.
type Accepting interface {
	NavigationState
	Accept(navCtx *navContext) (string, error)
}

// Back is a model state that provides a method to go back to the parent navigation node.
type Back interface {
	NavigationState
	Back(m model) (model, error)
	BackLabel() string
}

// rootState returns the root state for the active profile.
func rootState(ctx context.Context, upCtx *upbound.Context) (NavigationState, error) {
	switch upCtx.Profile.Type {
	case profile.TypeCloud:
		return &Organization{
			Name: upCtx.Organization,
		}, nil

	case profile.TypeDisconnected:
		return disconnectedSpaceFromKubeconfig(ctx, *upCtx.Profile.SpaceKubeconfig)

	default:
		return nil, errors.New("unknown profile type")
	}
}

func disconnectedSpaceFromKubeconfig(ctx context.Context, kubeconfig clientcmdapi.Config) (*DisconnectedSpace, error) {
	var cfg clientcmdapi.Config
	kubeconfig.DeepCopyInto(&cfg)
	if err := clientcmdapi.MinifyConfig(&cfg); err != nil {
		return nil, err
	}

	rest, err := clientcmd.NewDefaultClientConfig(cfg, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, err
	}

	cl, err := corev1client.NewForConfig(rest)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ingressHost, ingressCA, err := kube.GetIngressHost(reqCtx, cl)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &DisconnectedSpace{
		BaseKubeconfig: &cfg,
		Ingress: spaces.SpaceIngress{
			Host:   ingressHost,
			CAData: ingressCA,
		},
	}, nil
}

// Organization is the nav state containing an organization's spaces.
type Organization struct {
	Name string
}

// Items returns items for an organization nav state.
func (o *Organization) Items(ctx context.Context, upCtx *upbound.Context, navCtx *navContext) ([]list.Item, error) { //nolint:gocognit // TODO: refactor.
	cloudCfg, err := upCtx.BuildControllerClientConfig()
	if err != nil {
		return nil, err
	}

	cloudClient, err := client.New(cloudCfg, client.Options{})
	if err != nil {
		return nil, err
	}

	var l upboundv1alpha1.SpaceList
	err = cloudClient.List(ctx, &l, &client.ListOptions{Namespace: o.Name})
	if err != nil {
		return nil, err
	}

	authInfo, err := getOrgScopedAuthInfo(upCtx, o.Name)
	if err != nil {
		return nil, err
	}

	// Find ingresses for up to 20 Spaces in parallel to construct items for the
	// list.
	var wg sync.WaitGroup
	var mu sync.Mutex
	items := make([]list.Item, 0)
	unselectableItems := make([]list.Item, 0)
	ch := make(chan upboundv1alpha1.Space, len(l.Items))
	for range min(20, len(l.Items)) {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for space := range ch {
				if mode, ok := space.ObjectMeta.Labels[upboundv1alpha1.SpaceModeLabelKey]; ok {
					if mode == string(upboundv1alpha1.ModeLegacy) {
						continue
					}
				}

				if space.Labels[upboundv1alpha1.SpaceInaccessibleLabelKey] == "true" {
					mu.Lock()
					unselectableItems = append(unselectableItems, item{
						text:          space.GetObjectMeta().GetName() + " (requires tier upgrade)",
						kind:          "space",
						notSelectable: true,
						matchingTerms: []string{space.GetObjectMeta().GetName()},
					})
					mu.Unlock()
					continue
				}

				if space.Status.ConnectionDetails.Status == upboundv1alpha1.ConnectionStatusUnreachable {
					mu.Lock()
					unselectableItems = append(unselectableItems, item{
						text:          space.GetObjectMeta().GetName() + " (unreachable)",
						kind:          "space",
						notSelectable: true,
						matchingTerms: []string{space.GetObjectMeta().GetName()},
					})
					mu.Unlock()
					continue
				}

				ingress, err := navCtx.ingressReader.Get(ctx, space)
				if err != nil {
					mu.Lock()
					if errors.Is(err, spaces.ErrSpaceConnection) {
						unselectableItems = append(unselectableItems, item{
							text:          space.GetObjectMeta().GetName() + " (unreachable)",
							kind:          "space",
							notSelectable: true,
							matchingTerms: []string{space.GetObjectMeta().GetName()},
						})
					} else {
						unselectableItems = append(unselectableItems, item{
							text:          fmt.Sprintf("%s (error: %v)", space.GetObjectMeta().GetName(), err),
							kind:          "space",
							notSelectable: true,
							matchingTerms: []string{space.GetObjectMeta().GetName()},
						})
					}
					mu.Unlock()
					continue
				}

				mu.Lock()
				items = append(items, item{text: space.GetObjectMeta().GetName(), kind: "space", onEnter: func(m model) (model, error) {
					m.state = &CloudSpace{
						Org:      *o,
						name:     space.GetObjectMeta().GetName(),
						Ingress:  *ingress,
						AuthInfo: authInfo,
					}
					return m, nil
				}})
				mu.Unlock()
			}
		}()
	}
	for _, space := range l.Items {
		ch <- space
	}
	close(ch)
	wg.Wait()

	slices.SortFunc(items, itemSortFunc)
	slices.SortFunc(unselectableItems, itemSortFunc)

	return append(items, unselectableItems...), nil
}

// Breadcrumbs returns breadcrumbs for an organization nav state.
func (o *Organization) Breadcrumbs() Breadcrumbs {
	return []string{o.Name}
}

func itemSortFunc(a, b list.Item) int {
	aitem, aok := a.(item)
	bitem, bok := b.(item)

	// If either item is not our item type we can't compare them, so treat them
	// as equal.
	if !aok || !bok {
		return 0
	}

	return strings.Compare(aitem.text, bitem.text)
}

// Space abstracts over specific kinds of space contexts.
type Space interface {
	NavigationState
	Accepting

	Name() string
	BuildKubeconfig(resource types.NamespacedName) (clientcmd.ClientConfig, error)
	getClient() (client.Client, error)
}

func listGroupsInSpace(ctx context.Context, s Space) ([]*Group, error) {
	cl, err := s.getClient()
	if err != nil {
		return nil, err
	}

	nss := &corev1.NamespaceList{}
	if err := cl.List(ctx, nss, client.MatchingLabels(map[string]string{spacesv1beta1.ControlPlaneGroupLabelKey: "true"})); err != nil {
		return nil, err
	}

	groups := make([]*Group, 0, len(nss.Items))
	for _, ns := range nss.Items {
		groups = append(groups, &Group{Space: s, Name: ns.Name})
	}

	return groups, nil
}

// CloudSpace provides the navigation node for a connected or cloud space.
type CloudSpace struct {
	Org  Organization
	name string

	Ingress  spaces.SpaceIngress
	AuthInfo *clientcmdapi.AuthInfo
}

// Name returns the space's name.
func (s *CloudSpace) Name() string {
	return s.name
}

// Items returns items for a space nav state.
func (s *CloudSpace) Items(ctx context.Context, _ *upbound.Context, _ *navContext) ([]list.Item, error) {
	groups, err := listGroupsInSpace(ctx, s)
	if err != nil {
		return nil, err
	}

	items := make([]list.Item, 0, len(groups)+3)
	items = append(items, item{text: "..", kind: s.BackLabel(), onEnter: s.Back, back: true})
	for _, group := range groups {
		items = append(items, item{text: group.Name, kind: "group", onEnter: func(m model) (model, error) {
			m.state = group
			return m, nil
		}})
	}

	if len(groups) == 0 {
		items = append(items, item{text: "No groups found", notSelectable: true})
	}

	items = append(items, item{text: fmt.Sprintf("Switch context to %q", s.Name()), onEnter: func(m model) (model, error) {
		msg, err := s.Accept(m.navContext)
		if err != nil {
			return m, err
		}
		return m.WithTermination(msg, nil), nil
	}})

	return items, nil
}

// Back returns the parent of a space nav state.
func (s *CloudSpace) Back(m model) (model, error) { //nolint:revive // See todo at top of file.
	m.state = &s.Org
	return m, nil
}

// BackLabel returns the label for the back item of a space nav state.
func (s *CloudSpace) BackLabel() string {
	return "spaces"
}

// Breadcrumbs returns breadcrumbs for a space nav state.
func (s *CloudSpace) Breadcrumbs() Breadcrumbs {
	return append(s.Org.Breadcrumbs(), s.name)
}

// getClient returns a kube client pointed at the current space.
func (s *CloudSpace) getClient() (client.Client, error) {
	conf, err := s.BuildKubeconfig(types.NamespacedName{})
	if err != nil {
		return nil, err
	}

	rest, err := conf.ClientConfig()
	if err != nil {
		return nil, err
	}
	rest.UserAgent = version.UserAgent()

	return client.New(rest, client.Options{})
}

// BuildKubeconfig creates a new kubeconfig hardcoded to match the provided spaces
// access configuration and pointed directly at the resource. If the resource
// only specifies a namespace, then the client will point at the space and the
// context will be set at the group. If the resource specifies both a namespace
// and a name, then the client will point directly at the control plane ingress
// and set the namespace to "default".
func (s *CloudSpace) BuildKubeconfig(resource types.NamespacedName) (clientcmd.ClientConfig, error) {
	// reference name for all context, cluster and authinfo for in-memory
	// kubeconfig
	ref := "upbound"

	config := clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		CurrentContext: ref,
		Clusters:       make(map[string]*clientcmdapi.Cluster),
		Contexts:       make(map[string]*clientcmdapi.Context),
		AuthInfos:      make(map[string]*clientcmdapi.AuthInfo),
	}

	// Build a new context with a new cluster that points to the space's
	// ingress.
	refContext := &clientcmdapi.Context{
		Extensions: make(map[string]runtime.Object),
		Cluster:    ref,
	}

	if s.Ingress.Host == "" {
		return nil, errors.New("missing ingress address for context")
	}
	if len(s.Ingress.CAData) == 0 {
		return nil, errors.New("missing ingress CA for context")
	}

	config.Clusters[ref] = &clientcmdapi.Cluster{
		Server:                   profile.ToSpacesK8sURL(s.Ingress.Host, resource),
		CertificateAuthorityData: s.Ingress.CAData,
	}

	config.AuthInfos[ref] = s.AuthInfo
	refContext.AuthInfo = ref

	if resource.Name == "" {
		// point at the relevant namespace in the space hub
		refContext.Namespace = resource.Namespace
	} else {
		// since we are pointing at an individual control plane, point at the
		// "default" namespace inside it
		refContext.Namespace = "default"
	}

	refContext.Extensions[upbound.ContextExtensionKeySpace] = upbound.NewCloudV1Alpha1SpaceExtension(s.Org.Name, s.name)

	config.Contexts[ref] = refContext
	return clientcmd.NewDefaultClientConfig(config, &clientcmd.ConfigOverrides{}), nil
}

// DisconnectedSpace provides the navigation node for a disconnected space.
type DisconnectedSpace struct {
	BaseKubeconfig *clientcmdapi.Config
	Ingress        spaces.SpaceIngress
}

// Name returns the space's name.
func (s *DisconnectedSpace) Name() string {
	return s.BaseKubeconfig.CurrentContext
}

// Items returns items for a space nav state.
func (s *DisconnectedSpace) Items(ctx context.Context, _ *upbound.Context, _ *navContext) ([]list.Item, error) {
	groups, err := listGroupsInSpace(ctx, s)
	if err != nil {
		return nil, err
	}

	items := make([]list.Item, 0, len(groups)+1)
	for _, group := range groups {
		items = append(items, item{text: group.Name, kind: "group", onEnter: func(m model) (model, error) {
			m.state = group
			return m, nil
		}})
	}

	if len(groups) == 0 {
		items = append(items, item{text: "No groups found", notSelectable: true})
	}

	items = append(items, item{text: fmt.Sprintf("Switch context to %q", s.Name()), onEnter: func(m model) (model, error) {
		msg, err := s.Accept(m.navContext)
		if err != nil {
			return m, err
		}
		return m.WithTermination(msg, nil), nil
	}})

	return items, nil
}

// Breadcrumbs returns breadcrumbs for a space nav state.
func (s *DisconnectedSpace) Breadcrumbs() Breadcrumbs {
	return []string{"disconnected", s.Name()}
}

// getClient returns a kube client pointed at the current space.
func (s *DisconnectedSpace) getClient() (client.Client, error) {
	conf, err := s.BuildKubeconfig(types.NamespacedName{})
	if err != nil {
		return nil, err
	}

	rest, err := conf.ClientConfig()
	if err != nil {
		return nil, err
	}
	rest.UserAgent = version.UserAgent()

	return client.New(rest, client.Options{})
}

// BuildKubeconfig creates a new kubeconfig hardcoded to match the provided spaces
// access configuration and pointed directly at the resource. If the resource
// only specifies a namespace, then the client will point at the space and the
// context will be set at the group. If the resource specifies both a namespace
// and a name, then the client will point directly at the control plane ingress
// and set the namespace to "default".
func (s *DisconnectedSpace) BuildKubeconfig(resource types.NamespacedName) (clientcmd.ClientConfig, error) {
	// reference name for all context, cluster and authinfo for in-memory
	// kubeconfig
	ref := "upbound"

	base := s.BaseKubeconfig
	baseCtx := base.Contexts[base.CurrentContext]

	config := clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		CurrentContext: ref,
		Clusters:       make(map[string]*clientcmdapi.Cluster),
		Contexts:       make(map[string]*clientcmdapi.Context),
		AuthInfos:      make(map[string]*clientcmdapi.AuthInfo),
	}

	// Build a new context with a new cluster that points to the space's
	// ingress.
	refContext := &clientcmdapi.Context{
		Extensions: make(map[string]runtime.Object),
		Cluster:    ref,
	}

	if s.Ingress.Host == "" {
		return nil, errors.New("missing ingress address for context")
	}
	if len(s.Ingress.CAData) == 0 {
		return nil, errors.New("missing ingress CA for context")
	}

	config.Clusters[ref] = &clientcmdapi.Cluster{
		Server:                   profile.ToSpacesK8sURL(s.Ingress.Host, resource),
		CertificateAuthorityData: s.Ingress.CAData,
	}

	config.AuthInfos[ref] = base.AuthInfos[baseCtx.AuthInfo]
	refContext.AuthInfo = ref

	if resource.Name == "" {
		// point at the relevant namespace in the space hub
		refContext.Namespace = resource.Namespace
	} else {
		// since we are pointing at an individual control plane, point at the
		// "default" namespace inside it
		refContext.Namespace = "default"
	}

	refContext.Extensions[upbound.ContextExtensionKeySpace] = upbound.NewDisconnectedV1Alpha1SpaceExtension(base.CurrentContext)

	config.Contexts[ref] = refContext
	return clientcmd.NewDefaultClientConfig(config, &clientcmd.ConfigOverrides{}), nil
}

// Group provides the navigation node for a concrete group aka namespace.
type Group struct {
	Space Space
	Name  string
}

var (
	_ Accepting = &Group{}
	_ Back      = &Group{}
)

// Items returns the items for a group nav state.
func (g *Group) Items(ctx context.Context, _ *upbound.Context, _ *navContext) ([]list.Item, error) {
	cl, err := g.Space.getClient()
	if err != nil {
		return nil, err
	}

	ctps := &spacesv1beta1.ControlPlaneList{}
	if err := cl.List(ctx, ctps, client.InNamespace(g.Name)); err != nil {
		return nil, err
	}

	items := make([]list.Item, 0, len(ctps.Items)+3)
	items = append(items, item{text: "..", kind: g.BackLabel(), onEnter: g.Back, back: true})

	for _, ctp := range ctps.Items {
		items = append(items, item{text: ctp.Name, kind: "controlplane", onEnter: func(m model) (model, error) {
			m.state = &ControlPlane{Group: *g, Name: ctp.Name}
			return m, nil
		}})
	}

	if len(ctps.Items) == 0 {
		items = append(items, item{text: fmt.Sprintf("No control planes found in group %q", g.Name), notSelectable: true})
	}

	items = append(items, item{text: fmt.Sprintf("Switch context to %q", fmt.Sprintf("%s/%s", g.Space.Name(), g.Name)), onEnter: func(m model) (model, error) {
		msg, err := g.Accept(m.navContext)
		if err != nil {
			return m, err
		}
		return m.WithTermination(msg, nil), nil
	}})

	return items, nil
}

// Breadcrumbs returns breadcrumbs for a group nav state.
func (g *Group) Breadcrumbs() Breadcrumbs {
	return append(g.Space.Breadcrumbs(), g.Name)
}

// Back returns the parent of a group nav state.
func (g *Group) Back(m model) (model, error) { //nolint:revive // See todo at top of file.
	m.state = g.Space
	return m, nil
}

// BackLabel returns the label for the back item of a group nav state.
func (g *Group) BackLabel() string {
	return "groups"
}

// ControlPlane provides the navigation node for a concrete controlplane.
type ControlPlane struct {
	Group Group
	Name  string
}

var (
	_ Accepting = &ControlPlane{}
	_ Back      = &ControlPlane{}
)

// Items returns items for a control plane nav state.
func (ctp *ControlPlane) Items(_ context.Context, _ *upbound.Context, _ *navContext) ([]list.Item, error) {
	return []list.Item{
		item{text: "..", kind: ctp.BackLabel(), onEnter: ctp.Back, back: true},
		item{text: fmt.Sprintf("Connect to %q and quit", ctp.NamespacedName().Name), onEnter: keyFunc(func(m model) (model, error) {
			msg, err := ctp.Accept(m.navContext)
			if err != nil {
				return m, err
			}
			return m.WithTermination(msg, nil), nil
		})},
	}, nil
}

// Breadcrumbs returns breadcrumbs for a control plane nav state.
func (ctp *ControlPlane) Breadcrumbs() Breadcrumbs {
	return append(ctp.Group.Breadcrumbs(), ctp.Name)
}

// Back returns the parent of a control plane nav state.
func (ctp *ControlPlane) Back(m model) (model, error) { //nolint:revive // See todo at top of file.
	m.state = &ctp.Group
	return m, nil
}

// BackLabel returns the label for the back item of a control plane nav state.
func (ctp *ControlPlane) BackLabel() string {
	return "controlplanes"
}

// NamespacedName returns a Kubernetes name within a space for the control plane
// referred to by a control plane nav state.
func (ctp *ControlPlane) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: ctp.Name, Namespace: ctp.Group.Name}
}

func getOrgScopedAuthInfo(upCtx *upbound.Context, orgName string) (*clientcmdapi.AuthInfo, error) {
	// find the current executable path
	cmd, err := os.Executable()
	if err != nil {
		return nil, err
	}

	// if the current executable was the same `up` that is found in PATH
	path, err := exec.LookPath("up")
	if err == nil && path == cmd {
		cmd = "up"
	}

	return &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			APIVersion: "client.authentication.k8s.io/v1",
			Command:    cmd,
			Args:       []string{"organization", "token"},
			Env: []clientcmdapi.ExecEnvVar{
				{
					Name:  "ORGANIZATION",
					Value: orgName,
				},
				{
					Name:  "UP_PROFILE",
					Value: upCtx.ProfileName,
				},
			},
			InteractiveMode: clientcmdapi.IfAvailableExecInteractiveMode,
		},
	}, nil
}
