package main

import (
	"testing"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/scanner"
)

func TestSplitByRole(t *testing.T) {
	registrations := []scanner.Registration{
		{SourceInfo: scanner.SourceInfo{ID: model.SourceBrewFormula, Role: model.RoleInstalled}},
		{SourceInfo: scanner.SourceInfo{ID: model.SourcePath, Role: model.RoleAvailable}},
		{SourceInfo: scanner.SourceInfo{ID: model.SourceNPXHistory, Role: model.RoleHistory, Informational: true}},
	}
	tests := []struct {
		name        string
		input       []model.Tool
		wantReal    []model.Tool
		wantNPXHist []model.Tool
	}{
		{
			name: "mix of npx-history and other sources",
			input: []model.Tool{
				{Name: "gh", Source: model.SourceBrewFormula},
				{Name: "create-react-app", Source: model.SourceNPXHistory},
				{Name: "wget", Source: model.SourcePath},
				{Name: "cowsay", Source: model.SourceNPXHistory},
			},
			wantReal: []model.Tool{
				{Name: "gh", Source: model.SourceBrewFormula},
				{Name: "wget", Source: model.SourcePath},
			},
			wantNPXHist: []model.Tool{
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
			wantReal: []model.Tool{
				{Name: "gh", Source: "brew-formula"},
				{Name: "wget", Source: "path"},
			},
			wantNPXHist: nil,
		},
		{
			name: "all npx-history",
			input: []model.Tool{
				{Name: "create-react-app", Source: model.SourceNPXHistory},
				{Name: "cowsay", Source: model.SourceNPXHistory},
			},
			wantReal: nil,
			wantNPXHist: []model.Tool{
				{Name: "create-react-app", Source: model.SourceNPXHistory},
				{Name: "cowsay", Source: model.SourceNPXHistory},
			},
		},
		{
			name:        "empty input",
			input:       nil,
			wantReal:    nil,
			wantNPXHist: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReal, gotNPXHist := splitByRole(tt.input, registrations)

			if !toolsEqual(gotReal, tt.wantReal) {
				t.Errorf("real = %+v, want %+v", gotReal, tt.wantReal)
			}
			if !toolsEqual(gotNPXHist, tt.wantNPXHist) {
				t.Errorf("npxHistory = %+v, want %+v", gotNPXHist, tt.wantNPXHist)
			}
		})
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
