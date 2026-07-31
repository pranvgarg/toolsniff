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
		{Name: "gh", Source: "brew-formula"},
		{Name: "wget", Source: "brew-formula"},
		{Name: "opencode-ai", Source: "npm", Version: "1.18.4"},
	}
	npxHistory := []model.Tool{{Name: "create-vite", Source: "npx-history", Version: "2026-06-13"}}
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
		"3 tools across 2 sources",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

type errFixture struct{ msg string }

func (e errFixture) Error() string { return e.msg }
