// Copyright 2025 Upbound Inc.
// All rights reserved

package wizard

import (
	"encoding/json"
	"maps"
	"os"
	"slices"

	"github.com/pterm/pterm"
)

// FunctionLanguage represents the programming language used for function implementation.
type FunctionLanguage string

const (
	// FunctionLanguageKCL represents the KCL programming language.
	FunctionLanguageKCL FunctionLanguage = "kcl"
	// FunctionLanguageGo represents the Go programming language.
	FunctionLanguageGo FunctionLanguage = "go"
	// FunctionLanguageGoTemplating represents Go template-based functions.
	FunctionLanguageGoTemplating FunctionLanguage = "go-templating"
	// FunctionLanguagePython represents the Python programming language.
	FunctionLanguagePython FunctionLanguage = "python"
)

// SupportedLanguages contains all supported programming languages for functions.
var SupportedLanguages = []string{ //nolint:gochecknoglobals // this is a constant
	string(FunctionLanguageKCL),
	string(FunctionLanguageGo),
	string(FunctionLanguageGoTemplating),
	string(FunctionLanguagePython),
}

// SupportedLanguagesMap maps the user-friendly language name to the
// FunctionLanguage, used for the wizard.
var SupportedLanguagesMap = map[string]FunctionLanguage{ //nolint:gochecknoglobals // this is a constant
	"KCL":          FunctionLanguageKCL,
	"Go":           FunctionLanguageGo,
	"Go Templates": FunctionLanguageGoTemplating,
	"Python":       FunctionLanguagePython,
}

// SupportedTestLanguages contains languages supported for test implementation.
var SupportedTestLanguages = []string{ //nolint:gochecknoglobals // this is a constant
	string(FunctionLanguageKCL),
	string(FunctionLanguagePython),
}

// SupportedTestLanguagesMap maps the user-friendly language name to the
// FunctionLanguage, used for the wizard.
var SupportedTestLanguagesMap = map[string]FunctionLanguage{ //nolint:gochecknoglobals // this is a constant
	"KCL":    FunctionLanguageKCL,
	"Python": FunctionLanguagePython,
}

// BlankProjectTemplate is the URL of the blank project template.
const BlankProjectTemplate = "https://github.com/upbound/project-template-scratch"

// availableTemplates maps template names to their repository URLs.
var availableTemplates = map[string]string{ //nolint:gochecknoglobals // this is a constant
	"AWS Bucket":         "https://github.com/upbound/project-template-aws-s3",
	"Kubernetes WebApp":  "https://github.com/upbound/project-template-k8s-webapp",
	"Start from scratch": BlankProjectTemplate,
}

// AIToolingProvider represents the AI tooling providers supported for generation.
type AIToolingProvider string

const (
	// ToolGemini represents the gemini-cli.
	ToolGemini AIToolingProvider = "gemini-cli"
	// ToolClaude represents claude-code.
	ToolClaude AIToolingProvider = "claude-code"
	// ToolCursor represents cursor.
	ToolCursor AIToolingProvider = "cursor"
)

// SupportedAIToolingProvidersMap contains tools supported for projects.
var SupportedAIToolingProvidersMap = map[string]AIToolingProvider{ //nolint:gochecknoglobals // this is a constant
	"gemini-cli":  ToolGemini,
	"claude-code": ToolClaude,
	"cursor":      ToolCursor,
}

const (
	// StepContinue indicates the wizard should continue to the next step.
	StepContinue = iota
	// StepUseTemplate indicates the wizard is asking about using an template.
	StepUseTemplate
	// StepChooseTemplateLanguage indicates the wizard is asking for template language.
	StepChooseTemplateLanguage
	// StepChooseTemplateTestLanguage indicates the wizard is asking for template test language.
	StepChooseTemplateTestLanguage
	// StepKind indicates the wizard is asking for the kind of the resource.
	StepKind
	// StepAPIGroup indicates the wizard is asking for API group.
	StepAPIGroup
	// StepAPIVersion indicates the wizard is asking for API version.
	StepAPIVersion
	// StepXRScope indicates the wizard is asking about the scope of the XR.
	StepXRScope
	// StepGenerateXRD indicates the wizard is asking about XRD generation.
	StepGenerateXRD
	// StepGenerateComp indicates the wizard is asking about composition generation.
	StepGenerateComp
	// StepGenerateFunction indicates the wizard is asking about function generation.
	StepGenerateFunction
	// StepFuncLang indicates the wizard is asking for function language.
	StepFuncLang
	// StepGenerateTest indicates the wizard is asking about test generation.
	StepGenerateTest
	// StepTestLang indicates the wizard is asking for test language.
	StepTestLang
	// StepAITooling indicates the wizard is asking for AI tooling.
	StepAITooling
	// StepAIToolingChoice indicates the wizard is asking for which provider
	// (gemini, etc).
	StepAIToolingChoice
	// StepFinished indicates the wizard has completed all steps.
	StepFinished
)

// State stores the progress and inputs of the wizard.
type State struct {
	Step     int    `json:"step"`
	Template string `json:"template"`

	Kind          string `json:"kind"`
	APIGroup      string `json:"apiGroup"`
	APIVersion    string `json:"apiVersion"`
	ClusterScoped bool   `json:"clusterScoped"`

	MetadataName      string `json:"metadataName"`
	MetadataNamespace string `json:"metadataNamespace"`

	GenerateXRD       bool `json:"generateXrd"`
	GenerateComp      bool `json:"generateComp"`
	GenerateFunction  bool `json:"generateFunction"`
	GenerateTest      bool `json:"generateTest"`
	GenerateAITooling bool `json:"generateAiTooling"`

	FuncLang FunctionLanguage `json:"funcLang"`
	TestLang FunctionLanguage `json:"testLang"`

	AITooling []AIToolingProvider `json:"aiTooling"`
}

// defaultState returns a new State with default values.
func defaultState() State {
	return State{
		Step: StepContinue,

		Kind:          "Example",
		APIGroup:      "example.upbound.io",
		APIVersion:    "v1alpha1",
		ClusterScoped: false,

		MetadataName:      "example",
		MetadataNamespace: "default",

		GenerateXRD:      false,
		GenerateComp:     false,
		GenerateFunction: false,
		GenerateTest:     false,

		FuncLang: FunctionLanguageKCL,
		TestLang: FunctionLanguageKCL,
	}
}

// SaveState saves the wizard state to disk.
func SaveState(state State, statePath string) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath, data, 0o600)
}

// LoadState loads the wizard state from disk.
func LoadState(statePath string) (State, error) {
	var state State
	data, err := os.ReadFile(statePath) //nolint:gosec // this is a wizard state file
	if err != nil {
		return state, err
	}
	err = json.Unmarshal(data, &state)
	return state, err
}

// deleteState removes the saved wizard state file.
func deleteState(statePath string) {
	_ = os.Remove(statePath)
}

func collectOptions[T any](from map[string]T) []string {
	options := slices.Collect(maps.Keys(from))
	slices.Sort(options)
	return options
}

// askUser is the main function that runs the wizard. It handles the user's
// input and updates the state accordingly.
func askUser(state *State, statePath string) error { //nolint:gocognit // this is a state machine
	for state.Step < StepFinished {
		navigated := false

		var err error
		switch state.Step {
		case StepUseTemplate:
			choice, err := pterm.DefaultInteractiveSelect.WithOptions(collectOptions(availableTemplates)).Show("Would you like to use an existing template?")
			if err != nil {
				return err
			}
			template := availableTemplates[choice]
			state.Template = template
			if template == BlankProjectTemplate {
				state.Step = StepKind
				navigated = true
			}
		case StepChooseTemplateLanguage:
			result, err := pterm.DefaultInteractiveSelect.WithOptions(collectOptions(SupportedLanguagesMap)).Show("Select language used for composition functions")
			if err != nil {
				return err
			}
			state.FuncLang = SupportedLanguagesMap[result]
		case StepChooseTemplateTestLanguage:
			result, err := pterm.DefaultInteractiveSelect.WithOptions(collectOptions(SupportedTestLanguagesMap)).Show("Select language used for tests")
			if err != nil {
				return err
			}
			state.TestLang = SupportedTestLanguagesMap[result]
			state.Step = StepAITooling
			navigated = true
		case StepKind:
			val, err := pterm.DefaultInteractiveTextInput.WithDefaultValue(state.Kind).Show("Kind of the resource")
			if err != nil {
				return err
			}
			state.Kind = val
		case StepAPIGroup:
			val, err := pterm.DefaultInteractiveTextInput.WithDefaultValue(state.APIGroup).Show("API Group")
			if err != nil {
				return err
			}
			state.APIGroup = val
		case StepAPIVersion:
			val, err := pterm.DefaultInteractiveTextInput.WithDefaultValue(state.APIVersion).Show("API Version")
			if err != nil {
				return err
			}
			state.APIVersion = val
		case StepXRScope:
			result, err := pterm.DefaultInteractiveSelect.WithOptions([]string{"Namespace", "Cluster"}).Show("Should the XRs be namespace- or cluster-scoped?")
			if err != nil {
				return err
			}
			state.ClusterScoped = result == "Cluster"
		case StepGenerateXRD:
			state.GenerateXRD, err = pterm.DefaultInteractiveConfirm.WithDefaultValue(true).Show("Generate XRD?")
			if err != nil {
				return err
			}
			if !state.GenerateXRD {
				state.Step = StepGenerateTest
				navigated = true
			}
		case StepGenerateComp:
			state.GenerateComp, err = pterm.DefaultInteractiveConfirm.WithDefaultValue(true).Show("Generate Composition?")
			if err != nil {
				return err
			}
			if !state.GenerateComp {
				state.Step = StepGenerateTest
				navigated = true
			}
		case StepGenerateFunction:
			state.GenerateFunction, err = pterm.DefaultInteractiveConfirm.WithDefaultValue(true).Show("Generate Function?")
			if err != nil {
				return err
			}
			if !state.GenerateFunction {
				state.Step = StepGenerateTest
				navigated = true
			}
		case StepFuncLang:
			result, err := pterm.DefaultInteractiveSelect.WithOptions(collectOptions(SupportedLanguagesMap)).Show("Select composition function language")
			if err != nil {
				return err
			}
			state.FuncLang = SupportedLanguagesMap[result]
		case StepGenerateTest:
			state.GenerateTest, err = pterm.DefaultInteractiveConfirm.WithDefaultValue(true).Show("Generate test?")
			if err != nil {
				return err
			}
			if !state.GenerateTest {
				state.Step = StepFinished
				navigated = true
			}
		case StepTestLang:
			result, err := pterm.DefaultInteractiveSelect.WithOptions(collectOptions(SupportedTestLanguagesMap)).Show("Select test language")
			if err != nil {
				return err
			}
			state.TestLang = SupportedTestLanguagesMap[result]
		case StepAITooling:
			state.GenerateAITooling, err = pterm.DefaultInteractiveConfirm.WithDefaultValue(true).Show("Generate AI Tooling Configurations?")
			if err != nil {
				return err
			}
			if !state.GenerateAITooling {
				state.Step = StepFinished
				navigated = true
			}
		case StepAIToolingChoice:
			result, err := pterm.DefaultInteractiveMultiselect.WithOptions(collectOptions(SupportedAIToolingProvidersMap)).Show("Select the tooling provider(s)")
			if err != nil {
				return err
			}

			for _, r := range result {
				state.AITooling = append(state.AITooling, SupportedAIToolingProvidersMap[r])
			}

			state.Step = StepFinished
			navigated = true
		}

		if !navigated {
			state.Step++
		}

		if err := SaveState(*state, statePath); err != nil {
			return err
		}
	}
	return nil
}
