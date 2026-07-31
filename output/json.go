package output

import (
	"encoding/json"
	"fmt"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

// JSONReport is the full --json output shape.
type JSONReport struct {
	Tools      []model.Tool `json:"tools"`
	NPXHistory []model.Tool `json:"npx_history"`
	Added      []model.Tool `json:"added"`
	Removed    []model.Tool `json:"removed"`
	Warnings   []string     `json:"warnings"`
}

// RenderJSON produces the full scan report as indented JSON.
func RenderJSON(tools, npxHistory []model.Tool, diff registry.Diff, warnings []scanner.Warning) ([]byte, error) {
	report := JSONReport{
		Tools:      tools,
		NPXHistory: npxHistory,
		Added:      diff.Added,
		Removed:    diff.Removed,
		Warnings:   make([]string, 0, len(warnings)),
	}
	for _, w := range warnings {
		report.Warnings = append(report.Warnings, fmt.Sprintf("%s: %v", w.Source, w.Err))
	}
	return json.MarshalIndent(report, "", "  ")
}
