// Copyright 2025 Upbound Inc.
// All rights reserved

package report

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/spf13/afero"
	gcpopt "google.golang.org/api/option"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	itar "github.com/upbound/up/internal/tar"
	"github.com/upbound/up/internal/upterm"

	_ "embed"
)

const (
	providerAWS = "aws"
	providerGCP = "gcp"

	errFmtProviderNotSupported = "provider not supported: %s"

	reportDirName = "report"
)

type provider string

func (p provider) Validate() error {
	switch p {
	case providerAWS:
		return nil
	case providerGCP:
		return nil
	default:
		return fmt.Errorf(errFmtProviderNotSupported, p)
	}
}

type updateCmd struct {
	Target string `arg:"" help:"Path to billing report to update (local file path or cloud storage object key)."`
	Source string `arg:"" help:"Path to local billing report containing new data."                               type:"path"`

	// TODO(branden): Add support for Azure.
	Provider provider `default:""                enum:"aws,gcp," env:"UP_BILLING_PROVIDER"                           group:"Storage" help:"Storage provider (required for cloud storage). Must be one of: aws, gcp."`
	Bucket   string   `env:"UP_BILLING_BUCKET"   group:"Storage" help:"Storage bucket (required for cloud storage)." optional:""`
	Endpoint string   `env:"UP_BILLING_ENDPOINT" group:"Storage" help:"Custom storage endpoint."                     optional:""`

	fs afero.Fs
}

//go:embed help/update.md
var updateHelp string

func (c *updateCmd) Help() string {
	return updateHelp
}

func (c *updateCmd) Validate() error {
	// If provider is specified, bucket is required.
	if c.Provider != "" && c.Bucket == "" {
		return fmt.Errorf("--bucket is required when --provider is specified")
	}

	// If bucket is specified, provider is required.
	if c.Bucket != "" && c.Provider == "" {
		return fmt.Errorf("--provider is required when --bucket is specified")
	}

	return nil
}

func (c *updateCmd) AfterApply() error {
	c.fs = afero.NewOsFs()

	if !c.isTargetInCloudStorage() {
		var err error
		c.Target, err = filepath.Abs(c.Target)
		if err != nil {
			return errors.Wrap(err, "failed to get absolute path of target file")
		}
	}
	return nil
}

func (c *updateCmd) Run(p upterm.Printer) error {
	ctx := context.Background()

	c.printParams(p)

	targetPath, cleanupLocalTarget, err := c.prepareLocalTarget(ctx, p)
	if err != nil {
		return err
	}
	defer cleanupLocalTarget()

	updatedPath, err := c.updateLocalTarget(p, c.Source, targetPath)
	if err != nil {
		return err
	}

	if err := c.finalizeRemoteTarget(ctx, p, updatedPath); err != nil {
		return err
	}

	p.Printfln("\nBilling report updated successfully")
	return nil
}

func (c *updateCmd) printParams(p upterm.Printer) {
	p.Printfln("Source file: %s", c.Source)
	if c.isTargetInCloudStorage() {
		p.Printfln("Target object: %s", c.Target)
		p.Printfln("Target bucket: %s", c.Bucket)
		p.Printfln("Target provider: %s", c.Provider)
		if c.Endpoint != "" {
			p.Printfln("Endpoint: %s", c.Endpoint)
		}
	} else {
		p.Printfln("Target file: %s", c.Target)
	}
	p.Printfln("")
}

// prepareLocalTarget prepares a local copy of the target report, It returns the
// path to the local copy and a function that deletes it. If the target doesn't
// exist at its original location, the local copy is initialized as an empty
// report.
func (c *updateCmd) prepareLocalTarget(ctx context.Context, p upterm.Printer) (string, func(), error) {
	if c.isTargetInCloudStorage() {
		p.Printfln("Downloading target from cloud storage...")
		path, err := c.downloadFromCloudStorage(ctx)
		err = errors.Wrap(err, "failed to download report from cloud storage")
		cleanup := func() {
			c.fs.Remove(path) //nolint:errcheck // Cleaning up a tempfile
		}
		return path, cleanup, err
	}
	cleanup := func() {}
	_, err := os.Stat(c.Target)
	if err == nil {
		return c.Target, cleanup, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", nil, errors.Wrap(err, "failed to stat target file")
	}
	// Target doesn't exist, so initialize empty report.
	file, err := c.fs.OpenFile(c.Target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to open target file")
	}
	return c.Target, cleanup, errors.Wrap(createEmptyReport(file), "failed to create empty report")
}

// finalizeRemoteTarget moves a temporary local copy of an updated target report
// to the remote target location, replacing the original if it exists. It
// deletes the temporary local copy.
func (c *updateCmd) finalizeRemoteTarget(ctx context.Context, p upterm.Printer, tmpUpdatedPath string) error {
	if !c.isTargetInCloudStorage() {
		// Target is a local file. Move the updated report to replace the
		// target.
		return errors.Wrap(c.fs.Rename(tmpUpdatedPath, c.Target), "failed to replace target file")
	}

	p.Printfln("Uploading updated report to cloud storage...")
	// Upload updated report back to cloud storage and clean up the local
	// copy.
	defer c.fs.Remove(tmpUpdatedPath) //nolint:errcheck // Cleaning up a tempfile
	return errors.Wrap(c.uploadToCloudStorage(ctx, tmpUpdatedPath), "failed to upload updated report to cloud storage")
}

// isTargetInCloudStorage returns true if the target report to be updated is in
// cloud storage.
func (c *updateCmd) isTargetInCloudStorage() bool {
	return c.Provider != ""
}

func (c *updateCmd) downloadFromCloudStorage(ctx context.Context) (string, error) {
	tempFile, err := afero.TempFile(c.fs, "", "billing_report_target_*.tgz")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temporary file")
	}
	defer tempFile.Close() //nolint:errcheck // Closing a tempfile
	filename := tempFile.Name()

	switch c.Provider {
	case providerAWS:
		return filename, c.downloadFromS3(ctx, tempFile)
	case providerGCP:
		return filename, c.downloadFromGCS(ctx, tempFile)
	default:
		c.fs.Remove(tempFile.Name()) //nolint:errcheck // Cleaning up a tempfile
		return "", fmt.Errorf("unsupported provider: %s", c.Provider)
	}
}

func (c *updateCmd) uploadToCloudStorage(ctx context.Context, filePath string) error {
	switch c.Provider {
	case providerAWS:
		return c.uploadToS3(ctx, filePath)
	case providerGCP:
		return c.uploadToGCS(ctx, filePath)
	default:
		return fmt.Errorf("unsupported provider: %s", c.Provider)
	}
}

// downloadFromS3 downloads a file from S3 if it exists. If it doesn't exist, it
// writes an empty report.
func (c *updateCmd) downloadFromS3(ctx context.Context, file afero.File) error {
	client, err := c.s3Client(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create S3 client")
	}

	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(c.Target),
	})
	if err != nil {
		// Initialize empty report if target does not exist.
		var noSuchKeyErr *types.NoSuchKey
		if errors.As(err, &noSuchKeyErr) {
			return errors.Wrap(createEmptyReport(file), "failed to create empty report")
		}
		return errors.Wrap(err, "failed to get object from S3")
	}
	defer result.Body.Close() //nolint:errcheck // Closing an object that is only read

	_, err = io.Copy(file, result.Body)
	return errors.Wrap(err, "failed to copy object data")
}

// uploadToS3 uploads a file to S3. It will overwrite an existing file.
func (c *updateCmd) uploadToS3(ctx context.Context, filename string) error {
	client, err := c.s3Client(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create S3 client")
	}

	file, err := c.fs.Open(filename)
	if err != nil {
		return errors.Wrap(err, "failed to open file for upload")
	}
	defer file.Close() //nolint:errcheck // Closing file that is only read

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(c.Target),
		Body:   file,
	})
	return errors.Wrap(err, "failed to write to S3")
}

func (c *updateCmd) s3Client(ctx context.Context) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load AWS config")
	}
	opts := []func(*s3.Options){}
	if c.Endpoint != "" {
		opts = append(opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(c.Endpoint)
		})
	}
	return s3.NewFromConfig(cfg, opts...), nil
}

// downloadFromGCS downloads a file from GCS if it exists. If it doesn't exist,
// it creates an empty report.
func (c *updateCmd) downloadFromGCS(ctx context.Context, file afero.File) error {
	client, err := c.gcsClient(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create GCS client")
	}
	defer client.Close() //nolint:errcheck // Closing client

	obj := client.Bucket(c.Bucket).Object(c.Target)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		// Initialize empty report if target does not exist.
		if errors.Is(err, storage.ErrObjectNotExist) {
			return errors.Wrap(createEmptyReport(file), "failed to create empty report")
		}
		return errors.Wrap(err, "failed to create object reader")
	}
	defer reader.Close() //nolint:errcheck // Closing a reader

	_, err = io.Copy(file, reader)
	return errors.Wrap(err, "failed to copy object data")
}

// uploadToGCS uploads a file to GCS. It will overwrite an existing file.
func (c *updateCmd) uploadToGCS(ctx context.Context, filename string) error {
	client, err := c.gcsClient(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create GCS client")
	}
	defer client.Close() //nolint:errcheck // Closing client

	// Open file for upload
	file, err := c.fs.Open(filename)
	if err != nil {
		return errors.Wrap(err, "failed to open file for upload")
	}
	defer file.Close() //nolint:errcheck // Closing file that is only read

	// Upload to GCS
	obj := client.Bucket(c.Bucket).Object(c.Target)
	writer := obj.NewWriter(ctx)

	_, err = io.Copy(writer, file)
	if err != nil {
		writer.Close() //nolint:errcheck // Already handling an error
		return errors.Wrap(err, "failed to write to GCS")
	}

	return errors.Wrap(writer.Close(), "failed to close object writer")
}

func (c *updateCmd) gcsClient(ctx context.Context) (*storage.Client, error) {
	opts := []gcpopt.ClientOption{}
	if c.Endpoint != "" {
		opts = append(opts, gcpopt.WithEndpoint(c.Endpoint))
	}
	return storage.NewClient(ctx, opts...)
}

// updateLocalTarget returns a path to a tempfile containing a gzipped tarball
// of the consolidated report at targetPath updated with the contents of the
// report at sourcePath.
func (c *updateCmd) updateLocalTarget(p upterm.Printer, sourcePath, targetPath string) (string, error) {
	p.Printfln("Updating target report...")

	// Create a temporary working directory for extracting data.
	workDir, err := afero.TempDir(c.fs, "", "report-update-")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temporary working directory")
	}
	defer c.fs.RemoveAll(workDir) //nolint:errcheck // Cleaning up a tempdir
	workFS := afero.NewBasePathFs(c.fs, workDir)

	// Extract the entire target consolidated report.
	err = c.readGzip(targetPath, func(r io.Reader) error {
		return itar.ExtractAll(r, workFS)
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to extract target report")
	}

	// Extract the report dir from the source report, and rename it to prevent
	// collisions.
	dest, err := nextReportDirName(workFS)
	if err != nil {
		return "", errors.Wrap(err, "failed to determine dest report dir")
	}
	err = c.readGzip(sourcePath, func(r io.Reader) error {
		return itar.ExtractTo(r, workFS, reportDirName, dest)
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to extract source report")
	}

	// Create a tarball from the updated data.
	outPath, err := c.createTarGzip(workFS)
	return outPath, errors.Wrap(err, "failed to create updated report")
}

// readGzip calls read with a reader of the decompressed contents of the gzipped
// file at path.
func (c *updateCmd) readGzip(path string, read func(io.Reader) error) error {
	f, err := c.fs.Open(path)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // Closing a read-only file
	r, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer r.Close() //nolint:errcheck // Closing a reader
	return read(r)
}

// createTarGzip returns the path to a gzipped tarball created from fs.
func (c *updateCmd) createTarGzip(fs afero.Fs) (string, error) {
	out, err := afero.TempFile(c.fs, "", "report-updated-*.tgz")
	if err != nil {
		return "", err
	}
	name := out.Name()
	gzw := gzip.NewWriter(out)
	tw := tar.NewWriter(gzw)

	defer func() {
		// Close remaining closers. If a closer is not nil, it means we returned
		// an error before we could close it properly.
		if tw != nil {
			tw.Close() //nolint:errcheck // Already handling an error
		}
		if gzw != nil {
			gzw.Close() //nolint:errcheck // Already handling an error
		}
		if out != nil {
			out.Close() //nolint:errcheck // Already handling an error
		}
	}()

	if err := tw.AddFS(afero.NewIOFS(fs)); err != nil {
		return "", err
	}

	if err := tw.Close(); err != nil {
		return "", err
	}
	tw = nil // Prevent double-close in defer.
	if err := gzw.Close(); err != nil {
		return "", err
	}
	gzw = nil // Prevent double-close in defer.
	if err := out.Close(); err != nil {
		return "", err
	}
	out = nil // Prevent double-close in defer.

	return name, nil
}

func createEmptyReport(file afero.File) error {
	gw := gzip.NewWriter(file)
	tw := tar.NewWriter(gw)
	if err := tw.Close(); err != nil {
		gw.Close() //nolint:errcheck // Nothing to be done, already handling an error
		return errors.Wrap(err, "failed to close tar writer")
	}
	return errors.Wrap(gw.Close(), "failed to close gzip writer")
}

// nextReportDirName returns the next report dir name that won't conflict with
// existing files in fs, which is assumed to contain the contents of a
// consolidated report.
func nextReportDirName(fs afero.Fs) (string, error) {
	files, err := afero.ReadDir(fs, "/")
	if err != nil {
		return "", err
	}
	count := 0
	for _, f := range files {
		if strings.HasPrefix(f.Name(), reportDirName) {
			count++
		}
	}
	return reportDirName + strconv.Itoa(count), nil
}
