package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/scanner"
)

func TestRunVersionUsesProvidedOutput(t *testing.T) {
	var output, errorOutput bytes.Buffer

	if code := Run([]string{"--version"}, strings.NewReader(""), &output, &errorOutput); code != 0 {
		t.Fatalf("Run returned %d, want 0", code)
	}
	if got, want := output.String(), "dev\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	if errorOutput.Len() != 0 {
		t.Fatalf("unexpected error output: %q", errorOutput.String())
	}
}

func TestRunRejectsMultipleReportModes(t *testing.T) {
	var output, errorOutput bytes.Buffer

	if code := Run([]string{"--list", "--json"}, strings.NewReader(""), &output, &errorOutput); code != 2 {
		t.Fatalf("Run returned %d, want 2", code)
	}
	if got, want := errorOutput.String(), "only one of --list, --json, --save, --diff, or --update may be used\n"; got != want {
		t.Fatalf("error output = %q, want %q", got, want)
	}
}

func TestSplitByRole(t *testing.T) {
	registrations := []scanner.Registration{
		{SourceInfo: scanner.SourceInfo{ID: model.SourceBrewFormula, Role: model.RoleInstalled}},
		{SourceInfo: scanner.SourceInfo{ID: model.SourcePath, Role: model.RoleAvailable}},
		{SourceInfo: scanner.SourceInfo{ID: model.SourceNPXHistory, Role: model.RoleHistory, Informational: true}},
	}
	tests := []struct {
		name           string
		input          []model.Tool
		wantInstalled  []model.Tool
		wantAvailable  []model.Tool
		wantNPXHistory []model.Tool
	}{
		{
			name: "mix of npx-history and other sources",
			input: []model.Tool{
				{Name: "gh", Source: model.SourceBrewFormula},
				{Name: "create-react-app", Source: model.SourceNPXHistory},
				{Name: "wget", Source: model.SourcePath},
				{Name: "cowsay", Source: model.SourceNPXHistory},
			},
			wantInstalled: []model.Tool{{Name: "gh", Source: model.SourceBrewFormula}},
			wantAvailable: []model.Tool{{Name: "wget", Source: model.SourcePath}},
			wantNPXHistory: []model.Tool{
				{Name: "create-react-app", Source: model.SourceNPXHistory},
				{Name: "cowsay", Source: model.SourceNPXHistory},
			},
		},
		{
			name: "all real",
			input: []model.Tool{
				{Name: "gh", Source: "brew-formula"},
				{Name: "wget", Source: "path"},
			},
			wantInstalled: []model.Tool{{Name: "gh", Source: "brew-formula"}},
			wantAvailable: []model.Tool{{Name: "wget", Source: "path"}},
		},
		{
			name: "all npx-history",
			input: []model.Tool{
				{Name: "create-react-app", Source: model.SourceNPXHistory},
				{Name: "cowsay", Source: model.SourceNPXHistory},
			},
			wantNPXHistory: []model.Tool{
				{Name: "create-react-app", Source: model.SourceNPXHistory},
				{Name: "cowsay", Source: model.SourceNPXHistory},
			},
		},
		{
			name: "empty input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotInstalled, gotAvailable, gotNPXHistory := splitByRole(tt.input, registrations)

			if !toolsEqual(gotInstalled, tt.wantInstalled) {
				t.Errorf("installed = %+v, want %+v", gotInstalled, tt.wantInstalled)
			}
			if !toolsEqual(gotAvailable, tt.wantAvailable) {
				t.Errorf("available = %+v, want %+v", gotAvailable, tt.wantAvailable)
			}
			if !toolsEqual(gotNPXHistory, tt.wantNPXHistory) {
				t.Errorf("npxHistory = %+v, want %+v", gotNPXHistory, tt.wantNPXHistory)
			}
		})
	}
}

func TestValidateFlagsRequiresDiffForAvailability(t *testing.T) {
	if err := validateFlags(true, false, false, false); err == nil {
		t.Fatal("expected --available without --diff to be rejected")
	}
	if err := validateFlags(true, true, false, false); err != nil {
		t.Fatalf("expected --available with --diff to be accepted: %v", err)
	}
	if err := validateFlags(false, false, true, false); err != nil {
		t.Fatalf("expected --update to be accepted: %v", err)
	}
	if err := validateFlags(false, false, false, true); err == nil {
		t.Fatal("expected --yes without --update to be rejected")
	}
	if err := validateFlags(false, false, false, false); err != nil {
		t.Fatalf("expected ordinary mode to be accepted: %v", err)
	}
}

func toolsEqual(a, b []model.Tool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
