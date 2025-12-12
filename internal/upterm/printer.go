// Copyright 2025 Upbound Inc.
// All rights reserved

// Package upterm contains helpers for working with the terminal, primarily
// printing output.
package upterm

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/style"
	"github.com/upbound/up/internal/yaml"
)

// Printer describes interactions for working with the ObjectPrinter below.
// NOTE(tnthornton) ideally this would be called "ObjectPrinter".
// TODO(tnthornton) rename this to ObjectPrinter.
type Printer interface {
	Print(obj any, fieldNames []string, extractFields func(any) []string) error

	// PrintTemplate prints the object using the provided Go template, if format
	// is set to default, otherwise prints to JSON or YAML.
	PrintTemplate(obj any, template string) error
}

// The ObjectPrinter is intended to make it easy to print individual structs
// and lists of structs for the 'get' and 'list' commands. It can print as
// a human-readable table, or computer-readable (JSON or YAML).
type ObjectPrinter struct {
	Quiet  config.QuietFlag
	Pretty bool
	Format config.Format
}

// DefaultObjPrinter is the default object printer.
//
//nolint:gochecknoglobals // TODO(adamwg): Make this a function returning the default printer.
var DefaultObjPrinter = ObjectPrinter{
	Quiet:  false,
	Pretty: true,
	Format: config.FormatDefault,
}

// Print will print a single option or an array/slice of objects.
// When printing with default table output, it will only print a given set
// of fields. To specify those fields, the caller should provide the human-readable
// names for those fields (used for column headers) and a function that can be called
// on a single struct that returns those fields as strings.
// When printing JSON or YAML, this will print *all* fields, regardless of
// the list of fields.
func (p *ObjectPrinter) Print(obj any, fieldNames []string, extractFields func(any) []string) error {
	// If user specified quiet, skip printing entirely
	if p.Quiet {
		return nil
	}

	// Print the object with the appropriate formatting.
	switch p.Format {
	case config.FormatJSON:
		return printJSON(obj)
	case config.FormatYAML:
		return printYAML(obj)
	case config.FormatDefault:
		fallthrough
	default:
		return p.printDefault(obj, fieldNames, extractFields)
	}
}

// PrintTemplate prints an object using a go template.
func (p *ObjectPrinter) PrintTemplate(obj any, tmpl string) error {
	// If user specified quiet, skip printing entirely
	if p.Quiet {
		return nil
	}
	// Print the object with the appropriate formatting.
	switch p.Format {
	case config.FormatJSON:
		return printJSON(obj)
	case config.FormatYAML:
		return printYAML(obj)
	case config.FormatDefault:
		fallthrough
	default:
		templ, err := template.New("out").Parse(tmpl)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 8, 1, 1, ' ', 0)
		if err := templ.Execute(w, obj); err != nil {
			return err
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
		if err := w.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func printJSON(obj any) error {
	js, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(js)) //nolint:forbidigo // This is a printing library.
	return err
}

func printYAML(obj any) error {
	ys, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(ys)) //nolint:forbidigo // This is a printing library.
	return err
}

func (p *ObjectPrinter) printDefault(obj any, fieldNames []string, extractFields func(any) []string) error {
	t := reflect.TypeOf(obj)
	k := t.Kind()
	if k == reflect.Array || k == reflect.Slice {
		return p.printDefaultList(obj, fieldNames, extractFields)
	}
	return p.printDefaultObj(obj, fieldNames, extractFields)
}

func (p *ObjectPrinter) printDefaultList(obj any, fieldNames []string, extractFields func(any) []string) error {
	t := table.New().
		Headers(fieldNames...).
		StyleFunc(func(row, col int) lipgloss.Style {
			st := table.DefaultStyles(row, col).
				MarginLeft(1).
				MarginRight(1)
			if row == table.HeaderRow && p.Pretty {
				st = st.Foreground(style.UpboundBrandColor)
			}

			return st
		})
	if !p.Pretty {
		t = t.Border(lipgloss.ASCIIBorder())
	}

	s := reflect.ValueOf(obj)
	l := s.Len()
	data := table.NewStringData()
	for i := range l {
		data.Append(extractFields(s.Index(i).Interface()))
	}
	t = t.Data(data)

	fmt.Println(t.Render()) //nolint:forbidigo // Output library.

	return nil
}

func (p *ObjectPrinter) printDefaultObj(obj any, fieldNames []string, extractFields func(any) []string) error {
	return p.printDefaultList([]any{obj}, fieldNames, extractFields)
}

// PrintColoredError prints errors colored.
func PrintColoredError(finalErr error) {
	errorLines := strings.Split(finalErr.Error(), "\n")

	for _, line := range errorLines {
		switch {
		case strings.HasPrefix(line, "---") && !strings.HasPrefix(line, "----"):
			pterm.FgRed.Println(line) // Expected
		case strings.HasPrefix(line, "+++"):
			pterm.FgGreen.Println(line) // Actual
		case strings.HasPrefix(line, "@@"):
			pterm.FgYellow.Println(line) // Context lines
		case strings.HasPrefix(line, "- "):
			pterm.FgRed.Println(line) // Removed lines
		case strings.HasPrefix(line, "+ "):
			pterm.FgGreen.Println(line) // Added lines
		default:
			pterm.Println(line) // Default text
		}
	}
}
