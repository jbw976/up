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

// Package schemarunner contains functions for handling containers for schema generation
package schemarunner

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/filesystem"
)

// SchemaRunner defines an interface for schema generation.
type SchemaRunner interface {
	Generate(ctx context.Context, fs afero.Fs, folder string, basePath string, imageName string, args []string) error
}

// RealSchemaRunner implements the SchemaRunner interface and calls schemarunner.Generate.
type RealSchemaRunner struct{}

// Generate runs the containerized language tool for schema generation.
func (r RealSchemaRunner) Generate(ctx context.Context, fromFS afero.Fs, baseFolder, basePath, imageName string, command []string) error { //nolint:gocyclo // start container
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.FromEnv)
	if err != nil {
		return errors.Wrapf(err, "failed to use the docker client")
	}

	if _, _, err := cli.ImageInspectWithRaw(ctx, imageName); err != nil {
		// Attempt to pull the image if it's not found locally
		out, pullErr := cli.ImagePull(ctx, imageName, image.PullOptions{})
		if pullErr != nil {
			// Return the error encountered during image pull
			return errors.Wrapf(pullErr, "failed to pull image %s", imageName)
		}

		// Ensure the image pull is complete by reading the output stream
		if _, err := io.Copy(io.Discard, out); err != nil {
			return errors.Wrapf(err, "failed to read image pull output for %s", imageName)
		}
	}

	// Create the tarball from the Afero filesystem
	var opts []filesystem.FSToTarOption
	if basePath != "" {
		opts = append(opts, filesystem.WithSymlinkBasePath(basePath))
	}
	tarBuffer, err := filesystem.FSToTar(fromFS, baseFolder, opts...)
	if err != nil {
		return errors.Wrapf(err, "failed to create tar from fs")
	}

	// Create the container
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      imageName,
		Cmd:        command,
		WorkingDir: "/data/input",
	}, nil, nil, nil, "")
	if err != nil {
		return errors.Wrapf(err, "failed to launch container")
	}

	// Copy the tar archive to the container
	if err := cli.CopyToContainer(ctx, resp.ID, "/data/input", bytes.NewReader(tarBuffer), container.CopyToContainerOptions{}); err != nil {
		return errors.Wrapf(err, "failed to copy tar to container")
	}

	// Start the container
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return errors.Wrapf(err, "failed to start container")
	}

	// Wait for the container to finish
	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)

	select {
	case status := <-statusCh:
		// Check if the container exited with a non-zero status
		if status.StatusCode != 0 {
			// Get the container logs to understand why it failed
			out, err := cli.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
			if err != nil {
				return errors.Wrapf(err, "failed to get container logs")
			}

			// Read the logs and output them for debugging
			logs := new(strings.Builder)
			if _, err := io.Copy(logs, out); err != nil {
				return errors.Wrapf(err, "failed to read container logs")
			}

			// Return an error with the status code and logs
			return fmt.Errorf("container exited with non-zero status: %d, logs: %s", status.StatusCode, logs.String())
		}
	case err := <-errCh:
		return errors.Wrapf(err, "container unknown failure")
	}

	// Copy the results back from the container to the in-memory filesystem
	if err := copyFromContainerToFs(ctx, cli, resp.ID, "/data/input", fromFS); err != nil {
		return errors.Wrapf(err, "failed to copy tar from container")
	}

	if err := cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{
		RemoveVolumes: true,
	}); err != nil {
		return errors.Wrap(err, "failed to clean up container")
	}

	return nil
}

// copyFromContainerToFs copies files from the container back to the Afero filesystem.
func copyFromContainerToFs(ctx context.Context, cli *client.Client, containerID, containerPath string, fs afero.Fs) error { //nolint:gocyclo // copy from container
	// Copy the files from the container
	reader, _, err := cli.CopyFromContainer(ctx, containerID, containerPath)
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(reader)
	const maxFileSize = 10 * 1024 * 1024 // Set a max size (e.g., 10MB)

	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break // End of tar archive
		}
		if err != nil {
			return err
		}

		// Clean up the path by removing the "input/" prefix
		cleanedPath := filepath.Clean(strings.TrimPrefix(header.Name, "input/"))

		// Create directories or files in the MemMapFs
		switch header.Typeflag {
		case tar.TypeDir:
			if err := fs.MkdirAll(cleanedPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := fs.Create(cleanedPath)
			if err != nil {
				return err
			}

			limitedReader := io.LimitReader(tarReader, maxFileSize)
			if _, err := io.Copy(outFile, limitedReader); err != nil {
				if cerr := outFile.Close(); cerr != nil {
					err = errors.Wrap(cerr, "error while closing file")
				}
				return err
			}
			if cerr := outFile.Close(); cerr != nil {
				return errors.Wrapf(cerr, "error closing file %s", cleanedPath)
			}
		}
	}

	return nil
}
