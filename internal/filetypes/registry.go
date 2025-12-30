// Package filetypes provides unified file type detection and classification.
//
// This package serves as the single source of truth for determining which files
// should be indexed and how they should be processed, eliminating duplication
// across builder, indexer, and qdrant packages.
package filetypes

import (
	"path/filepath"
	"strings"
)

// Extensions maps file extensions to whether they should be indexed as code
var Extensions = map[string]bool{
	// Go
	".go": true,

	// JavaScript/TypeScript
	".js":  true,
	".ts":  true,
	".tsx": true,
	".jsx": true,
	".mjs": true,
	".cjs": true,

	// Python
	".py": true,

	// Java/JVM
	".java":  true,
	".kt":    true,
	".kts":   true,
	".scala": true,

	// C/C++
	".c":   true,
	".cpp": true,
	".cc":  true,
	".cxx": true,
	".h":   true,
	".hpp": true,
	".hxx": true,

	// C#
	".cs": true,

	// Ruby
	".rb": true,

	// PHP
	".php": true,

	// Swift
	".swift": true,

	// Rust
	".rs": true,

	// Shell
	".sh":   true,
	".bash": true,
	".zsh":  true,

	// Config/Data
	".proto": true,
	".sql":   true,
	".yaml":  true,
	".yml":   true,
	".json":  true,
	".toml":  true,
	".xml":   true,

	// Documentation
	".md":  true,
	".rst": true,
}

// Languages maps file extensions to their syntax highlighting language identifiers
var Languages = map[string]string{
	// Go
	".go": "go",

	// JavaScript/TypeScript
	".js":  "javascript",
	".mjs": "javascript",
	".cjs": "javascript",
	".ts":  "typescript",
	".tsx": "typescript",
	".jsx": "javascript",

	// Python
	".py": "python",

	// Java/JVM
	".java":  "java",
	".kt":    "kotlin",
	".kts":   "kotlin",
	".scala": "scala",

	// C/C++
	".c":   "c",
	".cpp": "cpp",
	".cc":  "cpp",
	".cxx": "cpp",
	".h":   "c",
	".hpp": "cpp",
	".hxx": "cpp",

	// C#
	".cs": "csharp",

	// Ruby
	".rb": "ruby",

	// PHP
	".php": "php",

	// Swift
	".swift": "swift",

	// Rust
	".rs": "rust",

	// Shell
	".sh":   "bash",
	".bash": "bash",
	".zsh":  "bash",

	// Config/Data
	".proto": "protobuf",
	".sql":   "sql",
	".yaml":  "yaml",
	".yml":   "yaml",
	".json":  "json",
	".toml":  "toml",
	".xml":   "xml",

	// Documentation
	".md":  "markdown",
	".rst": "restructuredtext",
}

// SkipDirectories lists directories that should be skipped during file walks
var SkipDirectories = map[string]bool{
	// Version control
	".git": true,
	".svn": true,
	".hg":  true,

	// Dependencies
	"node_modules": true,
	"vendor":       true,

	// Build artifacts
	"dist":   true,
	"build":  true,
	"target": true,
	"out":    true,

	// Python
	"__pycache__": true,
	".venv":       true,
	"venv":        true,
	".tox":        true,

	// IDE
	".idea":   true,
	".vscode": true,
	".vs":     true,

	// Other
	".next":  true,
	".cache": true,
	"tmp":    true,
	"temp":   true,
}

// IsCodeFile returns true if the file should be indexed based on its extension
func IsCodeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return Extensions[ext]
}

// GetLanguage returns the syntax highlighting language for a file path
// Returns empty string if not a recognized code file
func GetLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	return Languages[ext]
}

// ShouldSkipDirectory returns true if a directory should be skipped during file walks
func ShouldSkipDirectory(name string) bool {
	// Skip hidden directories (except . and ..)
	if len(name) > 0 && name[0] == '.' && name != "." && name != ".." {
		// Allow .github and other potentially useful hidden dirs
		if name != ".github" {
			return true
		}
	}

	// Check skip list
	return SkipDirectories[name]
}
