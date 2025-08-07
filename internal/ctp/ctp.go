// Copyright 2025 Upbound Inc.
// All rights reserved

// Package ctp manages control planes for inner-loop development purposes.
package ctp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	kind "sigs.k8s.io/kind/pkg/cluster"

	commonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"

	licensev1alpha1 "github.com/upbound/controller-manager/apis/licensing/v1alpha1"
	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/cmd/up/space/prerequisites/ingressnginx"
	"github.com/upbound/up/internal/async"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/ctp/certs"
	intctx "github.com/upbound/up/internal/ctx"
	"github.com/upbound/up/internal/docker"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/uxp"
	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/yaml"
	"github.com/upbound/uxp-licensing/pkg/license"
)

const (
	// devControlPlaneClass is used in project and test commands.
	devControlPlaneClass = "small"
	// devControlPlaneAnnotation is used in project and test commands.
	devControlPlaneAnnotation = "upbound.io/development-control-plane"
	// crossplaneNamespace is the namespace into which we install Crossplane.
	crossplaneNamespace = "crossplane-system"
	// pullSecretName is the name of the xpkg pull secret we create in the
	// crossplane namespace.
	pullSecretName = "upbound-pull-secret"
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
	registryDir       string
	ingress           bool
	portMapping       string
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

// WithLocalRegistryDirectory sets the directory to use for the local
// sideloading registry's data when creating a local dev control plane.
func WithLocalRegistryDirectory(path string) EnsureDevControlPlaneOption {
	return func(cfg *ensureDevControlPlaneConfig) {
		cfg.localConfig.registryDir = path
	}
}

// WithIngress sets whether to install ingress and the port mapping for local
// dev control planes.
func WithIngress(enabled bool, portMapping string) EnsureDevControlPlaneOption {
	return func(cfg *ensureDevControlPlaneConfig) {
		cfg.localConfig.ingress = enabled
		cfg.localConfig.portMapping = portMapping
	}
}

// DevControlPlane is a control plane used for local development. It may run in
// a variety of ways.
type DevControlPlane interface {
	// Info returns human-friendly information about the control plane.
	Info() string
	// ShortDescription returns a short description of the control plane,
	// suitable for use in a prompt.
	ShortDescription() string
	// Client returns a controller-runtime client for the control plane.
	Client() client.Client
	// Kubeconfig returns a kubeconfig for the control plane.
	Kubeconfig() clientcmd.ClientConfig
	// Teardown tears down the control plane, deleting any resources it may use.
	Teardown(ctx context.Context, force bool) error
}

// SideloadingControlPlane can sideload packages.
type SideloadingControlPlane interface {
	// Sideload sideloads packages.
	Sideload(ctx context.Context, imgMap project.ImageTagMap, tag name.Tag) error
}

// IngressControlPlane is a control plane with an ingress for the UXP Web UI.
type IngressControlPlane interface {
	// WebUIAddress returns the address for the UXP web UI.
	WebUIAddress() string
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

// ShortDescription returns a short description of the control plane.
func (s *spacesDevControlPlane) ShortDescription() string {
	return fmt.Sprintf("Spaces control plane %s", s.breadcrumbs)
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
	name                string
	kubeconfig          clientcmd.ClientConfig
	client              client.Client
	registryDir         string
	registryContainerID string
	registryHostname    string
}

// Info returns human-readable information about the dev control plane.
func (l *localDevControlPlane) Info() string {
	return fmt.Sprintf("💻 Local dev control plane running in kind cluster %q.", l.name)
}

// ShortDescription returns a short description of the control plane.
func (l *localDevControlPlane) ShortDescription() string {
	return fmt.Sprintf("kind cluster %s", l.name)
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
func (l *localDevControlPlane) Teardown(ctx context.Context, _ bool) error {
	provider := kind.NewProvider()

	if err := provider.Delete(l.name, ""); err != nil {
		return errors.Wrap(err, "failed to delete the local control plane")
	}

	if err := teardownLocalRegistry(ctx, l.registryContainerID); err != nil {
		return errors.Wrap(err, "failed to tear down registry")
	}

	_ = os.RemoveAll(l.registryDir)

	return nil
}

// Sideload sideloads packages into a control plane.
func (l *localDevControlPlane) Sideload(ctx context.Context, imgMap project.ImageTagMap, tag name.Tag) error {
	cfgImage, fnImages, err := project.SortImages(imgMap, tag.Repository.Name())
	if err != nil {
		return err
	}

	for repo, images := range fnImages {
		p := filepath.Join(l.registryDir, repo.RepositoryStr())
		if err := os.MkdirAll(p, 0o750); err != nil {
			return err
		}

		idx, _, err := xpkg.BuildIndex(images...)
		if err != nil {
			return err
		}

		l, err := layout.Write(p, empty.Index)
		if err != nil {
			return err
		}

		if err := l.AppendIndex(idx, layout.WithAnnotations(map[string]string{
			"org.opencontainers.image.ref.name": tag.TagStr(),
		})); err != nil {
			return err
		}
	}

	p := filepath.Join(l.registryDir, tag.RepositoryStr())
	if err := os.MkdirAll(p, 0o750); err != nil {
		return err
	}

	lpath, err := layout.Write(p, empty.Index)
	if err != nil {
		return err
	}

	if err := lpath.AppendImage(cfgImage, layout.WithAnnotations(map[string]string{
		"org.opencontainers.image.ref.name": tag.TagStr(),
	})); err != nil {
		return err
	}

	// Make everything in the layout is world-readable, since processes in
	// containers may run unprivileged and the registry container needs to read
	// everything in the layout.
	if err := filepath.WalkDir(l.registryDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return os.Chmod(path, 0o755) //nolint:gosec // Container needs to read the dir.
		}

		return os.Chmod(path, 0o644) //nolint:gosec // Container needs to read the file.
	}); err != nil {
		return errors.Wrap(err, "failed to adjust permissions on sideloaded images")
	}

	rewrite := path.Join(l.registryHostname, tag.RepositoryStr())
	imgcfg := &pkgv1beta1.ImageConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "local-registry",
		},
		Spec: pkgv1beta1.ImageConfigSpec{
			MatchImages: []pkgv1beta1.ImageMatch{{
				Type:   pkgv1beta1.Prefix,
				Prefix: tag.Repository.Name(),
			}},
			RewriteImage: &pkgv1beta1.ImageRewrite{
				Prefix: rewrite,
			},
		},
	}

	if err := pkgv1beta1.AddToScheme(l.client.Scheme()); err != nil {
		return err
	}
	if err := l.client.Create(ctx, imgcfg); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create image config")
	}

	return nil
}

type ingressLocalDevControlPlane struct {
	localDevControlPlane

	ingress     bool
	portMapping string
}

func (l *ingressLocalDevControlPlane) Info() string {
	info := l.localDevControlPlane.Info()

	addr := l.WebUIAddress()
	if addr != "" {
		info += fmt.Sprintf("\n🌐 WebUI endpoint: %s", addr)
	}

	return info
}

func (l *ingressLocalDevControlPlane) WebUIAddress() string {
	parts := strings.Split(l.portMapping, ":")
	if len(parts) > 0 {
		return fmt.Sprintf("http://127-0-0-1.nip.io:%s", parts[0])
	}
	// We should never get here.
	return ""
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

	telemetryDisabled := upCtx.Cfg.Upbound.Configuration[config.ConfigurationTelemetryDisabled] == "true"

	ctp, err := ensureLocalDevControlPlane(ctx, upCtx, cfg, telemetryDisabled)
	return ctp, errors.Wrap(err, "cannot create local dev control plane")
}

func ensureLocalDevControlPlane(ctx context.Context, upCtx *upbound.Context, cfg *ensureDevControlPlaneConfig, telemetryDisabled bool) (DevControlPlane, error) {
	evText := "Creating local development control plane"
	cfg.eventChan.SendEvent(evText, async.EventStatusStarted)

	// Check that we have a working Docker-compatible runtime, since everything
	// else will fail if we don't.
	if err := docker.Check(ctx); err != nil {
		cfg.eventChan.SendEvent(evText, async.EventStatusFailure)
		return nil, errors.Wrap(err, "failed to connect to Docker; local dev control planes require a Docker-compatible container runtime")
	}

	// kind creates a docker container named <name>-control-plane, and uses the
	// name as the container's hostname. Hostnames can be at most 63
	// characters. Truncate the name if needed.
	nameLen := len(cfg.name)
	nameLen = min(nameLen, 63-len("-control-plane"))
	cfg.name = cfg.name[:nameLen]

	kubeconfig, actualPortMapping, err := ensureKindCluster(ctx, cfg.name, cfg.localConfig.portMapping, cfg.localConfig.ingress)
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

	// Create the crossplane namespace before we create a bunch of stuff in it.
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: crossplaneNamespace,
		},
	}
	if err := cl.Create(ctx, ns); err != nil && !kerrors.IsAlreadyExists(err) {
		return nil, errors.Wrap(err, "failed to create crossplane-system namespace")
	}

	// Generate a CA and certificate for the local registry.
	regName := cfg.name + "-registry"
	certSecret, ca, err := ensureLocalRegistryCertificate(ctx, cl, regName)
	if err != nil {
		cfg.eventChan.SendEvent(evText, async.EventStatusFailure)
		return nil, errors.Wrap(err, "cannot generate certificate for registry")
	}

	// Create a directory to store sideloaded images and spin up a registry
	// container that uses it. This will let us get images into the dev CTP
	// without pushing to a registry.
	registryDir := cfg.localConfig.registryDir
	if registryDir == "" {
		registryDir = filepath.Join(os.TempDir(), "up-local-registry")
	}
	registryDir = filepath.Join(registryDir, cfg.name)
	if err := os.MkdirAll(registryDir, 0o755); err != nil { //nolint:gosec // Container needs to read the dir.
		return nil, err
	}
	cid, err := ensureLocalRegistry(ctx, cl, cfg.name+"-registry", registryDir, certSecret)
	if err != nil {
		cfg.eventChan.SendEvent(evText, async.EventStatusFailure)
		return nil, err
	}

	if err := ensureUpboundPullSecret(ctx, upCtx, cl); err != nil {
		cfg.eventChan.SendEvent(evText, async.EventStatusFailure)
		return nil, err
	}

	if err := ensureUXP(restConfig, cfg.localConfig.crossplaneVersion, ca.Name, telemetryDisabled); err != nil {
		cfg.eventChan.SendEvent(evText, async.EventStatusFailure)
		return nil, err
	}

	if err := ensureUXPDevLicense(ctx, cl); err != nil {
		cfg.eventChan.SendEvent(evText, async.EventStatusFailure)
		return nil, err
	}

	if err := ensureUpboundImageConfig(ctx, upCtx, cl); err != nil {
		cfg.eventChan.SendEvent(evText, async.EventStatusFailure)
		return nil, err
	}

	ret := &localDevControlPlane{
		name:                cfg.name,
		kubeconfig:          kubeconfig,
		client:              cl,
		registryDir:         registryDir,
		registryContainerID: cid,
		registryHostname:    cfg.name + "-registry:5000",
	}

	if !cfg.localConfig.ingress {
		cfg.eventChan.SendEvent(evText, async.EventStatusSuccess)
		return ret, nil
	}

	if err := ensureWebUIIngress(ctx, cl); err != nil {
		cfg.eventChan.SendEvent(evText, async.EventStatusFailure)
		return nil, err
	}
	if err := ensureIngress(ctx, restConfig, cl); err != nil {
		cfg.eventChan.SendEvent(evText, async.EventStatusFailure)
		return nil, err
	}

	cfg.eventChan.SendEvent(evText, async.EventStatusSuccess)
	return &ingressLocalDevControlPlane{
		localDevControlPlane: *ret,
		ingress:              cfg.localConfig.ingress,
		portMapping:          actualPortMapping,
	}, nil
}

// getExistingClusterPortMapping retrieves the actual port mapping from an existing cluster.
func getExistingClusterPortMapping(ctx context.Context, name string, defaultMapping string) string {
	containerName := name + "-control-plane"
	containerInfo, found, err := docker.GetContainerByName(ctx, containerName, false)
	if err != nil || !found {
		return defaultMapping
	}

	// Find existing ingress port (ports bound to 0.0.0.0, excluding API server).
	for _, portConfig := range containerInfo.Ports {
		if portConfig.IP == "0.0.0.0" && portConfig.PublicPort != 6443 {
			return fmt.Sprintf("%d:%d", portConfig.PublicPort, portConfig.PrivatePort)
		}
	}
	return defaultMapping
}

// createPortMappings creates the port mappings for the kind cluster.
func createPortMappings(portMapping string, ingressEnabled bool) ([]v1alpha4.PortMapping, error) {
	if portMapping == "" && ingressEnabled {
		return []v1alpha4.PortMapping{
			{
				ContainerPort: 80,
				HostPort:      0, // 0 means kind will pick a random available port.
				Protocol:      v1alpha4.PortMappingProtocolTCP,
			},
		}, nil
	}

	if portMapping != "" {
		var hostPort, containerPort int32
		_, err := fmt.Sscanf(portMapping, "%d:%d", &hostPort, &containerPort)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid port mapping: %s", portMapping)
		}
		return []v1alpha4.PortMapping{
			{
				ContainerPort: containerPort,
				HostPort:      hostPort,
				Protocol:      v1alpha4.PortMappingProtocolTCP,
			},
		}, nil
	}

	return nil, nil
}

// createKindClusterConfig creates the kind cluster configuration.
func createKindClusterConfig(extraPortMappings []v1alpha4.PortMapping, ingressEnabled bool) *v1alpha4.Cluster {
	node := v1alpha4.Node{
		Role:              v1alpha4.ControlPlaneRole,
		ExtraPortMappings: extraPortMappings,
	}

	// Only add ingress-ready label if ingress is enabled.
	if ingressEnabled {
		node.KubeadmConfigPatches = []string{
			`kind: InitConfiguration
nodeRegistration:
  kubeletExtraArgs:
    node-labels: "ingress-ready=true"`,
		}
	}

	return &v1alpha4.Cluster{
		TypeMeta: v1alpha4.TypeMeta{
			APIVersion: "kind.x-k8s.io/v1alpha4",
			Kind:       "Cluster",
		},
		Nodes: []v1alpha4.Node{node},
		ContainerdConfigPatches: []string{
			"[plugins.\"io.containerd.grpc.v1.cri\".registry]\nconfig_path = \"/etc/containerd/certs.d\"\n",
		},
	}
}

func ensureKindCluster(ctx context.Context, name string, portMapping string, ingressEnabled bool) (clientcmd.ClientConfig, string, error) {
	provider := kind.NewProvider()
	var actualPortMapping string

	kubeconfigFile, err := os.CreateTemp("", "up-*.kubeconfig")
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to create temporary kubeconfig")
	}
	// We don't need the file handle.
	_ = kubeconfigFile.Close()
	// Clean up the file when we're done, but don't try too hard. If it fails
	// the temporary kubeconfig will be left behind.
	defer func() { _ = os.Remove(kubeconfigFile.Name()) }()

	existing, err := provider.List()
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to list kind clusters")
	}

	if slices.Contains(existing, name) {
		// Handle existing cluster
		actualPortMapping = getExistingClusterPortMapping(ctx, name, portMapping)
		if err := provider.ExportKubeConfig(name, kubeconfigFile.Name(), false); err != nil {
			return nil, "", errors.Wrap(err, "failed to get kubeconfig for kind cluster")
		}
	} else {
		// Create new cluster
		actualPortMapping, err = createNewKindCluster(ctx, provider, name, portMapping, ingressEnabled, kubeconfigFile.Name())
		if err != nil {
			return nil, "", err
		}
	}

	kubeconfigBytes, err := os.ReadFile(kubeconfigFile.Name())
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to load kubeconfig")
	}

	kubeconfig, err := clientcmd.NewClientConfigFromBytes(kubeconfigBytes)
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to parse kubeconfig")
	}

	return kubeconfig, actualPortMapping, nil
}

// createNewKindCluster creates a new kind cluster with the specified configuration.
func createNewKindCluster(ctx context.Context, provider *kind.Provider, name string, portMapping string, ingressEnabled bool, kubeconfigPath string) (string, error) {
	extraPortMappings, err := createPortMappings(portMapping, ingressEnabled)
	if err != nil {
		return "", err
	}

	cfg := createKindClusterConfig(extraPortMappings, ingressEnabled)

	cfgBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal kind config")
	}

	if err := provider.Create(
		name,
		kind.CreateWithRawConfig(cfgBytes),
		// TODO(adamwg): Do we want to customize our base image? Or set a
		// specific version depending on the Crossplane version?
		kind.CreateWithNodeImage(defaults.Image),
		// Removes kind cluster information output.
		kind.CreateWithDisplayUsage(false),
		// Removes 'Thanks for using kind! 😊'
		kind.CreateWithDisplaySalutation(false),
		// Tell kind where to write the kubeconfig so it doesn't munge the
		// user's normal kubeconfig.
		kind.CreateWithKubeconfigPath(kubeconfigPath),
	); err != nil {
		return "", errors.Wrap(err, "failed to create kind cluster")
	}

	// For new clusters with ingress, retrieve the actual port that was assigned
	if ingressEnabled {
		return getExistingClusterPortMapping(ctx, name, portMapping), nil
	}

	return portMapping, nil
}

func ensureUXP(restConfig *rest.Config, version, caConfigMap string, telemetryDisabled bool) error {
	repo := uxp.RepoURL
	mgr, err := helm.NewManager(restConfig,
		"crossplane",
		*repo,
		crossplaneNamespace,
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

	values := map[string]any{
		"args": []string{
			"--enable-dependency-version-upgrades",
			"--enable-function-response-cache",
		},
		"registryCaBundleConfig": map[string]string{
			"name": caConfigMap,
			"key":  certs.SecretKeyCACert,
		},
		"upbound": map[string]any{
			"telemetry": map[string]any{
				"disabled": telemetryDisabled,
			},
		},
	}
	if err = mgr.Install(version, values); err != nil {
		return errors.Wrap(err, "failed to install crossplane")
	}

	return nil
}

func ensureUXPDevLicense(ctx context.Context, cl client.Client) error {
	s := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: crossplaneNamespace,
			Name:      licensev1alpha1.LicenseSecretName,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			licensev1alpha1.LicenseSecretKeyDefault: []byte(uxp.DevLicense),
		},
	}

	l := &licensev1alpha1.License{
		TypeMeta: metav1.TypeMeta{
			APIVersion: licensev1alpha1.LicenseGroupVersionKind.GroupVersion().String(),
			Kind:       licensev1alpha1.LicenseKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: licensev1alpha1.LicenseName,
		},
		Spec: licensev1alpha1.LicenseSpec{
			SecretRef: &licensev1alpha1.LicenseSecretRef{
				Namespace: s.GetNamespace(),
				Name:      s.GetName(),
				Key:       licensev1alpha1.LicenseSecretKeyDefault,
			},
		},
	}

	if err := licensev1alpha1.AddToScheme(cl.Scheme()); err != nil {
		return errors.Wrap(err, "failed to add license types to scheme")
	}
	if err := cl.Patch(ctx, s, client.Apply, client.FieldOwner("up-cli"), client.ForceOwnership); err != nil {
		return errors.Wrap(err, "failed to apply license secret")
	}
	if err := cl.Patch(ctx, l, client.Apply, client.FieldOwner("up-cli"), client.ForceOwnership); err != nil {
		return errors.Wrap(err, "failed to apply license resource")
	}

	return nil
}

func ensureUpboundPullSecret(ctx context.Context, upCtx *upbound.Context, cl client.Client) error {
	// If the user is not logged in we can't create a pull secret for them.
	if upCtx.Organization == "" {
		return nil
	}

	var username string
	switch upCtx.Profile.TokenType {
	case profile.TokenTypeUser:
		username = "_token"

	case profile.TokenTypeRobot:
		username = upCtx.Profile.ID

	case profile.TokenTypePAT:
		// Marketplace accepts only robot tokens and user session tokens, not
		// PATs, so we can't provision a pull secret if the user is
		// authenticated with a PAT.
		return nil
	}

	authStr := base64.StdEncoding.EncodeToString([]byte(username + ":" + upCtx.Profile.Session))
	auth := &create.DockerConfigJSON{
		Auths: map[string]create.DockerConfigEntry{
			upCtx.RegistryEndpoint.Host: {
				Username: username,
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
			Name:      pullSecretName,
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

	return nil
}

func ensureUpboundImageConfig(ctx context.Context, upCtx *upbound.Context, cl client.Client) error {
	// If the user is not logged in we can't create a pull secret for them, and
	// therefore don't have a secret at which to point the ImageConfig.
	if upCtx.Organization == "" {
		return nil
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
						Name: pullSecretName,
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

func ensureWebUIIngress(ctx context.Context, cl client.Client) error {
	// Create webui ingress.
	// ToDo(haarchri): lets add an ingress inside the webui helm-chart.
	ingressClassName := "nginx"
	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webui-ingress",
			Namespace: crossplaneNamespace,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: "127-0-0-1.nip.io",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "webui",
											Port: networkingv1.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := cl.Create(ctx, ingress); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create webui ingress")
	}

	return nil
}

func ensureIngress(ctx context.Context, restConfig *rest.Config, cl client.Client) error {
	// Create the ingress-nginx namespace.
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ingress-nginx",
		},
	}
	if err := cl.Create(ctx, ns); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create ingress-nginx namespace")
	}

	// Add the ingress-nginx helm repository.
	repoURL, err := url.Parse("https://kubernetes.github.io/ingress-nginx")
	if err != nil {
		return errors.Wrap(err, "failed to parse ingress-nginx repo URL")
	}

	mgr, err := helm.NewManager(restConfig,
		"ingress-nginx",
		*repoURL,
		"ingress-nginx",
		helm.Wait(),
	)
	if err != nil {
		return errors.Wrap(err, "failed to build new helm manager for ingress")
	}

	// Check if ingress is already installed.
	if _, err := mgr.GetCurrentVersion(); err == nil {
		// Already installed
		return nil
	}

	values := ingressnginx.GetValues(ingressnginx.NodePort)
	if err = mgr.Install("4.12.1", values); err != nil {
		return errors.Wrap(err, "failed to install ingress-nginx")
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
		group:       group,
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

// ShortDescription returns a short description of the control plane.
func (l *KubeconfigDevControlPlane) ShortDescription() string {
	return fmt.Sprintf("existing kubeconfig context %s", l.context)
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
func NewKubeconfigDevControlPlane(ctx context.Context, upCtx *upbound.Context) (*KubeconfigDevControlPlane, error) {
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

	// Create a pull secret and ImageConfig for the user so their control plane
	// will be able to pull their private packages.
	if err := ensureUpboundPullSecret(ctx, upCtx, cl); err != nil {
		return nil, err
	}
	if err := ensureUpboundImageConfig(ctx, upCtx, cl); err != nil {
		return nil, err
	}

	return &KubeconfigDevControlPlane{
		context:    ctxName,
		kubeconfig: upCtx.Kubecfg,
		client:     cl,
	}, checkUXPLicense(ctx, cl)
}

func checkUXPLicense(ctx context.Context, cl client.Client) error {
	_ = licensev1alpha1.AddToScheme(cl.Scheme())

	var l licensev1alpha1.License
	if err := cl.Get(ctx, types.NamespacedName{Name: licensev1alpha1.LicenseName}, &l); err != nil {
		return errors.Wrap(err, "failed to get uxp license")
	}

	if l.Spec.SecretRef == nil {
		// Community edition - this is fine.
		return nil
	}

	secretKey := l.Spec.SecretRef.Key
	if secretKey == "" {
		secretKey = licensev1alpha1.LicenseSecretKeyDefault
	}

	// Get the license data from the secret and validate it.
	var s corev1.Secret
	if err := cl.Get(ctx, types.NamespacedName{Namespace: l.Spec.SecretRef.Namespace, Name: l.Spec.SecretRef.Name}, &s); err != nil {
		return errors.Wrap(err, "failed to get uxp license data")
	}

	val := license.NewValidator(cl)
	if lic, err := val.Validate(ctx, s.Data[secretKey]); err != nil || lic == nil {
		return errors.Wrap(err, "invalid uxp license")
	}

	return nil
}
