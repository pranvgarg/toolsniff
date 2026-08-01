package output

import (
	"encoding/json"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

func TestRenderJSONShape(t *testing.T) {
	tools := []model.Tool{
		{Name: "gh", Source: model.SourceBrewFormula, Role: model.RoleInstalled},
		{Name: "gh", Source: model.SourcePath, Role: model.RoleAvailable, Path: "/opt/homebrew/bin/gh"},
	}
	npxHistory := []model.Tool{{Name: "create-vite", Source: model.SourceNPXHistory}}
	diff := registry.Diff{Added: []model.Tool{{Name: "flyctl", Source: "brew-formula"}}}
	warnings := []scanner.Warning{{Source: "pipx", Err: errFixture{"not found"}}}

	data, err := RenderJSON(tools, npxHistory, diff, warnings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var report JSONReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, data)
	}

	if len(report.Tools) != 2 || report.Tools[0].Name != "gh" || report.Tools[0].Role != model.RoleInstalled {
		t.Errorf("unexpected Tools: %+v", report.Tools)
	}
	if report.Tools[1].Source != model.SourcePath || report.Tools[1].Role != model.RoleAvailable || report.Tools[1].Path != "/opt/homebrew/bin/gh" {
		t.Errorf("unexpected available tool: %+v", report.Tools[1])
	}
	if len(report.NPXHistory) != 1 || report.NPXHistory[0].Name != "create-vite" {
		t.Errorf("unexpected NPXHistory: %+v", report.NPXHistory)
	}
	if len(report.Added) != 1 || report.Added[0].Name != "flyctl" {
		t.Errorf("unexpected Added: %+v", report.Added)
	}
	if len(report.Warnings) != 1 || report.Warnings[0] != "pipx: not found" {
		t.Errorf("unexpected Warnings: %+v", report.Warnings)
	}
}
