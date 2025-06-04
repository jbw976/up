// Copyright 2025 Upbound Inc.
// All rights reserved

// Package ctp manages control planes for inner-loop development purposes.
package ctp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/cmd/create"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kind/pkg/apis/config/defaults"
	kind "sigs.k8s.io/kind/pkg/cluster"

	commonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/cmd/up/uxp"
	"github.com/upbound/up/internal/async"
	intctx "github.com/upbound/up/internal/ctx"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/upbound"
)

const (
	// devControlPlaneClass is used in project and test commands.
	devControlPlaneClass = "small"
	// devControlPlaneAnnotation is used in project and test commands.
	devControlPlaneAnnotation = "upbound.io/development-control-plane"
	// crossplaneNamespace is the namespace into which we install Crossplane.
	crossplaneNamespace = "crossplane-system"
)

// errNotDevControlPlane is used in project and test commands.
var errNotDevControlPlane = errors.New("control plane exists but is not a development control plane")

// EnsureDevControlPlaneOption defines functional options for configuring control plane behavior.
type EnsureDevControlPlaneOption func(*ensureDevControlPlaneConfig)

// ensureDevControlPlaneConfig sets configuration options for creating dev control
// planes.
type ensureDevControlPlaneConfig struct {
	name         string
	forceLocal   bool
	spacesConfig spacesConfig
	localConfig  localConfig
	eventChan    async.EventChannel
}

// spacesConfig holds spaces-specific configuration options for creating dev
// control planes.
type spacesConfig struct {
	group       string
	allowProd   bool
	class       string
	annotations map[string]string
	crossplane  *spacesv1beta1.CrossplaneSpec
}

// localConfig holds local-specific configuration options for creating dev
// control planes.
type localConfig struct {
	crossplaneVersion string
}

// defaultCrossplaneSpec returns the default Crossplane configuration.
func defaultCrossplaneSpec() *spacesv1beta1.CrossplaneSpec {
	return &spacesv1beta1.CrossplaneSpec{
		AutoUpgradeSpec: &spacesv1beta1.CrossplaneAutoUpgradeSpec{
			// TODO(adamwg): For now, dev MCPs always use the rapid
			// channel because they require Crossplane features that are
			// only present in 1.18+. We can stop hard-coding this later
			// when other channels are upgraded.
			Channel: ptr.To(spacesv1beta1.CrossplaneUpgradeRapid),
		},
	}
}

// ForceLocal forces a local control plane to be created even if Spaces is
// available.
func ForceLocal(f bool) EnsureDevControlPlaneOption {
	return func(cfg *ensureDevControlPlaneConfig) {
		cfg.forceLocal = f
	}
}

// WithEventChannel specifies an event channel for progress events.
func WithEventChannel(ch async.EventChannel) EnsureDevControlPlaneOption {
	return func(cfg *ensureDevControlPlaneConfig) {
		cfg.eventChan = ch
	}
}

// SkipDevCheck allows the use of a production control plane.
func SkipDevCheck(s bool) EnsureDevControlPlaneOption {
	return func(cfg *ensureDevControlPlaneConfig) {
		cfg.spacesConfig.allowProd = s
	}
}

// WithSpacesCrossplaneSpec sets the Crossplane version and upgrade channel to
// use when creating a Spaces control plane.
func WithSpacesCrossplaneSpec(crossplane spacesv1beta1.CrossplaneSpec) EnsureDevControlPlaneOption {
	return func(cfg *ensureDevControlPlaneConfig) {
		cfg.spacesConfig.crossplane = &crossplane
	}
}

// WithSpacesGroup sets the name of the spaces group in which to create the
// control plane.
func WithSpacesGroup(g string) EnsureDevControlPlaneOption {
	return func(cfg *ensureDevControlPlaneConfig) {
		cfg.spacesConfig.group = g
	}
}

// WithControlPlaneName sets the name of the control plane to create.
func WithControlPlaneName(n string) EnsureDevControlPlaneOption {
	return func(cfg *ensureDevControlPlaneConfig) {
		cfg.name = n
	}
}

// WithLocalCrossplaneVersion sets the Crossplane version to use when creating a
// local control plane.
func WithLocalCrossplaneVersion(v string) EnsureDevControlPlaneOption {
	return func(cfg *ensureDevControlPlaneConfig) {
		cfg.localConfig.crossplaneVersion = v
	}
}

// DevControlPlane is a control plane used for local development. It may run in
// a variety of ways.
type DevControlPlane interface {
	// Info returns human-friendly information about the control plane.
	Info() string
	// Client returns a controller-runtime client for the control plane.
	Client() client.Client
	// Kubeconfig returns a kubeconfig for the control plane.
	Kubeconfig() clientcmd.ClientConfig
	// Teardown tears down the control plane, deleting any resources it may use.
	Teardown(ctx context.Context, force bool) error
}

// spacesDevControlPlane is a development control plane that runs in a Spaces
// cluster.
type spacesDevControlPlane struct {
	spaceClient client.Client
	group       string
	name        string

	client      client.Client
	kubeconfig  clientcmd.ClientConfig
	breadcrumbs string
}

const spacesInfoFmt = `🚀 Development control plane running in Upbound Spaces.
Connect to the development control plane with 'up ctx %s'.`

// Info returns human-readable information about the control plane.
func (s *spacesDevControlPlane) Info() string {
	return fmt.Sprintf(spacesInfoFmt, s.breadcrumbs)
}

// Client returns a controller-runtime client for the control plane.
func (s *spacesDevControlPlane) Client() client.Client {
	return s.client
}

// Kubeconfig returns a kubeconfig for the control plane.
func (s *spacesDevControlPlane) Kubeconfig() clientcmd.ClientConfig {
	return s.kubeconfig
}

// Teardown tears down the control plane, deleting any resources it may use.
func (s *spacesDevControlPlane) Teardown(ctx context.Context, force bool) error {
	nn := types.NamespacedName{Name: s.name, Namespace: s.group}
	var ctp spacesv1beta1.ControlPlane

	// Fetch the control plane to delete
	err := s.spaceClient.Get(ctx, nn, &ctp)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return errors.New("control plane does not exist, nothing to delete")
		}
		return errors.Wrap(err, "failed to fetch control plane for deletion")
	}

	// Never delete a production control plane unless force is set
	if !force && !isDevControlPlane(&ctp) {
		return errors.New("control plane exists but is not a development control plane")
	}

	// Delete the control plane
	if err := s.spaceClient.Delete(ctx, &ctp); err != nil {
		return errors.Wrap(err, "failed to delete control plane")
	}

	return nil
}

type localDevControlPlane struct {
	name       string
	kubeconfig clientcmd.ClientConfig
	client     client.Client
}

// Info returns human-readable information about the dev control plane.
func (l *localDevControlPlane) Info() string {
	return fmt.Sprintf("💻 Local dev control plane running in kind cluster %q.", l.name)
}

// Client returns a controller-runtime client for the control plane.
func (l *localDevControlPlane) Client() client.Client {
	return l.client
}

// Kubeconfig returns a kubeconfig for the control plane.
func (l *localDevControlPlane) Kubeconfig() clientcmd.ClientConfig {
	return l.kubeconfig
}

// Teardown tears down the control plane, deleting any resources it may use.
func (l *localDevControlPlane) Teardown(_ context.Context, _ bool) error {
	provider := kind.NewProvider()

	if err := provider.Delete(l.name, ""); err != nil {
		return errors.Wrap(err, "failed to delete the local control plane")
	}

	return nil
}

// EnsureDevControlPlane ensures the existence of a control plane for
// development.
func EnsureDevControlPlane(ctx context.Context, upCtx *upbound.Context, opts ...EnsureDevControlPlaneOption) (DevControlPlane, error) {
	cfg := &ensureDevControlPlaneConfig{
		spacesConfig: spacesConfig{
			class: devControlPlaneClass,
			annotations: map[string]string{
				devControlPlaneAnnotation: "true",
			},
			crossplane: defaultCrossplaneSpec(),
		},
	}

	// Apply functional options
	for _, opt := range opts {
		opt(cfg)
	}

	// Determine whether to create a spaces dev ctp or a local dev ctp, as
	// follows:
	//
	// 1. If local was explicitly requested, respect that.
	// 2. If the user's kubeconfig points to a Space, use Spaces by default.
	// 3. Otherwise, use local by default.

	if _, _, err := intctx.GetCurrentGroup(ctx, upCtx); err == nil && !cfg.forceLocal {
		ctp, err := ensureSpacesDevControlPlane(ctx, upCtx, cfg)
		return ctp, errors.Wrap(err, "cannot create dev control plane in Spaces")
	}

	ctp, err := ensureLocalDevControlPlane(ctx, upCtx, cfg)
	return ctp, errors.Wrap(err, "cannot create local dev control plane")
}

func ensureLocalDevControlPlane(ctx context.Context, upCtx *upbound.Context, cfg *ensureDevControlPlaneConfig) (*localDevControlPlane, error) {
	evText := "Creating local development control plane"
	cfg.eventChan.SendEvent(evText, async.EventStatusStarted)

	// kind creates a docker container named <name>-control-plane, and uses the
	// name as the container's hostname. Hostnames can be at most 63
	// characters. Truncate the name if needed.
	nameLen := len(cfg.name)
	nameLen = min(nameLen, 63-len("-control-plane"))
	cfg.name = cfg.name[:nameLen]

	kubeconfig, err := ensureKindCluster(cfg.name)
	if err != nil {
		cfg.eventChan.SendEvent(evText, async.EventStatusFailure)
		return nil, err
	}

	restConfig, err := kubeconfig.ClientConfig()
	if err != nil {
		cfg.eventChan.SendEvent(evText, async.EventStatusFailure)
		return nil, errors.Wrap(err, "cannot get rest config")
	}

	cl, err := client.New(restConfig, client.Options{})
	if err != nil {
		cfg.eventChan.SendEvent(evText, async.EventStatusFailure)
		return nil, errors.Wrap(err, "cannot construct control plane client")
	}

	if err := ensureUXP(ctx, restConfig, cl, cfg.localConfig.crossplaneVersion); err != nil {
		cfg.eventChan.SendEvent(evText, async.EventStatusFailure)
		return nil, err
	}

	if err := ensureUpboundPullSecret(ctx, upCtx, cl); err != nil {
		cfg.eventChan.SendEvent(evText, async.EventStatusFailure)
		return nil, err
	}

	cfg.eventChan.SendEvent(evText, async.EventStatusSuccess)
	return &localDevControlPlane{
		name:       cfg.name,
		kubeconfig: kubeconfig,
		client:     cl,
	}, nil
}

func ensureKindCluster(name string) (clientcmd.ClientConfig, error) {
	provider := kind.NewProvider()

	kubeconfigFile, err := os.CreateTemp("", "up-*.kubeconfig")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temporary kubeconfig")
	}
	// We don't need the file handle.
	_ = kubeconfigFile.Close()
	// Clean up the file when we're done, but don't try too hard. If it fails
	// the temporary kubeconfig will be left behind.
	defer func() { _ = os.Remove(kubeconfigFile.Name()) }()

	existing, err := provider.List()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list kind clusters")
	}
	if slices.Contains(existing, name) {
		if err := provider.ExportKubeConfig(name, kubeconfigFile.Name(), false); err != nil {
			return nil, errors.Wrap(err, "failed to get kubeconfig for kind cluster")
		}
	} else {
		if err := provider.Create(
			name,
			// TODO(adamwg): Do we want to customize our base image? Or set a
			// specific version depending on the Crossplane version?
			kind.CreateWithNodeImage(defaults.Image),
			// Removes kind cluster information output.
			kind.CreateWithDisplayUsage(false),
			// Removes 'Thanks for using kind! 😊'
			kind.CreateWithDisplaySalutation(false),
			// Tell kind where to write the kubeconfig so it doesn't munge the
			// user's normal kubeconfig.
			kind.CreateWithKubeconfigPath(kubeconfigFile.Name()),
		); err != nil {
			return nil, errors.Wrap(err, "failed to create kind cluster")
		}
	}

	kubeconfigBytes, err := os.ReadFile(kubeconfigFile.Name())
	if err != nil {
		return nil, errors.Wrap(err, "failed to load kubeconfig")
	}

	kubeconfig, err := clientcmd.NewClientConfigFromBytes(kubeconfigBytes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse kubeconfig")
	}

	return kubeconfig, nil
}

func ensureUXP(ctx context.Context, restConfig *rest.Config, cl client.Client, version string) error {
	repo := uxp.RepoURL
	mgr, err := helm.NewManager(restConfig,
		"universal-crossplane",
		repo,
		helm.WithNamespace(crossplaneNamespace),
		helm.Wait(),
	)
	if err != nil {
		return errors.Wrap(err, "failed to build new helm manager")
	}

	// If crossplane is already installed, check the version. If it's correct,
	// we'll just use the cluster. If it's incorrect, throw an error (we won't
	// bother upgrading - these are ephemeral clusers).
	if v, err := mgr.GetCurrentVersion(); err == nil {
		if version != "" && v != version {
			return errors.Errorf("existing cluster has wrong crossplane version installed: got %s, want %s", v, version)
		}
		return nil
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: crossplaneNamespace,
		},
	}
	if err := cl.Create(ctx, ns); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create crossplane-system namespace")
	}

	values := map[string]any{
		"args": []string{
			"--enable-dependency-version-upgrades",
			"--enable-function-response-cache",
		},
	}
	if err = mgr.Install(version, values); err != nil {
		return errors.Wrap(err, "failed to install crossplane")
	}

	return nil
}

func ensureUpboundPullSecret(ctx context.Context, upCtx *upbound.Context, cl client.Client) error {
	// If the user is not logged in we can't create a pull secret for them.
	if upCtx.Organization == "" {
		return nil
	}

	const secretName = "up-pull-secret"
	authStr := base64.StdEncoding.EncodeToString([]byte("_token:" + upCtx.Profile.Session))
	auth := &create.DockerConfigJSON{
		Auths: map[string]create.DockerConfigEntry{
			upCtx.RegistryEndpoint.Host: {
				Username: "_token",
				Password: upCtx.Profile.Session,
				Auth:     authStr,
			},
		},
	}
	authJSON, err := json.Marshal(auth)
	if err != nil {
		return errors.Wrap(err, "failed to marshal docker auth config")
	}

	xpkgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: crossplaneNamespace,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: authJSON,
		},
	}
	if err := cl.Create(ctx, xpkgSecret); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create xpkg pull secret")
	}

	imgcfg := &pkgv1beta1.ImageConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "upbound",
		},
		Spec: pkgv1beta1.ImageConfigSpec{
			MatchImages: []pkgv1beta1.ImageMatch{{
				Type:   pkgv1beta1.Prefix,
				Prefix: upCtx.RegistryEndpoint.Host,
			}},
			Registry: &pkgv1beta1.RegistryConfig{
				Authentication: &pkgv1beta1.RegistryAuthentication{
					PullSecretRef: corev1.LocalObjectReference{
						Name: secretName,
					},
				},
			},
		},
	}

	if err := pkgv1beta1.AddToScheme(cl.Scheme()); err != nil {
		return err
	}
	if err := cl.Create(ctx, imgcfg); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create image config")
	}

	return nil
}

func ensureSpacesDevControlPlane(ctx context.Context, upCtx *upbound.Context, cfg *ensureDevControlPlaneConfig) (*spacesDevControlPlane, error) {
	kubeconfig, err := intctx.GetSpacesKubeconfig(ctx, upCtx)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get kubeconfig for current spaces context")
	}
	restConfig, err := kubeconfig.ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get rest config for spaces client")
	}
	spaceClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot construct spaces client")
	}

	group := cfg.spacesConfig.group
	if group == "" {
		ns, _, err := kubeconfig.Namespace()
		if err != nil {
			return nil, errors.Wrap(err, "cannot determine default group")
		}
		if ns == "" {
			ns = "default"
		}
		group = ns
	}
	nn := types.NamespacedName{Name: cfg.name, Namespace: group}
	var ctp spacesv1beta1.ControlPlane

	err = spaceClient.Get(ctx, nn, &ctp)
	switch {
	case err == nil:
		// Make sure it's a dev control plane and not being deleted.
		if !isDevControlPlane(&ctp) && !cfg.spacesConfig.allowProd {
			return nil, errNotDevControlPlane
		}
		if ctp.DeletionTimestamp != nil {
			return nil, errors.New("control plane exists but is being deleted - retry after it finishes deleting")
		}
		// Ensure the Crossplane spec fully matches what the caller specified
		if !matchesCrossplaneSpec(ctp.Spec.Crossplane, *cfg.spacesConfig.crossplane) {
			return nil, errors.Errorf(
				"existing control plane has a different Crossplane spec than expected",
			)
		}

	case kerrors.IsNotFound(err):
		// Create a control plane.
		ctp = spacesv1beta1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:        cfg.name,
				Namespace:   group,
				Annotations: cfg.spacesConfig.annotations,
			},
			Spec: spacesv1beta1.ControlPlaneSpec{
				Class:      cfg.spacesConfig.class,
				Crossplane: *cfg.spacesConfig.crossplane,
			},
		}

		if err := createSpacesControlPlane(ctx, spaceClient, cfg.eventChan, ctp); err != nil {
			return nil, err
		}

	default:
		// Unexpected error.
		return nil, errors.Wrap(err, "failed to check for control plane existence")
	}

	// Get client for the control plane
	space, _, err := intctx.GetCurrentGroup(ctx, upCtx)
	if err != nil {
		return nil, err
	}

	ctpKubeconfig, err := space.BuildKubeconfig(nn)
	if err != nil {
		return nil, err
	}

	ctpRestConfig, err := ctpKubeconfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	ctpClient, err := client.New(ctpRestConfig, client.Options{})
	if err != nil {
		return nil, err
	}

	// Create and return the spacesDevControlPlane
	return &spacesDevControlPlane{
		spaceClient: spaceClient,
		group:       cfg.spacesConfig.group,
		name:        cfg.name,
		client:      ctpClient,
		kubeconfig:  ctpKubeconfig,
		breadcrumbs: fmt.Sprintf("%s/%s/%s", space.Breadcrumbs().String(), group, cfg.name),
	}, nil
}

func isDevControlPlane(ctp *spacesv1beta1.ControlPlane) bool {
	if ctp.Annotations != nil && ctp.Annotations[devControlPlaneAnnotation] == "true" {
		return true
	}

	return false
}

func createSpacesControlPlane(ctx context.Context, cl client.Client, ch async.EventChannel, ctp spacesv1beta1.ControlPlane) error {
	evText := "Creating development control plane in Spaces"
	ch.SendEvent(evText, async.EventStatusStarted)

	backoff := wait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
		Steps:    6,
	}

	if err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		// Try to create the resource
		if err := cl.Create(ctx, &ctp); err != nil {
			// Check if it exists and is being deleted
			existing := &spacesv1beta1.ControlPlane{}
			getErr := cl.Get(ctx, client.ObjectKey{
				Name:      ctp.Name,
				Namespace: ctp.Namespace,
			}, existing)

			if getErr == nil && existing.DeletionTimestamp != nil {
				// It's being deleted, so retry
				return false, nil
			}
			// Not a retryable error
			return false, err
		}

		// Successfully created
		return true, nil
	}); err != nil {
		ch.SendEvent(evText, async.EventStatusFailure)
		return errors.Wrap(err, "failed to create control plane")
	}

	nn := types.NamespacedName{
		Name:      ctp.Name,
		Namespace: ctp.Namespace,
	}
	if err := wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (done bool, err error) {
		err = cl.Get(ctx, nn, &ctp)
		if err != nil {
			return false, err
		}

		cond := ctp.Status.GetCondition(commonv1.TypeReady)
		return cond.Status == corev1.ConditionTrue, nil
	}); err != nil {
		ch.SendEvent(evText, async.EventStatusFailure)
		return errors.Wrap(err, "waiting for control plane to be ready")
	}

	ch.SendEvent(evText, async.EventStatusSuccess)

	return nil
}

func matchesCrossplaneSpec(existing, desired spacesv1beta1.CrossplaneSpec) bool {
	// Spaces applies defaults to the CrossplaneSpec, so we can't compare the
	// full structs. Ignore the version and state unless they're set in our
	// desired spec.

	if desired.Version == nil {
		existing.Version = nil
	}
	if desired.State == nil {
		existing.State = nil
	}

	return cmp.Equal(existing, desired)
}

// KubeconfigDevControlPlane is a dev control plane based on the user's current
// kubeconfig context. It's not really a dev control plane at all, but we want
// to give users this option and it's helpful to use the same abstraction.
type KubeconfigDevControlPlane struct {
	context string

	kubeconfig clientcmd.ClientConfig
	client     client.Client
}

// Info returns human-readable information about the dev control plane.
func (l *KubeconfigDevControlPlane) Info() string {
	return fmt.Sprintf("Using existing kubeconfig context %q.", l.context)
}

// Client returns a controller-runtime client for the control plane.
func (l *KubeconfigDevControlPlane) Client() client.Client {
	return l.client
}

// Kubeconfig returns a kubeconfig for the control plane.
func (l *KubeconfigDevControlPlane) Kubeconfig() clientcmd.ClientConfig {
	return l.kubeconfig
}

// Teardown does nothing for kubeconfig control planes.
func (l *KubeconfigDevControlPlane) Teardown(_ context.Context, _ bool) error {
	return nil
}

// NewKubeconfigDevControlPlane returns an initialized
// KubeconfigDevControlPlane.
func NewKubeconfigDevControlPlane(upCtx *upbound.Context) (*KubeconfigDevControlPlane, error) {
	ctxName, err := upCtx.GetCurrentContextName()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get current kubeconfig context name")
	}

	restConfig, err := upCtx.GetKubeconfig()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get kubeconfig")
	}

	cl, err := client.New(restConfig, client.Options{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot construct control plane client")
	}

	return &KubeconfigDevControlPlane{
		context:    ctxName,
		kubeconfig: upCtx.Kubecfg,
		client:     cl,
	}, nil
}
