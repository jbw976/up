// Copyright 2025 Upbound Inc.
// All rights reserved

package ctx

import (
	"context"
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/types"
	kruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/spaces"
	"github.com/upbound/up/internal/upbound"
)

const (
	contextSwitchedFmt = "Switched kubeconfig context to: %s\n"
)

var errParseSpaceContext = errors.New("unable to parse space info from context")

func init() {
	kruntime.Must(spacesv1beta1.AddToScheme(scheme.Scheme))
}

// Cmd is the `up ctx` command.
type Cmd struct {
	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`

	Argument    string `arg:""                                                                                                                       help:".. to move to the parent, '-' for the previous context, '.' for the current context, or any relative path." optional:""`
	Short       bool   `env:"UP_SHORT"                                                                                                               help:"Short output."                                                                                              name:"short"                             short:"s"`
	KubeContext string `default:"upbound"                                                                                                            env:"UP_CONTEXT"                                                                                                  help:"Kubernetes context to operate on." name:"context"`
	File        string `help:"Kubeconfig to modify when saving a new context. Overrides the --kubeconfig flag. Use '-' to write to standard output." short:"f"`
}

// AfterApply processes flags and sets defaults.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}

	if upCtx.ProfileName == "" {
		return errors.New("no profile found; use `up login` or `up profile create` to create one")
	}

	upCtx.SetupLogging()

	kongCtx.Bind(upCtx)

	return nil
}

// Termination is a model state that indicates the command should be terminated,
// optionally with a message and an error.
type Termination struct {
	Err     error
	Message string
}

// navContext contains the helpers and functions used when navigating through
// the up ctx flow.
type navContext struct {
	ingressReader spaces.IngressReader
	contextWriter kube.ContextWriter
}

type model struct {
	windowHeight int
	list         list.Model

	navDisabled    bool
	disabledKeyMap list.KeyMap

	state NavigationState
	err   error

	termination *Termination

	upCtx      *upbound.Context
	navContext *navContext
}

func (m model) WithTermination(msg string, err error) model {
	m.termination = &Termination{Message: msg, Err: err}
	return m
}

// Run runs the command.
func (c *Cmd) Run(ctx context.Context, kongCtx *kong.Context, upCtx *upbound.Context) error {
	// find profile and derive controlplane from kubeconfig
	po := clientcmd.NewDefaultPathOptions()
	conf, err := po.GetStartingConfig()
	if err != nil {
		return err
	}
	initialState, err := DeriveState(ctx, upCtx, conf, kube.GetIngressHost)
	if err != nil {
		return err
	}

	navCtx := &navContext{
		ingressReader: spaces.NewCachedReader(upCtx.Profile.Session),
		contextWriter: c.kubeContextWriter(upCtx),
	}

	// non-interactive mode via positional argument
	switch c.Argument {
	case "-":
		return c.RunSwap(ctx, upCtx)
	case "":
		return c.RunInteractive(ctx, kongCtx, upCtx, navCtx, initialState)
	default:
		return c.RunNonInteractive(ctx, upCtx, navCtx, initialState)
	}
}

func updateProfile(upCtx *upbound.Context, breadcrumbs Breadcrumbs) error {
	path := breadcrumbs.String()
	upCtx.Profile.CurrentKubeContext = path
	if err := upCtx.Cfg.AddOrUpdateUpboundProfile(upCtx.ProfileName, upCtx.Profile); err != nil {
		return err
	}
	return upCtx.CfgSrc.UpdateConfig(upCtx.Cfg)
}

// RunSwap runs the quick swap version of `up ctx`.
func (c *Cmd) RunSwap(ctx context.Context, upCtx *upbound.Context) error { //nolint:gocyclo // TODO: shorten
	last, err := kube.ReadLastContext()
	if err != nil {
		return err
	}

	// load kubeconfig
	confRaw, err := upCtx.GetRawKubeconfig()
	if err != nil {
		return err
	}
	conf := &confRaw

	// more complicated case: last context is upbound-previous and we have to rename
	conf, oldContext, err := activateContext(conf, last, c.KubeContext)
	if err != nil {
		return err
	}

	// write kubeconfig
	state, err := DeriveState(ctx, upCtx, conf, kube.GetIngressHost)
	if err != nil {
		return err
	}
	if err := clientcmd.ModifyConfig(upCtx.Kubecfg.ConfigAccess(), *conf, true); err != nil {
		return err
	}
	if err := kube.WriteLastContext(oldContext); err != nil {
		return err
	}
	if c.Short {
		fmt.Println(state.Breadcrumbs()) //nolint:forbidigo // Interactive command.
	} else {
		fmt.Printf(contextSwitchedFmt, withUpboundPrefix(state.Breadcrumbs().styledString())) //nolint:forbidigo // Interactive command.
	}

	return updateProfile(upCtx, state.Breadcrumbs())
}

func withUpboundPrefix(s string) string {
	return fmt.Sprintf("%s %s", upboundRootStyle.Render("Upbound"), s)
}

func activateContext(conf *clientcmdapi.Config, sourceContext, preferredContext string) (newConf *clientcmdapi.Config, newLastContext string, err error) { //nolint:gocognit // little long, but well tested
	// switch to non-upbound last context trivially via CurrentContext e.g.
	// - upbound <-> other
	// - something <-> other
	if sourceContext != preferredContext+kube.UpboundPreviousContextSuffix {
		oldCurrent := conf.CurrentContext
		conf.CurrentContext = sourceContext
		return conf, oldCurrent, nil
	}

	if sourceContext == conf.CurrentContext {
		return nil, conf.CurrentContext, nil
	}

	// swap upbound and upbound-previous context
	source, ok := conf.Contexts[sourceContext]
	if !ok {
		return nil, "", fmt.Errorf("no %q context found", preferredContext+kube.UpboundPreviousContextSuffix)
	}
	var current *clientcmdapi.Context
	if conf.CurrentContext != "" {
		current = conf.Contexts[conf.CurrentContext]
	}
	if conf.CurrentContext == preferredContext {
		conf.Contexts[preferredContext] = source

		if current == nil {
			delete(conf.Contexts, preferredContext+kube.UpboundPreviousContextSuffix)
			newLastContext = conf.CurrentContext
		} else {
			conf.Contexts[preferredContext+kube.UpboundPreviousContextSuffix] = current
			newLastContext = preferredContext + kube.UpboundPreviousContextSuffix
		}
	} else {
		// For other <-> upbound-previous, keep "other" for last context
		conf.Contexts[preferredContext] = source
		delete(conf.Contexts, preferredContext+kube.UpboundPreviousContextSuffix)
		newLastContext = conf.CurrentContext
	}
	conf.CurrentContext = preferredContext

	// swap upbound and upbound-previous cluster
	if conf.Contexts[preferredContext].Cluster == preferredContext+kube.UpboundPreviousContextSuffix {
		prev := conf.Clusters[preferredContext+kube.UpboundPreviousContextSuffix]
		if prev == nil {
			return nil, "", fmt.Errorf("no %q cluster found", preferredContext+kube.UpboundPreviousContextSuffix)
		}
		if current := conf.Clusters[preferredContext]; current == nil {
			delete(conf.Clusters, preferredContext+kube.UpboundPreviousContextSuffix)
		} else {
			conf.Clusters[preferredContext+kube.UpboundPreviousContextSuffix] = current
		}
		conf.Clusters[preferredContext] = prev
		for _, ctx := range conf.Contexts {
			if ctx.Cluster == preferredContext+kube.UpboundPreviousContextSuffix {
				ctx.Cluster = preferredContext
			} else if ctx.Cluster == preferredContext {
				ctx.Cluster = preferredContext + kube.UpboundPreviousContextSuffix
			}
		}
	}

	// swap upbound and upbound-previous authInfo
	if conf.Contexts[preferredContext].AuthInfo == preferredContext+kube.UpboundPreviousContextSuffix {
		prev := conf.AuthInfos[preferredContext+kube.UpboundPreviousContextSuffix]
		if prev == nil {
			return nil, "", fmt.Errorf("no %q user found", preferredContext+kube.UpboundPreviousContextSuffix)
		}
		if current := conf.AuthInfos[preferredContext]; current == nil {
			delete(conf.AuthInfos, preferredContext+kube.UpboundPreviousContextSuffix)
		} else {
			conf.AuthInfos[preferredContext+kube.UpboundPreviousContextSuffix] = current
		}
		conf.AuthInfos[preferredContext] = prev
		for _, ctx := range conf.Contexts {
			if ctx.AuthInfo == preferredContext+kube.UpboundPreviousContextSuffix {
				ctx.AuthInfo = preferredContext
			} else if ctx.AuthInfo == preferredContext {
				ctx.AuthInfo = preferredContext + kube.UpboundPreviousContextSuffix
			}
		}
	}

	return conf, newLastContext, nil
}

// RunNonInteractive runs the non-interactive version of `up ctx`.
func (c *Cmd) RunNonInteractive(ctx context.Context, upCtx *upbound.Context, navCtx *navContext, initialState NavigationState) error {
	config, breadcrumbs, err := getKubeconfigNonInteractive(ctx, upCtx, navCtx, initialState, c.Argument)
	if err != nil {
		return err
	}

	// final step if we moved: accept the state
	msg := fmt.Sprintf("Kubeconfig context %q: %s\n", c.KubeContext, withUpboundPrefix(breadcrumbs.styledString()))
	if breadcrumbs.String() != initialState.Breadcrumbs().String() || c.File == "-" {
		if err := navCtx.contextWriter.Write(config); err != nil {
			return err
		}
	}

	// if printing the kubeconfig to stdout, don't print anything else.
	if c.File == "-" {
		return nil
	}

	if c.Short {
		fmt.Println(breadcrumbs) //nolint:forbidigo // Interactive command.
	} else {
		fmt.Print(msg) //nolint:forbidigo // Interactive command.
	}

	return updateProfile(upCtx, breadcrumbs)
}

// GetKubeconfigForPath returns a kubeconfig for the given path.
func GetKubeconfigForPath(ctx context.Context, upCtx *upbound.Context, path string) (*clientcmdapi.Config, error) {
	initialState, err := rootState(ctx, upCtx)
	if err != nil {
		return nil, err
	}

	navCtx := &navContext{
		ingressReader: spaces.NewCachedReader(upCtx.Profile.Session),
		// This shouldn't be used in the call below, but if it does get used we
		// don't want anything to be written (the caller can write the
		// kubeconfig if desired).
		contextWriter: &kube.NopWriter{},
	}

	config, _, err := getKubeconfigNonInteractive(ctx, upCtx, navCtx, initialState, path)

	return config, err
}

func getKubeconfigNonInteractive(ctx context.Context, upCtx *upbound.Context, navCtx *navContext, initialState NavigationState, path string) (*clientcmdapi.Config, Breadcrumbs, error) { //nolint:gocognit // TODO: refactor
	// begin from root unless we're starting from a relative . or ..
	state := initialState
	if !strings.HasPrefix(path, ".") {
		s, err := rootState(ctx, upCtx)
		if err != nil {
			return nil, nil, err
		}
		state = s

		// The root state isn't the empty path; prune its path off of the
		// argument. If we don't prune anything that means the requested context
		// isn't available in the current profile (because it doesn't contain
		// the path to the profile's root state).
		trimmedPath := strings.TrimPrefix(path, strings.Join(state.Breadcrumbs(), "/"))
		if trimmedPath == path {
			return nil, nil, errors.Errorf("context %q is not available in the current profile", path)
		}
		path = trimmedPath
	}

	m := model{
		state:      state,
		upCtx:      upCtx,
		navContext: navCtx,
	}
	for _, s := range strings.Split(path, "/") {
		switch s {
		case "":
			// Ignore empty path components. This allows for trailing slashes,
			// as well as duplicate slashes.
		case ".":
		case "..":
			back, ok := m.state.(Back)
			if !ok {
				return nil, nil, fmt.Errorf("cannot move to parent context from: %s", m.state.Breadcrumbs())
			}
			var err error
			m, err = back.Back(m)
			if err != nil {
				return nil, nil, err
			}
		default:
			// find the string as item
			items, err := m.state.Items(ctx, m.upCtx, m.navContext)
			if err != nil {
				return nil, nil, err
			}
			found := false
			for _, i := range items {
				if i, ok := i.(item); ok && i.Matches(s) {
					if i.onEnter == nil {
						return nil, nil, fmt.Errorf("cannot enter %q in: %s", s, m.state.Breadcrumbs())
					}
					m, err = i.onEnter(m)
					if err != nil {
						return nil, nil, err
					}
					found = true
					break
				}
			}
			if !found {
				return nil, nil, fmt.Errorf("%q not found in: %s", s, m.state.Breadcrumbs())
			}
		}
	}

	a, ok := m.state.(Accepting)
	if !ok {
		return nil, nil, fmt.Errorf("cannot move context to: %s", m.state.Breadcrumbs())
	}

	config, err := a.GetKubeconfig()
	return config, a.Breadcrumbs(), err
}

// RunInteractive runs the interactive version of `up ctx`.
func (c *Cmd) RunInteractive(ctx context.Context, kongCtx *kong.Context, upCtx *upbound.Context, navCtx *navContext, initialState NavigationState) error {
	upCtx.HideLogging()

	// start interactive mode
	m := model{
		state:      initialState,
		upCtx:      upCtx,
		navContext: navCtx,
	}
	items, err := m.state.Items(ctx, m.upCtx, m.navContext)
	if err != nil {
		return err
	}
	m.list = newList(items)
	m.list.KeyMap.Quit = key.NewBinding(key.WithDisabled())
	if _, ok := m.state.(Accepting); ok {
		m.list.KeyMap.Quit = quitBinding
	}

	result, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	if m, ok := result.(model); !ok {
		return fmt.Errorf("unexpected model type: %T", result)
	} else if m.termination != nil {
		if m.termination.Message != "" {
			if _, err := fmt.Fprint(kongCtx.Stderr, m.termination.Message); err != nil {
				return err
			}
		}
		if m.termination.Err != nil {
			return m.termination.Err
		}
		return updateProfile(upCtx, m.state.Breadcrumbs())
	}

	return nil
}

func (c *Cmd) kubeContextWriter(upCtx *upbound.Context) kube.ContextWriter {
	if c.File == "-" {
		return &printWriter{}
	}

	return kube.NewFileWriter(upCtx, c.File, c.KubeContext)
}

type getIngressHostFn func(ctx context.Context, cl corev1client.ConfigMapsGetter) (host string, ca []byte, err error)

// DeriveState returns the navigation state based on the current context set in
// the given kubeconfig.
func DeriveState(ctx context.Context, upCtx *upbound.Context, conf *clientcmdapi.Config, getIngressHost getIngressHostFn) (NavigationState, error) {
	currentCtx := conf.Contexts[conf.CurrentContext]

	spaceExt, err := upbound.GetSpaceExtension(currentCtx)
	if err != nil {
		return nil, err
	}

	if spaceExt == nil || !spaceInProfile(upCtx.Profile, spaceExt) {
		return rootState(ctx, upCtx)
	}

	if spaceExt.Spec.Cloud != nil {
		return DeriveExistingCloudState(ctx, upCtx, conf, spaceExt.Spec.Cloud)
	} else if spaceExt.Spec.Disconnected != nil {
		return DeriveExistingDisconnectedState(ctx, upCtx, conf, spaceExt.Spec.Disconnected, getIngressHost)
	}
	return nil, errors.New("unable to derive state using context extension")
}

func spaceInProfile(p profile.Profile, spaceExt *upbound.SpaceExtension) bool {
	switch p.Type {
	case profile.TypeCloud:
		return spaceExt.Spec.Cloud != nil &&
			spaceExt.Spec.Cloud.Organization == p.Organization

	case profile.TypeDisconnected:
		return spaceExt.Spec.Disconnected != nil &&
			spaceExt.Spec.Disconnected.HubContext == p.SpaceKubeconfig.CurrentContext

	default:
		return false
	}
}

// DeriveExistingDisconnectedState derives the navigation state assuming the
// current context in the passed kubeconfig is pointing at an existing
// disconnected space created by the CLI.
func DeriveExistingDisconnectedState(ctx context.Context, upCtx *upbound.Context, conf *clientcmdapi.Config, disconnected *upbound.DisconnectedConfiguration, getIngressHost getIngressHostFn) (NavigationState, error) {
	if _, ok := upCtx.Profile.SpaceKubeconfig.Contexts[disconnected.HubContext]; !ok {
		return nil, fmt.Errorf("cannot find space hub context %q", disconnected.HubContext)
	}

	var ingress string
	var ctp types.NamespacedName
	var ca []byte

	// determine the ingress either by looking up the base URL if we're in a
	// ctp, or querying for the config map if we're in a group

	ingress, ctp, _ = upCtx.GetCurrentSpaceContextScope()
	if ctp.Name != "" {
		// we're in a ctp, so re-use the CA of the current cluster
		ca = conf.Clusters[conf.Contexts[conf.CurrentContext].Cluster].CertificateAuthorityData
	} else {
		// get ingress from hub
		rest, err := clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
			return upCtx.Profile.SpaceKubeconfig, nil
		})
		if err != nil {
			return rootState(ctx, upCtx)
		}

		cl, err := corev1client.NewForConfig(rest)
		if err != nil {
			return rootState(ctx, upCtx)
		}

		ingress, ca, err = getIngressHost(ctx, cl)
		if err != nil {
			// ingress inaccessible or doesn't exist
			return rootState(ctx, upCtx)
		}
	}

	space := DisconnectedSpace{
		BaseKubeconfig: upCtx.Profile.SpaceKubeconfig,
		Ingress: spaces.SpaceIngress{
			Host:   ingress,
			CAData: ca,
		},
	}

	// derive navigation state
	switch {
	case ctp.Namespace != "" && ctp.Name != "":
		return &ControlPlane{
			Group: Group{
				Space: &space,
				Name:  ctp.Namespace,
			},
			Name: ctp.Name,
		}, nil
	case ctp.Namespace != "":
		return &Group{
			Space: &space,
			Name:  ctp.Namespace,
		}, nil
	default:
		return &space, nil
	}
}

// DeriveExistingCloudState derives the navigation state assuming that the
// current context in the passed kubeconfig is pointing at an existing Cloud
// space previously created by the CLI.
func DeriveExistingCloudState(ctx context.Context, upCtx *upbound.Context, conf *clientcmdapi.Config, cloud *upbound.CloudConfiguration) (NavigationState, error) {
	auth := conf.AuthInfos[conf.Contexts[conf.CurrentContext].AuthInfo]
	ca := conf.Clusters[conf.Contexts[conf.CurrentContext].Cluster].CertificateAuthorityData

	// the exec was modified or wasn't produced by up
	if cloud == nil || cloud.Organization == "" {
		return rootState(ctx, upCtx)
	}

	org := &Organization{
		Name: cloud.Organization,
	}

	ingress, ctp, inSpace := upCtx.GetCurrentSpaceContextScope()
	if !inSpace {
		return nil, errParseSpaceContext
	}

	spaceName := cloud.SpaceName
	if spaceName == "" {
		// The space name wasn't always present in the extension. Fall back to
		// the old behavior of deriving it from the ingress URL.
		spaceName = strings.TrimPrefix(strings.Split(ingress, ".")[0], "https://")
	}
	space := CloudSpace{
		Org:  *org,
		name: spaceName,

		Ingress: spaces.SpaceIngress{
			Host:   strings.TrimPrefix(ingress, "https://"),
			CAData: ca,
		},

		AuthInfo: auth,
	}

	// derive navigation state
	switch {
	case ctp.Namespace != "" && ctp.Name != "":
		return &ControlPlane{
			Group: Group{
				Space: &space,
				Name:  ctp.Namespace,
			},
			Name: ctp.Name,
		}, nil
	case ctp.Namespace != "":
		return &Group{
			Space: &space,
			Name:  ctp.Namespace,
		}, nil
	default:
		return &space, nil
	}
}
