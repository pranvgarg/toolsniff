package model

import "sort"

// Tool is a single installed CLI tool or application discovered by a scanner.
type Tool struct {
	Name    string     `json:"name"`
	Source  string     `json:"source"`
	Role    SourceRole `json:"role,omitempty"`
	Version string     `json:"version"`
	Path    string     `json:"path"`
}

// ToolIdentity identifies one observation without merging different source
// types. A path is the strongest identity available for filesystem-derived
// observations; package-manager observations fall back to their package name.
func ToolIdentity(tool Tool) string {
	identity := tool.Name
	if tool.Path != "" {
		identity = tool.Path
	}
	return tool.Source + "\x00" + identity
}

// DeduplicateTools removes exact repeated observations while preserving
// distinct installation sources and distinct paths. The result is stable for
// machine-readable output.
func DeduplicateTools(tools []Tool) []Tool {
	seen := make(map[string]struct{}, len(tools))
	unique := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		key := ToolIdentity(tool)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, tool)
	}
	sort.SliceStable(unique, func(i, j int) bool {
		if unique[i].Source != unique[j].Source {
			return unique[i].Source < unique[j].Source
		}
		if unique[i].Name != unique[j].Name {
			return unique[i].Name < unique[j].Name
		}
		return unique[i].Path < unique[j].Path
	})
	return unique
}
