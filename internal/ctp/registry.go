// Copyright 2025 Upbound Inc.
// All rights reserved

package ctp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	docker "github.com/docker/docker/client"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/ctp/certs"
)

func ensureLocalRegistry(ctx context.Context, cl client.Client, regName, dir string, certSecret *corev1.Secret) (string, error) {
	// Mirrored from ghcr.io/olareg/olareg.
	const regImage = "xpkg.upbound.io/upbound/olareg:v0.1.2"
	certDir := filepath.Join(dir, ".certs")

	cli, err := docker.NewClientWithOpts(docker.WithAPIVersionNegotiation(), docker.FromEnv)
	if err != nil {
		return "", errors.Wrap(err, "failed to create docker client")
	}

	// Check for existing registry container.
	cs, err := cli.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{Key: "name", Value: regName}),
		// Include non-running containers, so we don't end up with a naming
		// conflict.
		All: true,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to list containers")
	}
	if len(cs) > 0 {
		// Registry container exists. Check whether it has the right certificate
		// data; if not, delete and re-create it. If we fail to read the file it
		// probably was deleted, so re-create.

		//nolint:gosec // We don't do anything dangerous with the CA data.
		caData, err := os.ReadFile(filepath.Join(certDir, "ca.crt"))
		if err == nil && bytes.Equal(caData, certSecret.Data[certs.SecretKeyCACert]) {
			// Make sure the container is running.
			if err := cli.ContainerStart(ctx, cs[0].ID, container.StartOptions{}); err != nil {
				return "", errors.Wrap(err, "failed to start registry container")
			}

			return cs[0].ID, nil
		}

		if err := teardownLocalRegistry(ctx, cli, cs[0].ID); err != nil {
			return "", errors.Wrap(err, "failed to tear down outdated registry")
		}
	}

	// Find kind's network so we can attach the registry to it.
	ns, err := cli.NetworkList(ctx, network.ListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{Key: "name", Value: "kind"}),
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to list networks")
	}
	if len(ns) < 1 {
		return "", errors.New("missing kind network")
	}

	// Write the TLS cert and key files.
	if err := os.MkdirAll(certDir, 0o755); err != nil { //nolint:gosec // Container needs to read the dir.
		return "", errors.New("failed to create cert directory")
	}
	if err := os.WriteFile(filepath.Join(certDir, "ca.crt"), certSecret.Data[certs.SecretKeyCACert], 0o644); err != nil { //nolint:gosec // Container needs to read the file.
		return "", errors.New("failed to write ca cert")
	}
	if err := os.WriteFile(filepath.Join(certDir, "tls.crt"), certSecret.Data[corev1.TLSCertKey], 0o644); err != nil { //nolint:gosec // Container needs to read the file.
		return "", errors.New("failed to write tls cert")
	}
	if err := os.WriteFile(filepath.Join(certDir, "tls.key"), certSecret.Data[corev1.TLSPrivateKeyKey], 0o644); err != nil { //nolint:gosec // Container needs to read the file.
		return "", errors.New("failed to write tls key")
	}

	// Start a new registry container.
	if _, err := cli.ImageInspect(ctx, regImage); err != nil {
		out, err := cli.ImagePull(ctx, regImage, image.PullOptions{})
		if err != nil {
			// Return the error encountered during image pull
			return "", errors.Wrapf(err, "failed to pull image %q", regImage)
		}

		// Ensure the image pull is complete by reading the output stream
		if _, err := io.Copy(io.Discard, out); err != nil {
			return "", errors.Wrapf(err, "failed to read image pull output for %s", regImage)
		}
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: regImage,
			Cmd:   []string{"serve", "--dir=/registry-data", "--api-push=false", "--store-ro", "--tls-cert=/registry-data/.certs/tls.crt", "--tls-key=/registry-data/.certs/tls.key"},
		},
		&container.HostConfig{
			Binds: []string{
				dir + ":/registry-data",
			},
		},
		nil, nil, regName)
	if err != nil {
		return "", errors.Wrap(err, "failed to create registry container")
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", errors.Wrap(err, "failed to start registry container")
	}

	// TODO(adamwg): Add a health/liveness check for the container to make sure
	// it's ready to serve images. If it crashed on startup and never became
	// ready, it will be dead now and Crossplane will be sad when we try to
	// install packages from it. Unfortunately, docker doesn't make this easy.

	// Connect to kind's network.
	if err := cli.NetworkConnect(ctx, ns[0].ID, resp.ID, nil); err != nil {
		return "", errors.Wrap(err, "failed to connect registry to network")
	}

	// Configure containerd in the cluster to accept the local registry's CA
	// certificate.
	if err := configureContainerdLocalRegistry(ctx, cl, regName, string(certSecret.Data[certs.SecretKeyCACert])); err != nil {
		return "", errors.Wrap(err, "failed to configure registry in kind cluster")
	}

	return resp.ID, nil
}

func teardownLocalRegistry(ctx context.Context, cli *docker.Client, cid string) error {
	if err := cli.ContainerStop(ctx, cid, container.StopOptions{}); err != nil {
		return errors.Wrap(err, "failed to stop registry container")
	}
	if err := cli.ContainerRemove(ctx, cid, container.RemoveOptions{Force: true}); err != nil {
		return errors.Wrap(err, "failed to remove registry container")
	}

	return nil
}

// ensureLocalRegistryCertificate creates a CA certificate and server
// certificate for the local registry, returning the previously created versions
// from the cluster if they exist. The CA certificate is stored in a ConfigMap
// for consumption by Crossplane.
func ensureLocalRegistryCertificate(ctx context.Context, cl client.Client, hostname string) (*corev1.Secret, *corev1.ConfigMap, error) {
	const secretName = "local-registry-tls"

	gen := certs.NewTLSCertificateGenerator(crossplaneNamespace, certs.RootCACertSecretName,
		certs.TLSCertificateGeneratorWithServerSecretName(secretName, []string{hostname}),
	)

	if err := gen.Run(ctx, cl); err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate local registry certificate")
	}

	var s corev1.Secret
	if err := cl.Get(ctx, types.NamespacedName{Namespace: crossplaneNamespace, Name: secretName}, &s); err != nil {
		return nil, nil, errors.Wrap(err, "failed to retrieve local registry certificate")
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "local-registry-cert",
			Namespace: crossplaneNamespace,
		},
		BinaryData: map[string][]byte{
			certs.SecretKeyCACert: s.Data[certs.SecretKeyCACert],
		},
	}
	if err := cl.Create(ctx, cm); err != nil && !kerrors.IsAlreadyExists(err) {
		return nil, nil, errors.Wrap(err, "failed to save local registry ca certificate")
	}

	return &s, cm, nil
}

func configureContainerdLocalRegistry(ctx context.Context, cl client.Client, regName, caCert string) error {
	// Configure kind's containerd to talk to the registry. This needs to run in
	// the cluster, and the most reliable way to do that is with a privileged
	// k8s job.
	hostsToml := fmt.Sprintf(`server = "https://%s:5000"

[host."https://%s:5000"]
  ca = "ca.crt"
`, regName, regName)
	cmd := fmt.Sprintf("mkdir -p /containerd-certs/%s:5000", regName)
	cmd += fmt.Sprintf("&& echo '%s' > /containerd-certs/%s:5000/ca.crt", caCert, regName)
	cmd += fmt.Sprintf("&& echo '%s' > /containerd-certs/%s:5000/hosts.toml", hostsToml, regName)
	j := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configure-kind-registry",
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name: "configurator",
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "containerd-certs",
							MountPath: "/containerd-certs",
						}},
						Image:   "docker.io/library/alpine:3",
						Command: []string{"sh", "-c", cmd},
					}},
					Volumes: []corev1.Volume{{
						Name: "containerd-certs",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/etc/containerd/certs.d",
							},
						},
					}},
				},
			},
		},
	}

	if err := cl.Create(ctx, j); err != nil && !kerrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
