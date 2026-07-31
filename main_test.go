package main

import (
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestSplitNPXHistory(t *testing.T) {
	tests := []struct {
		name        string
		input       []model.Tool
		wantReal    []model.Tool
		wantNPXHist []model.Tool
	}{
		{
			name: "mix of npx-history and other sources",
			input: []model.Tool{
				{Name: "gh", Source: "brew-formula"},
				{Name: "create-react-app", Source: "npx-history"},
				{Name: "wget", Source: "path"},
				{Name: "cowsay", Source: "npx-history"},
			},
			wantReal: []model.Tool{
				{Name: "gh", Source: "brew-formula"},
				{Name: "wget", Source: "path"},
			},
			wantNPXHist: []model.Tool{
				{Name: "create-react-app", Source: "npx-history"},
				{Name: "cowsay", Source: "npx-history"},
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
				{Name: "create-react-app", Source: "npx-history"},
				{Name: "cowsay", Source: "npx-history"},
			},
			wantReal: nil,
			wantNPXHist: []model.Tool{
				{Name: "create-react-app", Source: "npx-history"},
				{Name: "cowsay", Source: "npx-history"},
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
			gotReal, gotNPXHist := splitNPXHistory(tt.input)

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
