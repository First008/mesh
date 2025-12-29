package agent

import (
	"fmt"
	"strings"
)

// Personality defines the agent's behavior and expertise
type Personality struct {
	Name         string
	Repo         string
	SystemPrompt string
}

// NewPersonality creates a personality from the configuration
func NewPersonality(repoName, personalityText string, focusPaths []string) *Personality {
	if personalityText == "" {
		personalityText = fmt.Sprintf("You are an expert software engineer familiar with the %s codebase.", repoName)
	}

	return &Personality{
		Name:         fmt.Sprintf("%s-agent", repoName),
		Repo:         repoName,
		SystemPrompt: buildSystemPrompt(personalityText, focusPaths),
	}
}

// buildSystemPrompt constructs the full system prompt for the agent
func buildSystemPrompt(personalityText string, focusPaths []string) string {
	var b strings.Builder

	// Core personality first
	b.WriteString(personalityText)
	b.WriteString("\n\n")

	// Critical constraint - short and forceful
	b.WriteString("## CRITICAL: No fabrication\n\n")
	b.WriteString("- NEVER invent file paths, function names, or code not shown in the context below\n")
	b.WriteString("- You MAY use general Go/language knowledge to explain code that IS shown\n")
	b.WriteString("- If a file is not in context, say so: \"That file is not in my context\"\n\n")

	// Instructions - concise
	b.WriteString("## Guidelines\n\n")
	b.WriteString("1. **Reference files** by name; include line numbers only if provided in the context\n")
	b.WriteString("2. **Quote minimal snippets** - just enough to support your point\n")
	b.WriteString("3. **Admit gaps** - if context is missing, say what you'd need to answer fully\n")
	b.WriteString("4. **Be concise** - short answers unless detail is requested\n\n")

	// Focus paths if provided
	if len(focusPaths) > 0 {
		b.WriteString("## Focus Areas\n\n")
		for _, path := range focusPaths {
			b.WriteString(fmt.Sprintf("- `%s`\n", path))
		}
		b.WriteString("\n")
	}

	// Formatting - minimal
	b.WriteString("## Format\n\n")
	b.WriteString("- Markdown for code blocks\n")
	b.WriteString("- Bullets for lists\n")
	b.WriteString("- Most important code first\n")

	return b.String()
}

// GetSystemPrompt returns the full system prompt
func (p *Personality) GetSystemPrompt() string {
	return p.SystemPrompt
}
