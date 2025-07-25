// Copyright 2025 Upbound Inc.
// All rights reserved

// schema-generator to generate language schemas for packages
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
	"golang.org/x/term"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/filesystem"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg"
	xpkgmarshaler "github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	"github.com/upbound/up/internal/xpkg/mutators"
	"github.com/upbound/up/internal/xpkg/parser/schema"
	"github.com/upbound/up/internal/xpkg/schemagenerator"
	"github.com/upbound/up/internal/xpkg/schemarunner"
)

type cli struct {
	Format config.Format    `default:"default"           enum:"default,json,yaml"    help:"Format for get/list commands. Can be: json, yaml, default" name:"format"`
	Quiet  config.QuietFlag `help:"Suppress all output." name:"quiet"                short:"q"`
	Pretty *bool            `env:"PRETTY"                help:"Pretty print output." name:"pretty"`
	DryRun bool             `help:"dry-run output."      name:"dry-run"`

	SourceImage string `help:"The source image to pull."    required:""`
	TargetImage string `help:"The target image to push to." required:""`

	PythonExcludes []string `help:"List of CRD filenames to exclude from Python schema generation."`
	KclExcludes    []string `help:"List of CRD filenames to exclude from KCL schema generation."`
	GoExcludes     []string `help:"List of CRD filenames to exclude from Go schema generation."`
	JSONExcludes   []string `help:"List of CRD filenames to exclude from JSON schema generation."`

	printer upterm.ObjectPrinter
}

const customHelpMessage = `
The 'schema-generator' command generates schemas for all supported languages and appends them to provider images.
It pulls the specified source image, generates the schemas, appends them as separate layers, and pushes the modified image to the target destination.

Examples:
	schema-generator \
	--source-image xpkg.upbound.io/upbound/provider-gcp-datalossprevention:v1.8.3 \
	--target-image docker.io/haarchri/provider-gcp-datalossprevention:v1.8.3
		Pulls the source image, appends schema layers, and pushes to the target image.

	schema-generator \
	--source-image xpkg.upbound.io/upbound/provider-gcp-datalossprevention:v1.8.3 \
	--target-image docker.io/haarchri/provider-gcp-datalossprevention:v1.8.3 \
	--python-excludes "datalossprevention.gcp.upbound.io_deidentifytemplates.yaml"
		Pulls the source image, appends schema layers, but excludes specific CRD files from the Python schema generation, then pushes to the target image.
`

// AfterApply configures global settings before executing commands.
func (c *cli) AfterApply(ctx *kong.Context) error { //nolint:unparam // Kong requires an error return.
	var pretty bool
	if c.Pretty != nil {
		pretty = *c.Pretty
	} else {
		pretty = term.IsTerminal(int(os.Stdout.Fd()))
	}

	pterm.EnableStyling()
	if !pretty {
		pterm.DisableStyling()
	}

	printer := upterm.DefaultObjPrinter
	printer.DryRun = c.DryRun
	printer.Format = c.Format
	printer.Pretty = pretty
	printer.Quiet = c.Quiet

	ctx.Bind(printer)
	ctx.BindTo(&printer, (*upterm.Printer)(nil))
	c.printer = printer
	return nil
}

func main() {
	c := cli{}

	parser := kong.Parse(&c,
		kong.Description(customHelpMessage),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
	)

	if err := parser.Run(&c); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func (c *cli) Run() error {
	ctx := context.Background()
	return c.generateSchema(ctx)
}

func (c *cli) generateSchema(ctx context.Context) error { //nolint:gocyclo // schemas
	var (
		processedImages []v1.Image
		mu              sync.Mutex
	)

	// Explicitly pass the default keychain to remote.* calls so we look for Docker credentials.
	keychain := remote.WithAuthFromKeychain(authn.NewMultiKeychain(authn.DefaultKeychain))

	indexRef, err := name.ParseReference(c.SourceImage, name.StrictValidation)
	if err != nil {
		return errors.Wrapf(err, "error parsing source image reference")
	}
	index, err := remote.Index(indexRef, keychain)
	if err != nil {
		return errors.Wrapf(err, "error pulling image index")
	}

	indexManifest, err := index.IndexManifest()
	if err != nil {
		return errors.Wrapf(err, "error retrieving index manifest")
	}

	g, gCtx := errgroup.WithContext(ctx)

	for _, desc := range indexManifest.Manifests {
		g.Go(func() error {
			digestRef := indexRef.Context().Digest(desc.Digest.String())
			img, err := remote.Image(digestRef, keychain)
			if err != nil {
				return errors.Wrapf(err, "error pulling architecture-specific image %s", desc.Digest)
			}

			configFile, err := img.ConfigFile()
			if err != nil {
				return errors.Wrapf(err, "error getting image config file")
			}

			m, err := xpkgmarshaler.NewMarshaler()
			if err != nil {
				return errors.Wrapf(err, "error creating xpkg marshaler")
			}

			parsedPkg, err := m.FromImage(xpkg.Image{Image: img}) //nolint:contextcheck // not needed
			if err != nil {
				return errors.Wrapf(err, "error parsing image")
			}

			memFs := filesystem.MemOverlay(afero.NewMemMapFs())
			if cerr := copyCrdToFs(parsedPkg, memFs); cerr != nil {
				return errors.Wrapf(cerr, "error copying CRDs to filesystem")
			}

			err = upterm.WrapWithSuccessSpinner("Schema Generation", upterm.CheckmarkSuccessSpinner, func() error {
				img, err = c.runSchemaGeneration(gCtx, memFs, img, configFile.Config)
				return err
			}, c.printer)
			if err != nil {
				return errors.Wrapf(err, "error generating schema for architecture %v", desc.Platform)
			}

			mu.Lock()
			processedImages = append(processedImages, img)
			mu.Unlock()

			return nil
		})
	}

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		return err
	}

	// Build a multi-architecture index using the processed images
	multiArchIndex, _, err := xpkg.BuildIndex(processedImages...)
	if err != nil {
		return errors.Wrapf(err, "error building multi-architecture index")
	}

	// Parse the target image reference
	targetRef, err := name.ParseReference(c.TargetImage, name.StrictValidation)
	if err != nil {
		return errors.Wrapf(err, "error parsing target image reference")
	}

	// Push the new multi-arch index using remote.WriteIndex
	err = upterm.WrapWithSuccessSpinner(fmt.Sprintf("Pushing Target Multi-Arch Image %s", c.TargetImage), upterm.CheckmarkSuccessSpinner, func() error {
		return remote.WriteIndex(
			targetRef,
			multiArchIndex,
			keychain)
	}, c.printer)
	if err != nil {
		return errors.Wrapf(err, "error pushing multi-arch image to registry %v", c.TargetImage)
	}

	return nil
}

// copyCrdToFs get Objs from ParsedPackage identifies CRDs, and stores them in FS.
func copyCrdToFs(pp *xpkgmarshaler.ParsedPackage, fs afero.Fs) error {
	for i, obj := range pp.Objs {
		crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
		if !ok {
			return errors.New("object is not a CustomResourceDefinition")
		}

		data, err := yaml.Marshal(crd)
		if err != nil {
			return errors.Wrapf(err, "failed to serialize CRD %d", i)
		}

		crdName := fmt.Sprintf("%s_%s.yaml", crd.Spec.Group, crd.Spec.Names.Plural)
		filePath := filepath.Join(pp.DepName, crdName)

		err = afero.WriteFile(fs, filePath, data, 0o644)
		if err != nil {
			return errors.Wrapf(err, "failed to write CRD %d to FS", i)
		}
	}
	return nil
}

// runSchemaGeneration generates the schema and applies mutators to the base configuration.
func (c *cli) runSchemaGeneration(ctx context.Context, memFs afero.Fs, image v1.Image, cfg v1.Config) (v1.Image, error) {
	schemaRunner := schemarunner.RealSchemaRunner{}

	pfs, err := schemagenerator.GenerateSchemaPython(ctx, memFs, c.PythonExcludes, schemaRunner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate Python schema")
	}
	kfs, err := schemagenerator.GenerateSchemaKcl(ctx, memFs, c.KclExcludes, schemaRunner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate KCL schema")
	}
	gofs, err := schemagenerator.GenerateSchemaGo(ctx, memFs, c.GoExcludes, schemaRunner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate Go schema")
	}
	jsonfs, err := schemagenerator.GenerateSchemaJSON(ctx, memFs, c.JSONExcludes, schemaRunner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate JSON schema")
	}

	var muts []xpkg.Mutator
	if pfs != nil {
		muts = append(muts, mutators.NewSchemaMutator(schema.New(pfs, "", xpkg.StreamFileMode), xpkg.SchemaPythonAnnotation))
	}
	if kfs != nil {
		muts = append(muts, mutators.NewSchemaMutator(schema.New(kfs, "", xpkg.StreamFileMode), xpkg.SchemaKclAnnotation))
	}
	if gofs != nil {
		muts = append(muts, mutators.NewSchemaMutator(schema.New(gofs, "", xpkg.StreamFileMode), xpkg.SchemaGoAnnotation))
	}
	if jsonfs != nil {
		muts = append(muts, mutators.NewSchemaMutator(schema.New(jsonfs, "", xpkg.StreamFileMode), xpkg.SchemaJSONAnnotation))
	}

	for _, mut := range muts {
		if mut != nil {
			var err error
			image, cfg, err = mut.Mutate(image, cfg)
			if err != nil {
				return nil, errors.Wrap(err, "failed to apply mutator")
			}
		}
	}

	image, err = mutate.Config(image, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to mutate config for image")
	}

	image, err = xpkg.AnnotateImage(image)
	if err != nil {
		return nil, errors.Wrap(err, "failed to annotate image")
	}

	return image, nil
}
