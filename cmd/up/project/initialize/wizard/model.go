// Copyright 2025 Upbound Inc.
// All rights reserved

package wizard

import (
	"encoding/json"
	"maps"
	"os"
	"slices"

	"github.com/upbound/up/internal/upterm"
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

// BlankProjectTemplate is the URL of the blank project template.
const BlankProjectTemplate = "https://github.com/upbound/project-template-scratch"

// availableTemplates maps template names to their repository URLs.
var availableTemplates = map[string]string{ //nolint:gochecknoglobals // this is a constant
	"AWS Bucket":         "https://github.com/upbound/project-template-aws-s3",
	"Azure Storage":      "https://github.com/upbound/project-template-azure-storage",
	"GCP Storage":        "https://github.com/upbound/project-template-gcp-storage",
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
	// ToolCopilot represents GitHub Copilot.
	ToolCopilot AIToolingProvider = "copilot"
)

// SupportedAIToolingProvidersMap contains tools supported for projects.
var SupportedAIToolingProvidersMap = map[string]AIToolingProvider{ //nolint:gochecknoglobals // this is a constant
	"gemini-cli":  ToolGemini,
	"claude-code": ToolClaude,
	"cursor":      ToolCursor,
	"copilot":     ToolCopilot,
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
			opts := collectOptions(availableTemplates)
			choice, err := upterm.Selection("Would you like to use an existing template?", opts, opts[0])
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
			opts := collectOptions(SupportedLanguagesMap)
			result, err := upterm.Selection("Select language used for composition functions", opts, opts[0])
			if err != nil {
				return err
			}
			state.FuncLang = SupportedLanguagesMap[result]
		case StepChooseTemplateTestLanguage:
			// Set the default to the same language the user selected for
			// functions, but fall back to the first option if needed.
			opts := collectOptions(SupportedLanguagesMap)
			def := opts[0]
			for opt, lang := range SupportedLanguagesMap {
				if state.FuncLang == lang {
					def = opt
					break
				}
			}

			result, err := upterm.Selection("Select language used for tests", opts, def)
			if err != nil {
				return err
			}
			state.TestLang = SupportedLanguagesMap[result]
			state.Step = StepAITooling
			navigated = true
		case StepKind:
			val, err := upterm.Prompt("Kind of the resource", state.Kind)
			if err != nil {
				return err
			}
			state.Kind = val
		case StepAPIGroup:
			val, err := upterm.Prompt("API Group", state.APIGroup)
			if err != nil {
				return err
			}
			state.APIGroup = val
		case StepAPIVersion:
			val, err := upterm.Prompt("API Version", state.APIVersion)
			if err != nil {
				return err
			}
			state.APIVersion = val
		case StepXRScope:
			result, err := upterm.Selection("Should the XRs be namespace- or cluster-scoped?", []string{"Namespace", "Cluster"}, "Namespace")
			if err != nil {
				return err
			}
			state.ClusterScoped = result == "Cluster"
		case StepGenerateXRD:
			state.GenerateXRD, err = upterm.Confirm("Generate XRD?", true)
			if err != nil {
				return err
			}
			if !state.GenerateXRD {
				state.Step = StepGenerateTest
				navigated = true
			}
		case StepGenerateComp:
			state.GenerateComp, err = upterm.Confirm("Generate Composition?", true)
			if err != nil {
				return err
			}
			if !state.GenerateComp {
				state.Step = StepGenerateTest
				navigated = true
			}
		case StepGenerateFunction:
			state.GenerateFunction, err = upterm.Confirm("Generate Function?", true)
			if err != nil {
				return err
			}
			if !state.GenerateFunction {
				state.Step = StepGenerateTest
				navigated = true
			}
		case StepFuncLang:
			opts := collectOptions(SupportedLanguagesMap)
			result, err := upterm.Selection("Select composition function language", opts, opts[0])
			if err != nil {
				return err
			}
			state.FuncLang = SupportedLanguagesMap[result]
		case StepGenerateTest:
			state.GenerateTest, err = upterm.Confirm("Generate test?", true)
			if err != nil {
				return err
			}
			if !state.GenerateTest {
				state.Step = StepFinished
				navigated = true
			}
		case StepTestLang:
			opts := collectOptions(SupportedLanguagesMap)
			result, err := upterm.Selection("Select test language", opts, opts[0])
			if err != nil {
				return err
			}
			state.TestLang = SupportedLanguagesMap[result]
		case StepAITooling:
			state.GenerateAITooling, err = upterm.Confirm("Generate AI Tooling Configurations?", true)
			if err != nil {
				return err
			}
			if !state.GenerateAITooling {
				state.Step = StepFinished
				navigated = true
			}
		case StepAIToolingChoice:
			opts := collectOptions(SupportedAIToolingProvidersMap)
			result, err := upterm.MultiSelection("Select the tooling provider(s)", opts, nil)
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
