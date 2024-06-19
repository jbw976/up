// Copyright 2025 Upbound Inc.
// All rights reserved

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/alecthomas/kong"
	"github.com/gobuffalo/flect"

	"github.com/upbound/up/internal/version"

	_ "embed"
)

var (
	//go:embed docs-templates/command.md.tmpl
	tmpl string
	//go:embed docs-templates/index.md.tmpl
	indexTmpl string
)

type docsCmd struct {
	OutputDir string `help:"Root path of the docs repository" name:"output-dir"`

	tmpl      *template.Template
	indexTmpl *template.Template
}

func (d *docsCmd) AfterApply() error {
	replacer := strings.NewReplacer("<", "&lt;")

	t, err := template.New("docs").
		Funcs(template.FuncMap{"htmlEscape": replacer.Replace}).
		Parse(tmpl)
	if err != nil {
		return err
	}
	d.tmpl = t

	it, err := template.New("index").
		Funcs(template.FuncMap{"htmlEscape": replacer.Replace}).
		Parse(indexTmpl)
	if err != nil {
		return err
	}
	d.indexTmpl = it

	return nil
}

func (d *docsCmd) Run(ctx *kong.Context) error {
	root := ctx.Model.Node
	if err := traverseChildren(root, d.docsForNode); err != nil {
		return err
	}

	// Build the index by collecting all the commands and their doc filenames,
	// then constructing an mdx file using them.
	type docsNode struct {
		Cmd        string
		ImportName string
		ImportStr  string
	}
	type indexInput struct {
		Version string
		Nodes   []docsNode
	}
	input := indexInput{
		Version: version.Version(),
	}

	if err := traverseChildren(root, func(n *kong.Node) error {
		importName := n.FullPath()
		importName = flect.Titleize(importName)
		importName = strings.ReplaceAll(importName, " ", "")

		fname := n.FullPath()
		fname = strings.ReplaceAll(fname, " ", "_")
		fname += ".md"

		input.Nodes = append(input.Nodes, docsNode{
			Cmd:        n.FullPath(),
			ImportName: importName,
			ImportStr:  fmt.Sprintf("import %s from '/cli/%s';", importName, fname),
		})

		return nil
	}); err != nil {
		return err
	}

	// Sort the nodes by command name, since that looks nicer in the docs than
	// our grouping-based sorting.
	slices.SortFunc(input.Nodes, func(a, b docsNode) int {
		return strings.Compare(a.Cmd, b.Cmd)
	})

	var buf bytes.Buffer
	if err := d.indexTmpl.Execute(&buf, input); err != nil {
		return err
	}
	//nolint:gosec // 0644 is a fine mode for docs.
	return os.WriteFile(filepath.Join(d.OutputDir, "docs", "reference", "cli-reference.md"), buf.Bytes(), 0o644)
}

func (d *docsCmd) docsForNode(n *kong.Node) error {
	fname := n.FullPath()
	fname = strings.ReplaceAll(fname, " ", "_")
	fname += ".md"
	fname = filepath.Join(d.OutputDir, "static", "cli", fname)

	var buf bytes.Buffer
	if err := d.tmpl.Execute(&buf, n); err != nil {
		return err
	}
	md := buf.Bytes()

	return os.WriteFile(fname, md, 0o644) //nolint:gosec // 0644 is a fine mode for docs.
}

func traverseChildren(root *kong.Node, fn func(*kong.Node) error) error {
	root.Aliases = nil

	if root.Hidden {
		return nil
	}
	err := fn(root)
	if err != nil {
		return err
	}
	for _, node := range root.Children {
		err := traverseChildren(node, fn)
		if err != nil {
			return err
		}
	}
	return nil
}
