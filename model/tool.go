package model

// Tool is a single installed CLI tool or application discovered by a scanner.
type Tool struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Version string `json:"version"`
	Path    string `json:"path"`
}
