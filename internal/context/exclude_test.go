package context

import (
	"testing"
)

func TestMatchGlobPattern(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		pattern  string
		want     bool
	}{
		// Basic glob patterns
		{
			name:     "simple extension match",
			filePath: "README.md",
			pattern:  "*.md",
			want:     true,
		},
		{
			name:     "simple extension no match",
			filePath: "main.go",
			pattern:  "*.md",
			want:     false,
		},
		{
			name:     "exact filename match",
			filePath: ".env",
			pattern:  ".env",
			want:     true,
		},
		{
			name:     "exact filename in subdirectory",
			filePath: "config/.env",
			pattern:  ".env",
			want:     true,
		},

		// ** patterns - recursive directory matching
		{
			name:     "**/*.md matches markdown in root",
			filePath: "README.md",
			pattern:  "**/*.md",
			want:     true,
		},
		{
			name:     "**/*.md matches markdown in subdirectory",
			filePath: "docs/api/README.md",
			pattern:  "**/*.md",
			want:     true,
		},
		{
			name:     "**/*.md matches markdown deeply nested",
			filePath: "a/b/c/d/e/file.md",
			pattern:  "**/*.md",
			want:     true,
		},
		{
			name:     "**/*.md does not match .go files",
			filePath: "internal/api/handler.go",
			pattern:  "**/*.md",
			want:     false,
		},
		{
			name:     "**/.env* matches .env",
			filePath: ".env",
			pattern:  "**/.env*",
			want:     true,
		},
		{
			name:     "**/.env* matches .env.local",
			filePath: "config/.env.local",
			pattern:  "**/.env*",
			want:     true,
		},
		{
			name:     "**/.env* matches .env.production",
			filePath: "deploy/.env.production",
			pattern:  "**/.env*",
			want:     true,
		},
		{
			name:     "**/*.json matches json in subdirectory",
			filePath: "config/settings.json",
			pattern:  "**/*.json",
			want:     true,
		},

		// Directory patterns
		{
			name:     "**/vendor/** matches vendor subdirectory",
			filePath: "vendor/github.com/pkg/errors.go",
			pattern:  "**/vendor/**",
			want:     true,
		},
		{
			name:     "**/vendor/** matches nested vendor",
			filePath: "lib/vendor/pkg/file.go",
			pattern:  "**/vendor/**",
			want:     true,
		},
		{
			name:     "**/node_modules/** matches node_modules",
			filePath: "node_modules/react/index.js",
			pattern:  "**/node_modules/**",
			want:     true,
		},
		{
			name:     "docs/** matches everything under docs",
			filePath: "docs/api/reference.md",
			pattern:  "docs/**",
			want:     true,
		},
		{
			name:     "docs/** does not match sibling directories",
			filePath: "internal/api/handler.go",
			pattern:  "docs/**",
			want:     false,
		},

		// Edge cases
		{
			name:     "** matches everything",
			filePath: "any/path/to/file.txt",
			pattern:  "**",
			want:     true,
		},
		{
			name:     "pattern with leading slash",
			filePath: "src/main.go",
			pattern:  "/src/*.go",
			want:     false, // leading / means root-anchored in gitignore
		},
		{
			name:     "Windows-style path gets normalized",
			filePath: "internal\\api\\handler.go",
			pattern:  "**/*.go",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchGlobPattern(tt.filePath, tt.pattern)
			if got != tt.want {
				t.Errorf("matchGlobPattern(%q, %q) = %v, want %v",
					tt.filePath, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestShouldExclude(t *testing.T) {
	tests := []struct {
		name            string
		excludePatterns []string
		filePath        string
		want            bool
	}{
		{
			name:            "no patterns means no exclusions",
			excludePatterns: []string{},
			filePath:        "any/file.txt",
			want:            false,
		},
		{
			name:            "exclude all markdown files",
			excludePatterns: []string{"**/*.md"},
			filePath:        "docs/README.md",
			want:            true,
		},
		{
			name:            "exclude .env files",
			excludePatterns: []string{"**/.env*"},
			filePath:        ".env.local",
			want:            true,
		},
		{
			name:            "exclude json files",
			excludePatterns: []string{"**/*.json"},
			filePath:        "config/settings.json",
			want:            true,
		},
		{
			name:            "multiple patterns - matches first",
			excludePatterns: []string{"**/*.md", "**/*.json", "**/.env*"},
			filePath:        "docs/api.md",
			want:            true,
		},
		{
			name:            "multiple patterns - matches second",
			excludePatterns: []string{"**/*.md", "**/*.json", "**/.env*"},
			filePath:        "config.json",
			want:            true,
		},
		{
			name:            "multiple patterns - no match",
			excludePatterns: []string{"**/*.md", "**/*.json", "**/.env*"},
			filePath:        "main.go",
			want:            false,
		},
		{
			name:            "exclude vendor directory",
			excludePatterns: []string{"**/vendor/**"},
			filePath:        "vendor/github.com/pkg/errors.go",
			want:            true,
		},
		{
			name:            "exclude test files",
			excludePatterns: []string{"**/*_test.go"},
			filePath:        "internal/api/handler_test.go",
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Builder{
				excludePatterns: tt.excludePatterns,
			}
			got := b.shouldExclude(tt.filePath)
			if got != tt.want {
				t.Errorf("shouldExclude(%q) with patterns %v = %v, want %v",
					tt.filePath, tt.excludePatterns, got, tt.want)
			}
		})
	}
}
