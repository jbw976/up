// Copyright 2025 Upbound Inc.
// All rights reserved

// Package upterm contains helpers for working with the terminal, primarily
// printing output.
package upterm

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"text/tabwriter"
	"text/template"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/style"
	"github.com/upbound/up/internal/yaml"
)

// Printer provides printing support for CLI commands.
//
//nolint:interfacebloat // We want to pass around a single thing, configured appropriately, so this interface is intentionally big.
type Printer interface {
	ResultPrinter
	SpinnerPrinter

	// Print prints in the manner of fmt.Print.
	Print(a ...any)
	// Println prints in the manner of fmt.Println.
	Println(a ...any)
	// Printf formats and prints in the manner of fmt.Printf.
	Printf(format string, a ...any)
	// Printfnl formatss and prints in the manner of fmt.Printf, followed by a
	// newline.
	Printfln(format string, a ...any)

	// PrintInfo prints styled info.
	PrintInfo(a ...any)
	// PrintInfo prints a styled success message.
	PrintSuccess(a ...any)
	// PrintWarning prints a styled warning.
	PrintWarning(a ...any)
	// PrintError prints a styled error.
	PrintError(a ...any)
}

// ResultPrinter prints the result of a command.
type ResultPrinter interface {
	// PrintObject prints extracted fields from an object.
	PrintObject(obj any, fieldNames []string, extractFields func(any) []string) error

	// PrintObjectTemplate prints the object using the provided Go template, if
	// format is set to default, otherwise prints to JSON or YAML.
	PrintObjectTemplate(obj any, template string) error

	// PrintResult prints an arbitrary result, in the manner of
	// fmt.Println. Prefer the PrintObject methods whenever possible since they
	// respect desired output format. PrintResult prints its arguments without
	// further formatting.
	PrintResult(a ...any)
}

// NewPrinter returns a configured printer. Regular output is printed to out;
// results are printed to result.
func NewPrinter(out, result io.Writer, format config.Format, pretty bool) Printer {
	// Spinner output is suppressed for JSON/YAML formats to prevent status
	// messages from corrupting structured output.
	spinnerOut := out
	if format == config.FormatJSON || format == config.FormatYAML {
		spinnerOut = io.Discard
	}

	var op ResultPrinter
	switch format {
	case config.FormatJSON:
		op = &jsonResultPrinter{
			pretty: pretty,
			out:    result,
		}
	case config.FormatYAML:
		op = &yamlResultPrinter{
			out: result,
		}
	case config.FormatDefault:
		fallthrough
	default:
		op = &tableResultPrinter{
			out:    result,
			pretty: pretty,
		}
	}

	switch {
	case pretty:
		return &prettyPrinter{
			ResultPrinter: op,
			SpinnerPrinter: &defaultSpinnerPrinter{
				pretty: pretty,
				out:    spinnerOut,
			},
			out: out,
		}
	default:
		return &plainPrinter{
			ResultPrinter: op,
			SpinnerPrinter: &defaultSpinnerPrinter{
				pretty: pretty,
				out:    spinnerOut,
			},
			out: out,
		}
	}
}

// NewTestPrinter returns a printer that suppresses all output, suitable for use
// in unit tests.
func NewTestPrinter() Printer {
	return NewPrinter(io.Discard, io.Discard, config.FormatDefault, false)
}

// prettyPrinter prints in a pretty format suitable for modern terminals.
type prettyPrinter struct {
	ResultPrinter
	SpinnerPrinter

	out io.Writer
}

func (p *prettyPrinter) Print(a ...any) {
	_, _ = fmt.Fprint(p.out, a...)
}

func (p *prettyPrinter) Println(a ...any) {
	_, _ = fmt.Fprintln(p.out, a...)
}

func (p *prettyPrinter) Printf(format string, a ...any) {
	_, _ = fmt.Fprintf(p.out, format, a...)
}

func (p *prettyPrinter) Printfln(format string, a ...any) { //nolint:goprintffuncname // This name is fine.
	_, _ = fmt.Fprintf(p.out, format+"\n", a...)
}

func (p *prettyPrinter) PrintInfo(a ...any) {
	p.Print("ℹ️ ")
	p.Println(a...)
}

func (p *prettyPrinter) PrintSuccess(a ...any) {
	st := lipgloss.NewStyle().Foreground(style.GreenColor)
	styled := make([]any, len(a))
	for i, elem := range a {
		styled[i] = st.Render(fmt.Sprint(elem))
	}

	p.Print("🙌 ")
	p.Println(styled...)
}

func (p *prettyPrinter) PrintWarning(a ...any) {
	st := lipgloss.NewStyle().Foreground(style.YellowColor)
	styled := make([]any, len(a))
	for i, elem := range a {
		styled[i] = st.Render(fmt.Sprint(elem))
	}

	p.Print("⚠️ ")
	p.Println(styled...)
}

func (p *prettyPrinter) PrintError(a ...any) {
	st := lipgloss.NewStyle().Foreground(style.RedColor)
	styled := make([]any, len(a))
	for i, elem := range a {
		styled[i] = st.Render(fmt.Sprint(elem))
	}

	p.Print("⛔ ")
	p.Println(styled...)
}

// plainPrinter prints in a plain format suitable for non-TTY outputs.
type plainPrinter struct {
	ResultPrinter
	SpinnerPrinter

	out io.Writer
}

func (p *plainPrinter) Print(a ...any) {
	_, _ = fmt.Fprint(p.out, a...)
}

func (p *plainPrinter) Println(a ...any) {
	_, _ = fmt.Fprintln(p.out, a...)
}

func (p *plainPrinter) Printf(format string, a ...any) {
	_, _ = fmt.Fprintf(p.out, format, a...)
}

func (p *plainPrinter) Printfln(format string, a ...any) { //nolint:goprintffuncname // This name is fine.
	_, _ = fmt.Fprintf(p.out, format+"\n", a...)
}

func (p *plainPrinter) PrintInfo(a ...any) {
	p.Print("INFO: ")
	p.Println(a...)
}

func (p *plainPrinter) PrintSuccess(a ...any) {
	p.Print("SUCCESS: ")
	p.Println(a...)
}

func (p *plainPrinter) PrintWarning(a ...any) {
	p.Print("WARNING: ")
	p.Println(a...)
}

func (p *plainPrinter) PrintError(a ...any) {
	p.Print("ERROR: ")
	p.Println(a...)
}

// tableResultPrinter prints objects as tables.
type tableResultPrinter struct {
	pretty bool
	out    io.Writer
}

func (p *tableResultPrinter) PrintObject(obj any, fieldNames []string, extractFields func(any) []string) error {
	k := reflect.TypeOf(obj).Kind()
	if k != reflect.Array && k != reflect.Slice {
		// Single object case - print it as a table with one row.
		obj = []any{obj}
	}

	return p.printTable(obj, fieldNames, extractFields)
}

func (p *tableResultPrinter) PrintObjectTemplate(obj any, tmpl string) error {
	templ, err := template.New("out").Parse(tmpl)
	if err != nil {
		return err
	}

	// Templates can use tabs to produce aligned output, with the following
	// parameters:
	w := tabwriter.NewWriter(
		p.out,
		// Minimum cell width of 8.
		8,
		// Tab width of 1 character.
		1,
		// Padding of 1 character added to cell content.
		1,
		' ',
		0,
	)
	if err := templ.Execute(w, obj); err != nil {
		return err
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		return err
	}

	return w.Flush()
}

func (p *tableResultPrinter) PrintResult(a ...any) {
	_, _ = fmt.Fprintln(p.out, a...)
}

func (p *tableResultPrinter) printTable(obj any, fieldNames []string, extractFields func(any) []string) error {
	t := table.New().
		Headers(fieldNames...).
		StyleFunc(func(row, col int) lipgloss.Style {
			st := table.DefaultStyles(row, col).
				MarginLeft(1).
				MarginRight(1)
			if row == table.HeaderRow && p.pretty {
				st = st.Foreground(style.UpboundBrandColor)
			}

			return st
		})
	if !p.pretty {
		t = t.Border(lipgloss.ASCIIBorder())
	}

	s := reflect.ValueOf(obj)
	l := s.Len()
	data := table.NewStringData()
	for i := range l {
		data.Append(extractFields(s.Index(i).Interface()))
	}
	t = t.Data(data)

	_, _ = fmt.Fprintln(p.out, t.Render())

	return nil
}

// jsonResultPrinter prints objects as JSON.
type jsonResultPrinter struct {
	pretty bool
	out    io.Writer
}

func (p *jsonResultPrinter) PrintObject(obj any, _ []string, _ func(any) []string) error {
	if p.pretty {
		return printJSONPretty(p.out, obj)
	}
	return printJSON(p.out, obj)
}

func (p *jsonResultPrinter) PrintObjectTemplate(obj any, _ string) error {
	return printJSON(p.out, obj)
}

func (p *jsonResultPrinter) PrintResult(a ...any) {
	_, _ = fmt.Fprintln(p.out, a...)
}

func printJSONPretty(out io.Writer, obj any) error {
	e := json.NewEncoder(out)
	e.SetIndent("", "  ")
	return e.Encode(obj)
}

func printJSON(out io.Writer, obj any) error {
	return json.NewEncoder(out).Encode(obj)
}

// yamlResultPrinter prints objects as YAML.
type yamlResultPrinter struct {
	out io.Writer
}

func (p *yamlResultPrinter) PrintObject(obj any, _ []string, _ func(any) []string) error {
	return printYAML(p.out, obj)
}

func (p *yamlResultPrinter) PrintObjectTemplate(obj any, _ string) error {
	return printYAML(p.out, obj)
}

func (p *yamlResultPrinter) PrintResult(a ...any) {
	_, _ = fmt.Fprintln(p.out, a...)
}

func printYAML(out io.Writer, obj any) error {
	ys, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(out, string(ys))
	return err
}
