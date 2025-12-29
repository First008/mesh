package agent

import (
	"strings"
	"testing"
)

func TestNewPersonality_DefaultPersonality(t *testing.T) {
	p := NewPersonality("test-repo", "", nil)

	if p == nil {
		t.Fatal("Expected personality, got nil")
	}

	if p.Name != "test-repo-agent" {
		t.Errorf("Expected name 'test-repo-agent', got '%s'", p.Name)
	}

	if p.Repo != "test-repo" {
		t.Errorf("Expected repo 'test-repo', got '%s'", p.Repo)
	}

	if p.SystemPrompt == "" {
		t.Error("Expected non-empty system prompt")
	}

	// Default text should be included
	if !strings.Contains(p.SystemPrompt, "test-repo") {
		t.Error("System prompt should mention the repo name")
	}
}

func TestNewPersonality_CustomPersonality(t *testing.T) {
	customText := "You are a specialized Go expert for distributed systems."
	p := NewPersonality("crypto-repo", customText, nil)

	if p == nil {
		t.Fatal("Expected personality, got nil")
	}

	if !strings.Contains(p.SystemPrompt, customText) {
		t.Errorf("System prompt should contain custom personality text")
	}
}

func TestNewPersonality_WithFocusPaths(t *testing.T) {
	focusPaths := []string{"internal/", "pkg/core/"}
	p := NewPersonality("test-repo", "", focusPaths)

	if p == nil {
		t.Fatal("Expected personality, got nil")
	}

	if !strings.Contains(p.SystemPrompt, "Focus Areas") {
		t.Error("System prompt should contain focus areas section")
	}

	for _, path := range focusPaths {
		if !strings.Contains(p.SystemPrompt, path) {
			t.Errorf("System prompt should contain focus path '%s'", path)
		}
	}
}

func TestNewPersonality_EmptyRepoName(t *testing.T) {
	p := NewPersonality("", "", nil)

	if p == nil {
		t.Fatal("Expected personality, got nil")
	}

	if p.Name != "-agent" {
		t.Errorf("Expected name '-agent', got '%s'", p.Name)
	}

	if p.Repo != "" {
		t.Errorf("Expected empty repo, got '%s'", p.Repo)
	}
}

func TestGetSystemPrompt(t *testing.T) {
	p := NewPersonality("test-repo", "Custom personality", nil)

	prompt := p.GetSystemPrompt()

	if prompt == "" {
		t.Error("GetSystemPrompt should return non-empty string")
	}

	if prompt != p.SystemPrompt {
		t.Error("GetSystemPrompt should return the SystemPrompt field")
	}
}

func TestSystemPrompt_ContainsCriticalRule(t *testing.T) {
	p := NewPersonality("test-repo", "", nil)

	if !strings.Contains(p.SystemPrompt, "CRITICAL") {
		t.Error("System prompt should contain CRITICAL section")
	}

	if !strings.Contains(p.SystemPrompt, "NEVER invent") {
		t.Error("System prompt should warn against inventing code")
	}
}

func TestSystemPrompt_AllowsGeneralKnowledge(t *testing.T) {
	p := NewPersonality("test-repo", "", nil)

	// Should explicitly allow using general language knowledge to explain shown code
	if !strings.Contains(p.SystemPrompt, "MAY use general") {
		t.Error("System prompt should allow general knowledge for explaining shown code")
	}
}

func TestSystemPrompt_ContainsGuidelines(t *testing.T) {
	p := NewPersonality("test-repo", "", nil)

	if !strings.Contains(p.SystemPrompt, "Guidelines") {
		t.Error("System prompt should contain Guidelines section")
	}

	if !strings.Contains(p.SystemPrompt, "Reference files") {
		t.Error("System prompt should instruct to reference files")
	}

	if !strings.Contains(p.SystemPrompt, "concise") {
		t.Error("System prompt should emphasize conciseness")
	}
}

func TestSystemPrompt_ContainsFormat(t *testing.T) {
	p := NewPersonality("test-repo", "", nil)

	if !strings.Contains(p.SystemPrompt, "Format") {
		t.Error("System prompt should contain Format section")
	}

	if !strings.Contains(p.SystemPrompt, "Markdown") {
		t.Error("System prompt should mention markdown formatting")
	}
}

func TestNewPersonality_MultipleFocusPaths(t *testing.T) {
	focusPaths := []string{"internal/core/", "pkg/api/", "cmd/server/", "lib/utils/"}
	p := NewPersonality("test-repo", "", focusPaths)

	for _, path := range focusPaths {
		if !strings.Contains(p.SystemPrompt, path) {
			t.Errorf("System prompt missing focus path: %s", path)
		}
	}
}

func TestNewPersonality_EmptyFocusPaths(t *testing.T) {
	p := NewPersonality("test-repo", "", []string{})

	if p == nil {
		t.Fatal("Expected personality with empty focus paths")
	}

	if p.SystemPrompt == "" {
		t.Error("System prompt should not be empty even without focus paths")
	}

	// Should not contain Focus Areas section when empty
	if strings.Contains(p.SystemPrompt, "Focus Areas") {
		t.Error("System prompt should not contain Focus Areas when no paths provided")
	}
}

func TestSystemPrompt_LengthReasonable(t *testing.T) {
	p := NewPersonality("test-repo", "", nil)

	promptLen := len(p.SystemPrompt)
	// Prompt should be concise (under 2000 chars without focus paths)
	if promptLen < 200 {
		t.Errorf("System prompt seems too short: %d characters", promptLen)
	}

	if promptLen > 2000 {
		t.Errorf("System prompt seems too long: %d characters", promptLen)
	}
}

func TestNewPersonality_DifferentRepoNames(t *testing.T) {
	testCases := []struct {
		repoName     string
		expectedName string
	}{
		{"my-app", "my-app-agent"},
		{"backend-service", "backend-service-agent"},
		{"react-frontend", "react-frontend-agent"},
		{"data-pipeline", "data-pipeline-agent"},
	}

	for _, tc := range testCases {
		t.Run(tc.repoName, func(t *testing.T) {
			p := NewPersonality(tc.repoName, "", nil)

			if p.Name != tc.expectedName {
				t.Errorf("Expected name '%s', got '%s'", tc.expectedName, p.Name)
			}

			if p.Repo != tc.repoName {
				t.Errorf("Expected repo '%s', got '%s'", tc.repoName, p.Repo)
			}
		})
	}
}

func TestSystemPrompt_NoTokenLimitMentioned(t *testing.T) {
	p := NewPersonality("test-repo", "", nil)

	// Should NOT mention token limits (it's counterproductive)
	if strings.Contains(p.SystemPrompt, "8192") || strings.Contains(p.SystemPrompt, "token limit") {
		t.Error("System prompt should not mention token limits")
	}
}

func TestSystemPrompt_LineNumberGuidance(t *testing.T) {
	p := NewPersonality("test-repo", "", nil)

	// Should only require line numbers when provided in context
	if !strings.Contains(p.SystemPrompt, "only if provided") {
		t.Error("System prompt should clarify line numbers are optional based on context")
	}
}
