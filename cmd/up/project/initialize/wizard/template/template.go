// Copyright 2025 Upbound Inc.
// All rights reserved

// Package template implements utility routines for fetching and initializing projects from project templates.
package template

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/upbound/up/internal/git"
	"github.com/upbound/up/internal/upterm"
)

// templateConfig defines the structure of template metadata.
type templateConfig struct {
	Name        string                `yaml:"name"`
	Description string                `yaml:"description"`
	Version     string                `yaml:"version"`
	Languages   []string              `yaml:"languages"`
	Files       map[string]fileConfig `yaml:"files"`
	Values      map[string]string     `yaml:"values"`
	Rename      renameConfig          `yaml:"rename"`
}

type fileConfig struct {
	Languages []string `yaml:"languages"`
	Template  bool     `yaml:"template"`
	Required  bool     `yaml:"required"`
	Rename    string   `yaml:"rename,omitempty"`
}

type renameConfig struct {
	Directories []directoryRename `yaml:"directories"`
	Files       []fileRename      `yaml:"files"`
}

type directoryRename struct {
	Pattern     string   `yaml:"pattern"`
	Replacement string   `yaml:"replacement"`
	Languages   []string `yaml:"languages"`
}

type fileRename struct {
	Pattern     string   `yaml:"pattern"`
	Replacement string   `yaml:"replacement"`
	Languages   []string `yaml:"languages"`
}

// Cloner handles the complete cloning and transformation process.
type Cloner struct {
	templateURL     RepoURL
	targetDir       string
	language        string
	testLanguage    string
	debug           bool
	tempDir         string
	values          map[string]string
	config          *templateConfig
	gitCloner       git.Cloner
	gitAuthProvider git.AuthProvider
	printer         upterm.Printer
	fs              afero.Fs
	tempFs          afero.Fs
}

// NewCloner creates a new template cloner.
func NewCloner(templateURL RepoURL, targetDir, language, testLanguage string, values map[string]string, gitCloner git.Cloner, gitAuthProvider git.AuthProvider, printer upterm.Printer, debug bool) *Cloner {
	targetBasePathFs := afero.NewBasePathFs(afero.NewOsFs(), targetDir)
	return &Cloner{
		templateURL:     templateURL,
		targetDir:       targetDir,
		language:        language,
		testLanguage:    testLanguage,
		debug:           debug,
		values:          values,
		gitCloner:       gitCloner,
		gitAuthProvider: gitAuthProvider,
		printer:         printer,
		fs:              targetBasePathFs,
		tempFs:          nil, // Will be set in cloneRepository
	}
}

// CloneAndTransform performs the complete clone and transform operation.
func (c *Cloner) CloneAndTransform() (*plumbing.Reference, error) {
	// 1. Clone the repository
	ref, err := c.cloneRepository()
	if err != nil {
		return nil, errors.Wrap(err, "failed to clone repository")
	}
	defer c.cleanup()

	// 2. Load template configuration
	if err := c.loadConfig(); err != nil {
		return nil, errors.Wrap(err, "failed to load template config")
	}

	// 3. Validate language support
	if !c.isLanguageSupported() {
		return nil, fmt.Errorf("language %s not supported by template %s", c.language, c.config.Name)
	}

	// 4. Create target directory
	if err := c.fs.MkdirAll(".", 0o755); err != nil {
		return nil, errors.Wrap(err, "failed to create target directory")
	}

	// 5. Process template files
	exists, err := afero.DirExists(c.tempFs, "template")
	if err != nil {
		return nil, errors.Wrap(err, "failed to check template directory")
	}
	if !exists {
		return nil, fmt.Errorf("template directory not found in repository")
	}

	if err := c.processFiles(); err != nil {
		return nil, errors.Wrap(err, "failed to process template files")
	}

	return ref, nil
}

// cloneRepository clones the template repository to a temporary directory.
func (c *Cloner) cloneRepository() (*plumbing.Reference, error) {
	tempDir, err := afero.TempDir(afero.NewOsFs(), "", "project-template")
	if err != nil {
		return nil, err
	}
	c.tempDir = tempDir

	// Initialize tempFs with the base path
	c.tempFs = afero.NewBasePathFs(afero.NewOsFs(), tempDir)

	c.debugf("Cloning template repository %s...", c.templateURL)

	ref, err := c.gitCloner.CloneRepository(memory.NewStorage(), osfs.New(tempDir, osfs.WithBoundOS()), c.gitAuthProvider, git.CloneOptions{
		Repo:      c.templateURL.URL,
		RefName:   c.templateURL.Ref,
		Directory: tempDir,
	})
	if err != nil {
		return nil, err
	}

	return ref, nil
}

// loadConfig loads the template configuration from template.yaml.
func (c *Cloner) loadConfig() error {
	configPath := "template.yaml"
	data, err := afero.ReadFile(c.tempFs, configPath)
	if err != nil {
		return errors.Wrap(err, "template.yaml not found")
	}

	c.config = &templateConfig{}
	return yaml.Unmarshal(data, c.config)
}

// isLanguageSupported checks if the specified language is supported.
func (c *Cloner) isLanguageSupported() bool {
	return slices.Contains(c.config.Languages, c.language)
}

// processFiles walks through template files and processes them.
func (c *Cloner) processFiles() error {
	c.debugf("Processing template for language: %s", c.language)

	return afero.Walk(c.tempFs, "template", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root template directory
		if path == "template" {
			return nil
		}

		// Get relative path from template directory
		relPath, err := filepath.Rel("template", path)
		if err != nil {
			return err
		}

		// Check if this file/directory should be included
		if !c.shouldInclude(relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Determine target path with renaming
		targetRelPath := c.applyRenames(relPath)

		if info.IsDir() {
			c.debugf("Creating directory: %s", targetRelPath)
			return c.fs.MkdirAll(targetRelPath, info.Mode())
		}

		c.debugf("Processing file: %s → %s", relPath, targetRelPath)
		return c.processFile(path, targetRelPath, relPath)
	})
}

// shouldInclude determines if a file/directory should be included.
func (c *Cloner) shouldInclude(relPath string) bool {
	// Check explicit file configuration first
	if fileConfig, exists := c.config.Files[relPath]; exists {
		return c.isFileIncludedByConfig(fileConfig)
	}

	// Check for language suffixes in the path
	pathParts := strings.Split(relPath, string(filepath.Separator))
	isTestDir := pathParts[0] == "tests"
	for _, part := range pathParts {
		if c.hasLanguageSuffix(part) {
			if isTestDir {
				return c.matchesCurrentTestLanguage(part)
			}
			return c.matchesCurrentLanguage(part)
		}
	}

	// No language suffixes found - it's a common file/directory
	return true
}

// isFileIncludedByConfig checks explicit file configuration.
func (c *Cloner) isFileIncludedByConfig(fileConfig fileConfig) bool {
	return len(fileConfig.Languages) == 0 ||
		slices.Contains(fileConfig.Languages, c.language)
}

// hasLanguageSuffix checks if a directory name has any language suffix.
func (c *Cloner) hasLanguageSuffix(dirName string) bool {
	for _, lang := range c.config.Languages {
		if strings.HasSuffix(dirName, "-"+lang) {
			return true
		}
	}
	return false
}

// matchesCurrentLanguage checks if directory name matches current language.
func (c *Cloner) matchesCurrentLanguage(dirName string) bool {
	return strings.HasSuffix(dirName, "-"+c.language)
}

// matchesCurrentTestLanguage checks if directory name matches current test language.
func (c *Cloner) matchesCurrentTestLanguage(dirName string) bool {
	return strings.HasSuffix(dirName, "-"+c.testLanguage)
}

// applyRenames applies directory and file rename patterns.
func (c *Cloner) applyRenames(relPath string) string {
	// Apply directory renames
	targetPath := c.applyDirectoryRenames(relPath)

	// Apply file renames
	targetPath = c.applyFileRenames(targetPath)

	// Apply explicit file renames from config
	if fileConfig, exists := c.config.Files[relPath]; exists && fileConfig.Rename != "" {
		dir := filepath.Dir(targetPath)
		if dir == "." {
			return fileConfig.Rename
		}
		return filepath.Join(dir, fileConfig.Rename)
	}

	return targetPath
}

// applyDirectoryRenames applies directory rename patterns.
func (c *Cloner) applyDirectoryRenames(targetPath string) string {
	parts := strings.Split(targetPath, string(filepath.Separator))

	for i, part := range parts {
		for _, rename := range c.config.Rename.Directories {
			if len(rename.Languages) > 0 && !c.containsLanguage(rename.Languages) && !c.containsTestLanguage(rename.Languages) {
				continue
			}

			if newPart := c.applyPatternReplace(part, rename.Pattern, rename.Replacement); newPart != part {
				parts[i] = newPart
				break
			}
		}
	}

	return strings.Join(parts, string(filepath.Separator))
}

// applyFileRenames applies file rename patterns.
func (c *Cloner) applyFileRenames(targetPath string) string {
	fileName := filepath.Base(targetPath)
	dir := filepath.Dir(targetPath)

	for _, rename := range c.config.Rename.Files {
		if len(rename.Languages) > 0 && !c.containsLanguage(rename.Languages) && !c.containsTestLanguage(rename.Languages) {
			continue
		}

		if newFileName := c.applyPatternReplace(fileName, rename.Pattern, rename.Replacement); newFileName != fileName {
			return filepath.Join(dir, newFileName)
		}
	}

	return targetPath
}

// applyPatternReplace applies pattern-based string replacement.
func (c *Cloner) applyPatternReplace(input, pattern, replacement string) string {
	if !strings.Contains(pattern, "*") && pattern == input {
		return replacement
	}

	// Find the last asterisk position
	lastAsteriskIndex := strings.LastIndex(pattern, "*")
	if lastAsteriskIndex == -1 {
		return input
	}

	prefix := pattern[:lastAsteriskIndex]
	suffix := pattern[lastAsteriskIndex+1:]

	if strings.HasPrefix(input, prefix) && strings.HasSuffix(input, suffix) {
		middle := input[len(prefix) : len(input)-len(suffix)]

		return strings.Replace(replacement, "*", middle, 1)
	}

	return input
}

// containsLanguage checks if current language is in the list.
func (c *Cloner) containsLanguage(languages []string) bool {
	return slices.Contains(languages, c.language)
}

// containsTestLanguage checks if current test language is in the list.
func (c *Cloner) containsTestLanguage(languages []string) bool {
	return slices.Contains(languages, c.testLanguage)
}

// processFile processes a single file (template rendering or copying).
func (c *Cloner) processFile(srcPath, targetPath, relPath string) error {
	// Check if this is a template file
	isTemplate := c.isTemplateFile(srcPath, relPath)

	if isTemplate {
		// Remove .template extension if present
		targetPath = strings.TrimSuffix(targetPath, ".template")
		return c.processTemplateFile(srcPath, targetPath)
	}

	return c.copyFile(srcPath, targetPath)
}

// isTemplateFile determines if a file should be processed as a template.
func (c *Cloner) isTemplateFile(srcPath, relPath string) bool {
	// Check explicit configuration
	if fileConfig, exists := c.config.Files[relPath]; exists {
		return fileConfig.Template
	}

	// Check file extension
	return strings.HasSuffix(srcPath, ".template")
}

// processTemplateFile processes a template file with variable substitution.
func (c *Cloner) processTemplateFile(srcPath, targetPath string) error {
	// Read template content
	tmplData, err := afero.ReadFile(c.tempFs, srcPath)
	if err != nil {
		return err
	}

	tmpl := template.New(filepath.Base(srcPath))

	// Sprig's env and expandenv can lead to information leakage (injected tokens/passwords).
	// Both Helm and ArgoCD remove these due to security implications.
	// see: https://masterminds.github.io/sprig/os.html
	sprigFuncs := sprig.FuncMap()
	delete(sprigFuncs, "env")
	delete(sprigFuncs, "expandenv")
	tmpl.Funcs(sprigFuncs)

	tmpl, err = tmpl.Parse(string(tmplData))
	if err != nil {
		return errors.Wrapf(err, "failed to parse template %s", srcPath)
	}

	// Prepare template variables
	variables := c.getTemplateVariables()

	// Create target file
	if err := c.fs.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	targetFile, err := c.fs.Create(targetPath)
	if err != nil {
		return err
	}
	defer targetFile.Close() //nolint:errcheck // nothing to do here

	// Execute template
	return tmpl.Execute(targetFile, variables)
}

// getTemplateVariables returns variables for template processing.
func (c *Cloner) getTemplateVariables() map[string]any {
	variables := make(map[string]any)
	values := make(map[string]any)

	// Add configured variables
	for key, value := range c.config.Values {
		values[key] = value
	}

	// Add user-provided values
	for key, value := range c.values {
		values[key] = value
	}

	// Add runtime variables
	variables["Language"] = c.language
	variables["ProjectName"] = filepath.Base(c.targetDir)
	variables["TemplateName"] = c.config.Name
	variables["TemplateVersion"] = c.config.Version
	variables["Values"] = values

	return variables
}

// copyFile copies a file from source to destination.
func (c *Cloner) copyFile(srcPath, targetPath string) error {
	// Check if source is a symlink
	tempBasePathFs, ok := c.tempFs.(*afero.BasePathFs)
	if !ok {
		return errors.Errorf("Unexpected filesystem type")
	}
	srcInfo, _, err := tempBasePathFs.LstatIfPossible(srcPath)
	if err != nil {
		return err
	}

	if srcInfo.Mode()&os.ModeSymlink != 0 {
		// For symlinks, we'll use the OS filesystem directly
		target, err := os.Readlink(filepath.Join(c.tempDir, srcPath))
		if err != nil {
			return err
		}

		// Create target directory if it doesn't exist
		if err := c.fs.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		// Create the symlink in the target location
		return os.Symlink(target, filepath.Join(c.targetDir, targetPath))
	}

	// Regular file handling
	srcFile, err := c.tempFs.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close() //nolint:errcheck // nothing to do here

	// Create target directory
	if err := c.fs.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	targetFile, err := c.fs.Create(targetPath)
	if err != nil {
		return err
	}
	defer targetFile.Close() //nolint:errcheck // nothing to do here

	_, err = io.Copy(targetFile, srcFile)
	return err
}

// cleanup removes the temporary directory.
func (c *Cloner) cleanup() {
	if c.tempDir != "" {
		os.RemoveAll(c.tempDir) //nolint:errcheck // nothing to do here
	}
}

func (c *Cloner) debugf(format string, a ...any) {
	if c.debug {
		c.printer.Printfln(format, a...)
	}
}

// RepoURL contains both the repository URL and its reference.
type RepoURL struct {
	URL string
	Ref string
}

// ResolveTemplateURL parses the repository url reference if specified in repo@ref format.
func ResolveTemplateURL(template string) RepoURL {
	repo := template
	ref := "main"

	// the template might contain an @ if it's an ssh url.
	// Split on the first @ after the last slash to detect if we have a ref.
	lastSlash := strings.LastIndex(template, "/")
	sep := strings.LastIndex(template, "@")
	if sep > lastSlash {
		repo = template[:sep]
		ref = template[sep+1:]
	}

	switch {
	case strings.HasPrefix(repo, ".") || strings.HasPrefix(repo, "/"):
		// default to HEAD instead of main for local repos.
		if sep == -1 {
			ref = "HEAD"
		}

		// Local git repo - return a file URL.
		path, _ := filepath.Abs(repo)
		return RepoURL{URL: fmt.Sprintf("file://%s", path), Ref: ref}

	case strings.Contains(repo, "://") || IsSSHShortURL(repo):
		// Already a full URL, return as-is.
		return RepoURL{URL: repo, Ref: ref}

	case strings.Contains(repo, "/"):
		// Partially-qualified: assume github.com/org/repo.
		return RepoURL{
			URL: fmt.Sprintf("https://github.com/%s.git", repo),
			Ref: ref,
		}

	default:
		// Default to upbound organization.
		return RepoURL{
			URL: fmt.Sprintf("https://github.com/upbound/%s.git", repo),
			Ref: ref,
		}
	}
}

// IsSSHShortURL checks if the input url is an scp-style ssh url.
// ssh urls can be structured as [<user>@]<host>:/<path-to-git-repo>, recognized as no slashes before the first colon.
func IsSSHShortURL(url string) bool {
	colon := strings.Index(url, ":")
	slash := strings.Index(url, "/")

	return colon > -1 && slash > colon
}
