package scanner

import (
	"sort"

	"github.com/pranvgarg/toolsniff/model"
)

// PathLookup resolves a command name to its full path, matching the
// signature of exec.LookPath so it can be passed directly in production.
type PathLookup func(name string) (string, error)

// PathScanner checks a curated candidate list against $PATH rather than
// enumerating every binary on the system.
type PathScanner struct {
	lookup     PathLookup
	candidates []string
}

func NewPathScanner(lookup PathLookup, candidates []string) *PathScanner {
	return &PathScanner{lookup: lookup, candidates: candidates}
}

// DefaultPathCandidates is the curated list of dev/AI CLI tool names to
// check for on $PATH. Extend this list to widen what the scanner picks up.
func DefaultPathCandidates() []string {
	return []string{
		"claude", "ollama", "opencode", "gemini", "codex", "pi", "mo",
		"gh", "vercel", "flyctl", "azd", "whisper-cpp", "aider",
		"cursor", "cline", "continue", "llm", "sgpt", "cody", "warp",
		"uv", "uvx", "ngrok",
	}
}

func (s *PathScanner) Name() string { return "path" }

func (s *PathScanner) Scan() ([]model.Tool, error) {
	tools := make([]model.Tool, 0)
	for _, name := range s.candidates {
		path, err := s.lookup(name)
		if err != nil {
			continue
		}
		tools = append(tools, model.Tool{Name: name, Source: "path", Path: path})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}
