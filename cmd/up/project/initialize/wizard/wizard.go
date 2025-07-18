// Copyright 2025 Upbound Inc.
// All rights reserved

// Package wizard provides functionality for interactive project initialization through
// a step-by-step wizard interface.
package wizard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/afero"

	"github.com/upbound/up/cmd/up/runner"
	"github.com/upbound/up/pkg/apis/project/v2alpha1"
)

// Wizard handles the interactive project initialization process, managing state
// and resource generation.
type Wizard struct {
	StatePath   string
	Runner      runner.CommandRunner
	Paths       *v2alpha1.ProjectPaths
	ProjectFile string
	ProjectFS   afero.Fs
}

// Run executes the wizard workflow, handling user interaction and resource generation.
// It returns the final state of the wizard and any error that occurred.
func (w *Wizard) Run() (State, error) {
	var state State
	var err error
	if _, err = os.Stat(w.StatePath); err == nil {
		savedState, err := LoadState(w.StatePath)
		if err != nil {
			return state, fmt.Errorf("failed to load wizard state: %w", err)
		}
		cont, _ := pterm.DefaultInteractiveConfirm.Show("Continue from previous wizard state?")
		if cont {
			state = savedState
		} else {
			state = defaultState()
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(w.StatePath), 0o750); err != nil {
			return state, fmt.Errorf("failed to create wizard state directory: %w", err)
		}
		state = defaultState()
	}

	if err := askUser(&state, w.StatePath); err != nil {
		return state, fmt.Errorf("wizard failed: %w", err)
	}

	// if an example was selected, don't generate resources
	if state.Template != "" {
		deleteState(w.StatePath)
		return state, nil
	}

	deleteState(w.StatePath)
	pterm.Success.Println("Wizard complete!")
	pterm.Debug.Printfln("Final state: %+v", state)

	return state, nil
}

// GenerateResources creates all requested resources based on the wizard state.
// This includes examples, XRDs, compositions, functions, and tests.
func (w *Wizard) GenerateResources(state State) error {
	if err := w.GenerateExample(state); err != nil {
		return err
	}

	if state.GenerateXRD {
		if err := w.GenerateXRD(state); err != nil {
			return err
		}
	}
	if state.GenerateComp {
		if err := w.GenerateComp(state); err != nil {
			return err
		}
	}
	if state.GenerateFunction {
		if err := w.GenerateFunction(state); err != nil {
			return err
		}
	}
	if state.GenerateTest {
		if err := w.GenerateTest(state); err != nil {
			return err
		}
	}
	if state.GenerateAITooling {
		if err := w.GenerateAIToolingCfg(state); err != nil {
			return err
		}
	}

	return nil
}

// GenerateTest generates a test for the given state.
func (w *Wizard) GenerateTest(state State) error {
	args := []string{
		"test",
		"generate",
		"--project-file", w.ProjectFile,
		"--language", w.getLanguage(state.TestLang),
	}

	args = append(args, w.testName(state)) // name of the function

	if err := w.Runner.RunCommand(args); err != nil {
		return fmt.Errorf("failed to generate test: %w", err)
	}

	return nil
}

// GenerateFunction generates a function for the given state.
func (w *Wizard) GenerateFunction(state State) error {
	args := []string{
		"function",
		"generate",
		"--project-file", w.ProjectFile,
		"--language", w.getLanguage(state.FuncLang),
		w.functionName(state),                          // name of the function
		filepath.Join(w.Paths.APIs, w.compPath(state)), // path to the composition
	}

	if err := w.Runner.RunCommand(args); err != nil {
		return fmt.Errorf("failed to generate function: %w", err)
	}

	return nil
}

// GenerateComp generates a composition for the given state.
func (w *Wizard) GenerateComp(state State) error {
	args := []string{
		"composition",
		"generate",
		"--project-file", w.ProjectFile,
		"--path", w.compPath(state),
		filepath.Join(w.Paths.APIs, w.xrdPath(state)),
	}

	if err := w.Runner.RunCommand(args); err != nil {
		return fmt.Errorf("failed to generate comp: %w", err)
	}

	return nil
}

// GenerateXRD generates an XRD for the given state.
func (w *Wizard) GenerateXRD(state State) error {
	args := []string{
		"xrd",
		"generate",
		"--project-file", w.ProjectFile,
		"--path", w.xrdPath(state),
		filepath.Join(w.Paths.Examples, w.examplePath(state)),
	}

	if err := w.Runner.RunCommand(args); err != nil {
		return fmt.Errorf("failed to generate xrd: %w", err)
	}

	return nil
}

// GenerateExample generates an example resource for the given state.
func (w *Wizard) GenerateExample(state State) error {
	args := []string{
		"example",
		"generate",
		"--project-file", w.ProjectFile,
		"--api-group", state.APIGroup,
		"--api-version", state.APIVersion,
		"--kind", state.Kind,
		"--name", state.MetadataName,
	}

	if state.UseXR {
		args = append(args, "--type", "xr")
	} else {
		args = append(args, "--type", "claim", "--namespace", state.MetadataNamespace)
	}

	args = append(args, "--path", w.examplePath(state))

	if err := w.Runner.RunCommand(args); err != nil {
		return fmt.Errorf("failed to generate example: %w", err)
	}

	return nil
}

// GenerateAIToolingCfg generates the tooling configs in the project chosen by
// the user.
func (w *Wizard) GenerateAIToolingCfg(state State) error {
	tools := []string{}
	for _, t := range state.AITooling {
		tools = append(tools, fmt.Sprintf("--%s", w.getAITool(t)))
	}

	args := []string{
		"project",
		"ai",
		"configure-tools",
		"--project-file", w.ProjectFile,
	}
	// append selected tools
	args = append(args, tools...)

	if err := w.Runner.RunCommand(args); err != nil {
		return fmt.Errorf("failed to generate ai tooling config: %w", err)
	}

	return nil
}

func (w *Wizard) examplePath(state State) string {
	return fmt.Sprintf("%s/%s.yaml", strings.ToLower(state.Kind), strings.ToLower(state.MetadataName))
}

func (w *Wizard) xrdPath(state State) string {
	return fmt.Sprintf("%s/definition.yaml", state.Kind)
}

func (w *Wizard) compPath(state State) string {
	return fmt.Sprintf("%s/composition.yaml", state.Kind)
}

func (w *Wizard) functionName(state State) string {
	return strings.ToLower(state.Kind)
}

func (w *Wizard) testName(state State) string {
	return strings.ToLower(state.Kind)
}

func (w *Wizard) getLanguage(lang FunctionLanguage) string {
	switch lang {
	case FunctionLanguageKCL:
		return "kcl"
	case FunctionLanguageGo:
		return "go"
	case FunctionLanguageGoTemplating:
		return "go-templating"
	case FunctionLanguagePython:
		return "python"
	}
	return ""
}

func (w *Wizard) getAITool(tool AIToolingProvider) string {
	switch tool {
	case ToolGemini:
		return "gemini-cli"
	case ToolClaude:
		return "claude-code"
	case ToolCursor:
		return "cursor"
	}
	return ""
}

// PrintNextSteps outputs a series of info messages to the user about the next
// steps to take after the wizard is complete.
func (w *Wizard) PrintNextSteps(state State) {
	pterm.Info.Println("Next steps:")

	nextSteps := []string{}

	nextSteps = append(nextSteps, fmt.Sprintf("Edit the example file at %q", filepath.Join(w.Paths.Examples, w.examplePath(state))))

	if state.GenerateXRD {
		nextSteps = append(nextSteps, fmt.Sprintf("Regenerate the XRD using `up xrd generate --path %s %s`", w.xrdPath(state), filepath.Join(w.Paths.Examples, w.examplePath(state))))
	} else {
		nextSteps = append(nextSteps, fmt.Sprintf("Generate an XRD using `up xrd generate --path %s %s`", w.xrdPath(state), filepath.Join(w.Paths.Examples, w.examplePath(state))))
	}

	if state.GenerateFunction {
		nextSteps = append(nextSteps, fmt.Sprintf("Edit the function files at %q", filepath.Join(w.Paths.Functions, w.functionName(state))))
	} else {
		nextSteps = append(nextSteps, fmt.Sprintf("Generate a function using `up function generate --path %s %s`", w.compPath(state), filepath.Join(w.Paths.APIs, w.xrdPath(state))))
	}

	nextSteps = append(nextSteps, "Build the project using `up project build` or run it using `up project run`")

	if state.GenerateTest {
		nextSteps = append(nextSteps,
			fmt.Sprintf("Edit the test files at %q", filepath.Join(w.Paths.Tests, w.testName(state))),
			"Run the tests using `up test run tests/*`",
		)
	}

	for i, step := range nextSteps {
		pterm.Info.Println(fmt.Sprintf("%d. %s", i+1, step))
	}
}
