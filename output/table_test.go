package output

import (
	"strings"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

func TestRenderTableGroupsBySourceAndShowsCounts(t *testing.T) {
	tools := []model.Tool{
		{Name: "gh", Source: model.SourceBrewFormula, Role: model.RoleInstalled},
		{Name: "wget", Source: model.SourceBrewFormula, Role: model.RoleInstalled},
		{Name: "opencode-ai", Source: model.SourceNPM, Role: model.RoleInstalled, Version: "1.18.4"},
		{Name: "gh", Source: model.SourcePath, Role: model.RoleAvailable, Path: "/opt/homebrew/bin/gh"},
	}
	npxHistory := []model.Tool{{Name: "create-vite", Source: model.SourceNPXHistory, Version: "2026-06-13"}}
	diff := registry.Diff{Added: []model.Tool{{Name: "flyctl", Source: "brew-formula"}}}
	warnings := []scanner.Warning{{Source: "pipx", Err: errFixture{"pipx not found"}}}

	out := RenderTable(tools, npxHistory, diff, warnings)

	for _, want := range []string{
		"BREW-FORMULA (2)",
		"gh",
		"wget",
		"NPM (1)",
		"opencode-ai",
		"1.18.4",
		"NPX HISTORY (1, informational)",
		"create-vite",
		"NEW SINCE LAST SCAN",
		"+ flyctl",
		"warning: pipx",
		"3 installed tools and 1 available command across 3 sources",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestCountToolRolesDistinguishesAvailableCommands(t *testing.T) {
	installed, available := countToolRoles([]model.Tool{
		{Name: "brew-tool", Source: model.SourceBrewFormula, Role: model.RoleInstalled},
		{Name: "path-tool", Source: model.SourcePath, Role: model.RoleAvailable},
	})
	if installed != 1 || available != 1 {
		t.Fatalf("expected one installed and one available observation, got installed=%d available=%d", installed, available)
	}
}

func TestCountSourcesIncludesAvailableSourcesAndExcludesHistory(t *testing.T) {
	got := countSources([]model.Tool{
		{Name: "brew-tool", Source: model.SourceBrewFormula, Role: model.RoleInstalled},
		{Name: "path-tool", Source: model.SourcePath, Role: model.RoleAvailable},
		{Name: "history-tool", Source: model.SourceNPXHistory, Role: model.RoleHistory},
	})
	if got != 2 {
		t.Fatalf("expected two non-history sources, got %d", got)
	}
}

type errFixture struct{ msg string }

func (e errFixture) Error() string { return e.msg }
