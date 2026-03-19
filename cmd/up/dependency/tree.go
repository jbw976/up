// Copyright 2025 Upbound Inc.
// All rights reserved

package dependency

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/project"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg/dep"
	"github.com/upbound/up/internal/xpkg/dep/cache"
	dmanager "github.com/upbound/up/internal/xpkg/dep/manager"
	xpkg "github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/resolver/image"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"

	_ "embed"
)

//go:embed help/tree.md
var treeHelp string

func (c *treeCmd) Help() string {
	return treeHelp
}

// treeTemplate is used by printer.PrintObjectTemplate for default output.
// JSON and YAML modes ignore this and serialize the treeOutput struct directly.
const treeTemplate = "{{.AsciiText}}"

// treeCmd displays the dependency tree for a project or a specific package.
type treeCmd struct {
	Package     string `arg:"" optional:"" help:"Package reference to show tree for (e.g. xpkg.upbound.io/org/name:version). Defaults to current project."`
	ProjectFile string `default:"upbound.yaml" help:"Path to project definition file." short:"f"`
	CacheDir    string `default:"~/.up/cache/" env:"CACHE_DIR" help:"Directory used for caching package images." type:"path"`

	// cch and res are built in AfterApply; the manager is built in Run so
	// it can be wired with a progress callback tied to the spinner.
	cch  dmanager.Cache
	res  *image.Resolver
	proj *v2alpha1.Project
}

// AfterApply constructs and binds context for the tree command.
func (c *treeCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	ctx := context.Background()

	cacheFS := afero.NewBasePathFs(afero.NewOsFs(), c.CacheDir)
	cch, err := cache.NewLocal("/", cache.WithFS(cacheFS))
	if err != nil {
		return errors.Wrap(err, "failed to create xpkg cache")
	}
	c.cch = cch

	var imageConfigs []v2alpha1.ImageConfig

	if c.Package == "" {
		// Project mode: read project file for image config and deps.
		projFilePath, err := filepath.Abs(c.ProjectFile)
		if err != nil {
			return err
		}
		projDirPath := filepath.Dir(projFilePath)
		projFS := afero.NewBasePathFs(afero.NewOsFs(), projDirPath)

		prj, err := project.Parse(projFS, c.ProjectFile)
		if err != nil {
			return errors.New("this is not a project directory")
		}
		c.proj = prj

		if prj.Spec != nil {
			imageConfigs = prj.Spec.ImageConfig
		}
	}

	c.res = image.NewResolver(
		image.WithImageConfig(imageConfigs),
		image.WithFetcher(image.NewLocalFetcher(image.WithKeychain(upCtx.RegistryKeychain()))),
	)

	kongCtx.BindTo(ctx, (*context.Context)(nil))
	return nil
}

// Run executes the tree command.
func (c *treeCmd) Run(ctx context.Context, printer upterm.Printer) error {
	if c.Package != "" {
		return c.runPackageMode(ctx, printer)
	}
	return c.runProjectMode(ctx, printer)
}

func (c *treeCmd) runPackageMode(ctx context.Context, printer upterm.Printer) error {
	d := dep.New(c.Package)

	sp := printer.NewSuccessSpinner(fmt.Sprintf("Resolving %s...", d.Package))
	sp.Start()

	m, err := dmanager.New(
		dmanager.WithCache(c.cch),
		dmanager.WithResolver(c.res),
		dmanager.WithProgressFunc(func(pkg string) {
			sp.UpdateText(fmt.Sprintf("Resolving %s...", pkg))
		}),
	)
	if err != nil {
		sp.Fail()
		return errors.Wrap(err, "failed to create dependency manager")
	}

	resolved, pkgs, err := m.AddAll(ctx, d)
	if err != nil {
		sp.Fail()
		return fmt.Errorf("failed to resolve %s: %w", d.Package, err)
	}
	sp.Success()

	pkgMap := buildPkgMap(pkgs)
	root := pkgMap[resolved.Package]

	// Build ASCII tree into a strings.Builder.
	var b strings.Builder
	fmt.Fprintln(&b, nodeLabel(resolved, root))
	if root != nil {
		asciiVisited := map[string]bool{resolved.Package: true}
		printChildren(root.Dependencies(), pkgMap, asciiVisited, "", &b)
	}

	// Build structured depNode tree (no global deduplication; cycle-safe).
	depDeps := buildDepNodeChildren(root, pkgMap, map[string]bool{resolved.Package: true})

	output := treeOutput{
		asciiText:    b.String(),
		Package:      resolved.Package,
		Version:      pkgNodeVersion(resolved, root),
		Kind:         pkgNodeKind(root),
		Dependencies: depDeps,
	}
	return printer.PrintObjectTemplate(&output, treeTemplate)
}

func (c *treeCmd) runProjectMode(ctx context.Context, printer upterm.Printer) error {
	if c.proj == nil || c.proj.Spec == nil || len(c.proj.Spec.DependsOn) == 0 {
		name := ""
		if c.proj != nil {
			name = c.proj.Name
		}
		if name == "" {
			name = "(project)"
		}
		output := treeOutput{
			asciiText: name + "\n(no dependencies)\n",
			Name:      name,
		}
		return printer.PrintObjectTemplate(&output, treeTemplate)
	}

	sp := printer.NewSuccessSpinner("Resolving dependencies...")
	sp.Start()

	m, err := dmanager.New(
		dmanager.WithCache(c.cch),
		dmanager.WithResolver(c.res),
		dmanager.WithProgressFunc(func(pkg string) {
			sp.UpdateText(fmt.Sprintf("Resolving %s...", pkg))
		}),
	)
	if err != nil {
		sp.Fail()
		return errors.Wrap(err, "failed to create dependency manager")
	}

	pkgMap := make(map[string]*xpkg.ParsedPackage)
	resolvedDeps := make([]v1beta1.Dependency, 0, len(c.proj.Spec.DependsOn))

	for _, d := range c.proj.Spec.DependsOn {
		converted, ok := dmanager.ConvertToV1beta1(d)
		if !ok {
			continue
		}
		resolved, pkgs, err := m.AddAll(ctx, converted)
		if err != nil {
			sp.Fail()
			return fmt.Errorf("failed to resolve %s: %w", converted.Package, err)
		}
		for name, pkg := range buildPkgMap(pkgs) {
			pkgMap[name] = pkg
		}
		resolvedDeps = append(resolvedDeps, resolved)
	}
	sp.Success()

	// Build ASCII tree.
	var b strings.Builder
	fmt.Fprintln(&b, c.proj.Name)
	asciiVisited := make(map[string]bool)
	for i, d := range resolvedDeps {
		isLast := i == len(resolvedDeps)-1
		node := buildTreeNode(d, pkgMap, asciiVisited)
		printTree(node, "", isLast, &b)
	}

	// Build structured depNode list.
	depNodes := make([]*depNode, 0, len(resolvedDeps))
	for _, d := range resolvedDeps {
		depNodes = append(depNodes, buildDepNode(d, pkgMap, map[string]bool{}))
	}

	output := treeOutput{
		asciiText:    b.String(),
		Name:         c.proj.Name,
		Dependencies: depNodes,
	}
	return printer.PrintObjectTemplate(&output, treeTemplate)
}

// treeOutput is the top-level object passed to printer.PrintObjectTemplate.
// asciiText holds the pre-rendered ASCII tree used by treeTemplate; it is not
// serialized in JSON/YAML output.
type treeOutput struct {
	asciiText    string
	Name         string     `json:"name,omitempty"         yaml:"name,omitempty"`
	Package      string     `json:"package,omitempty"      yaml:"package,omitempty"`
	Version      string     `json:"version,omitempty"      yaml:"version,omitempty"`
	Kind         string     `json:"kind,omitempty"         yaml:"kind,omitempty"`
	Dependencies []*depNode `json:"dependencies"           yaml:"dependencies"`
}

// AsciiText is called by treeTemplate to render the ASCII tree in default mode.
func (t *treeOutput) AsciiText() string { return t.asciiText }

// depNode is a single node in the structured dependency tree used for JSON/YAML output.
// Diamond dependencies are fully expanded (each occurrence includes its full subtree);
// true cycles are broken by the inPath guard.
type depNode struct {
	Package      string     `json:"package"                yaml:"package"`
	Version      string     `json:"version,omitempty"      yaml:"version,omitempty"`
	Kind         string     `json:"kind,omitempty"         yaml:"kind,omitempty"`
	Dependencies []*depNode `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
}

// buildDepNode builds a depNode tree rooted at d.
// inPath tracks the current ancestry path to break true cycles without
// deduplicating diamond dependencies.
func buildDepNode(d v1beta1.Dependency, pkgMap map[string]*xpkg.ParsedPackage, inPath map[string]bool) *depNode {
	pkg := pkgMap[d.Package]
	node := &depNode{
		Package: d.Package,
		Version: pkgNodeVersion(d, pkg),
		Kind:    pkgNodeKind(pkg),
	}
	if pkg == nil || inPath[d.Package] {
		return node
	}
	inPath[d.Package] = true
	for _, child := range pkg.Dependencies() {
		node.Dependencies = append(node.Dependencies, buildDepNode(child, pkgMap, inPath))
	}
	delete(inPath, d.Package)
	return node
}

// buildDepNodeChildren builds depNode children for a resolved root package.
func buildDepNodeChildren(pkg *xpkg.ParsedPackage, pkgMap map[string]*xpkg.ParsedPackage, inPath map[string]bool) []*depNode {
	if pkg == nil {
		return nil
	}
	children := make([]*depNode, 0, len(pkg.Dependencies()))
	for _, child := range pkg.Dependencies() {
		children = append(children, buildDepNode(child, pkgMap, inPath))
	}
	return children
}

// pkgNodeVersion returns the resolved version for a dependency node.
func pkgNodeVersion(d v1beta1.Dependency, pkg *xpkg.ParsedPackage) string {
	if pkg != nil && pkg.Version() != "" {
		return pkg.Version()
	}
	return d.Constraints
}

// pkgNodeKind returns the package kind string, or empty if unknown.
func pkgNodeKind(pkg *xpkg.ParsedPackage) string {
	if pkg != nil {
		return pkg.PKind()
	}
	return ""
}

// treeNode is an intermediate representation used for ASCII rendering.
type treeNode struct {
	label    string
	children []*treeNode
	deduped  bool
}

// buildPkgMap builds a name→ParsedPackage map from a flat slice.
func buildPkgMap(pkgs []*xpkg.ParsedPackage) map[string]*xpkg.ParsedPackage {
	m := make(map[string]*xpkg.ParsedPackage, len(pkgs))
	for _, pkg := range pkgs {
		m[pkg.Name()] = pkg
	}
	return m
}

// nodeLabel returns a display label for a dependency node.
func nodeLabel(d v1beta1.Dependency, pkg *xpkg.ParsedPackage) string {
	version := pkgNodeVersion(d, pkg)
	kind := pkgNodeKind(pkg)
	if kind != "" {
		kind = " (" + kind + ")"
	}
	if version != "" {
		return d.Package + ":" + version + kind
	}
	return d.Package + kind
}

// buildTreeNode recursively builds a treeNode for ASCII rendering.
// visited deduplicates packages that appear more than once in the graph.
func buildTreeNode(d v1beta1.Dependency, pkgMap map[string]*xpkg.ParsedPackage, visited map[string]bool) *treeNode {
	pkg := pkgMap[d.Package]
	label := nodeLabel(d, pkg)

	if visited[d.Package] {
		return &treeNode{label: label + " (*)", deduped: true}
	}
	visited[d.Package] = true

	node := &treeNode{label: label}
	if pkg != nil {
		for _, child := range pkg.Dependencies() {
			node.children = append(node.children, buildTreeNode(child, pkgMap, visited))
		}
	}
	return node
}

// printChildren writes ASCII tree lines for a slice of dependencies.
func printChildren(deps []v1beta1.Dependency, pkgMap map[string]*xpkg.ParsedPackage, visited map[string]bool, prefix string, w io.Writer) {
	for i, d := range deps {
		isLast := i == len(deps)-1
		node := buildTreeNode(d, pkgMap, visited)
		printTree(node, prefix, isLast, w)
	}
}

// printTree renders a treeNode with ASCII tree connectors.
func printTree(node *treeNode, prefix string, isLast bool, w io.Writer) {
	connector := "├── "
	childPrefix := prefix + "│   "
	if isLast {
		connector = "└── "
		childPrefix = prefix + "    "
	}

	fmt.Fprintf(w, "%s%s%s\n", prefix, connector, node.label)

	if !node.deduped {
		for i, child := range node.children {
			printTree(child, childPrefix, i == len(node.children)-1, w)
		}
	}
}
