// Copyright 2025 Upbound Inc.
// All rights reserved

package style

import (
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Help text styles.
	//nolint:gochecknoglobals // We'd make these consts if we could.
	helpHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(UpboundBrandColor)

	//nolint:gochecknoglobals // We'd make these consts if we could.
	helpSubheaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(NeutralColor)

	//nolint:gochecknoglobals // We'd make these consts if we could.
	helpInlineCodeStyle = lipgloss.NewStyle().
				Foreground(UpboundBrandColor)

	//nolint:gochecknoglobals // We'd make these consts if we could.
	helpBulletStyle = lipgloss.NewStyle().
			Foreground(UpboundBrandColor)

	//nolint:gochecknoglobals // We'd make these consts if we could.
	helpEmphasisStyle = lipgloss.NewStyle().
				Italic(true).
				Foreground(NeutralColor)

	//nolint:gochecknoglobals // We'd make these consts if we could.
	helpLinkStyle = lipgloss.NewStyle().
			Underline(true).
			Foreground(UpboundBrandColor)
)

// - Bullet points: Lines starting with "- ".
func FormatHelp(help string) string {
	lines := strings.Split(strings.TrimSpace(help), "\n")
	result := make([]string, 0, len(lines))
	inExampleSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			result = append(result, "")
			continue
		}

		// Main header
		if strings.HasPrefix(trimmed, "# ") {
			header := strings.TrimPrefix(trimmed, "# ")
			result = append(result, helpHeaderStyle.Render(header))
			inExampleSection = false
			continue
		}

		// Subheader
		if strings.HasPrefix(trimmed, "## ") {
			subheader := strings.TrimPrefix(trimmed, "## ")
			result = append(result, helpSubheaderStyle.Render(subheader))
			// Check if this is an Examples section
			inExampleSection = strings.Contains(strings.ToLower(subheader), "example")
			continue
		}

		// Bullet points
		if strings.HasPrefix(trimmed, "- ") {
			bullet := helpBulletStyle.Render("•")
			content := strings.TrimPrefix(trimmed, "- ")
			content = formatInlineElements(content)
			result = append(result, "  "+bullet+" "+content)
			continue
		}

		// Handle indented lines in example section
		if inExampleSection && (strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t")) {
			// Keep original indentation but apply inline formatting
			leadingSpaces := line[:len(line)-len(trimmed)]
			result = append(result, leadingSpaces+formatInlineElements(trimmed))
			continue
		}

		// Regular text with inline formatting
		result = append(result, formatInlineElements(line))
	}

	return strings.Join(result, "\n")
}

// formatInlineElements handles inline markdown elements like `code`, *emphasis*, <params>, and [links](url).
func formatInlineElements(text string) string {
	// Handle angle brackets <param> - highlight in purple
	angleRegex := regexp.MustCompile(`<([^>]+)>`)
	text = angleRegex.ReplaceAllStringFunc(text, func(match string) string {
		param := strings.Trim(match, "<>")
		return helpInlineCodeStyle.Render(param)
	})

	// Handle inline code (backticks)
	codeRegex := regexp.MustCompile("`([^`]+)`")
	text = codeRegex.ReplaceAllStringFunc(text, func(match string) string {
		code := strings.Trim(match, "`")
		return helpInlineCodeStyle.Render(code)
	})

	// Handle emphasis (asterisks)
	emphasisRegex := regexp.MustCompile(`\*([^*]+)\*`)
	text = emphasisRegex.ReplaceAllStringFunc(text, func(match string) string {
		emphasis := strings.Trim(match, "*")
		return helpEmphasisStyle.Render(emphasis)
	})

	// Handle links [text](url)
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	text = linkRegex.ReplaceAllStringFunc(text, func(match string) string {
		parts := linkRegex.FindStringSubmatch(match)
		if len(parts) >= 3 {
			linkText := parts[1]
			// We can't make clickable links in terminal, so just style the text
			return helpLinkStyle.Render(linkText)
		}
		return match
	})

	return text
}

// RenderHelp is a convenience function that takes a traditional help string
// and returns a formatted version. Commands can use this in their Help() method.
func RenderHelp(help string) string {
	// Check if NO_COLOR is set to disable styling
	if os.Getenv("NO_COLOR") != "" {
		return strings.TrimSpace(help)
	}

	return FormatHelp(help)
}
