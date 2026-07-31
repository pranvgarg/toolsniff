package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
)

// ApplicationsScanner finds dev/AI-relevant apps in /Applications, filtered
// by keyword since the directory otherwise contains everything on the machine.
type ApplicationsScanner struct {
	dir      string
	keywords []string
}

func NewApplicationsScanner(dir string, keywords []string) *ApplicationsScanner {
	lowered := make([]string, len(keywords))
	for i, kw := range keywords {
		lowered[i] = strings.ToLower(kw)
	}
	return &ApplicationsScanner{dir: dir, keywords: lowered}
}

// DefaultApplicationsDir returns the standard macOS /Applications path.
func DefaultApplicationsDir() string { return "/Applications" }

// DefaultApplicationKeywords is the curated list of substrings (matched
// case-insensitively against app bundle names) considered dev/AI-relevant.
// Extend this list to widen what the scanner picks up.
func DefaultApplicationKeywords() []string {
	return []string{
		"claude", "chatgpt", "gpt", "cursor", "devin", "ollama",
		"lm studio", "antigravity", "finetune", "agents", "wispr",
		"copilot", "codex", "windsurf", "docker", "github desktop",
	}
}

func (s *ApplicationsScanner) Name() string { return "applications" }

func (s *ApplicationsScanner) Scan() ([]model.Tool, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("applications: %w", err)
	}

	tools := make([]model.Tool, 0)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".app") {
			continue
		}
		lower := strings.ToLower(name)
		matched := false
		for _, kw := range s.keywords {
			if strings.Contains(lower, kw) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		tools = append(tools, model.Tool{
			Name:   name,
			Source: "applications",
			Path:   filepath.Join(s.dir, name),
		})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}
