package registry

import (
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestComputeDiffAddedAndRemoved(t *testing.T) {
	old := []model.Tool{
		{Name: "gh", Source: "brew-formula"},
		{Name: "wget", Source: "brew-formula"},
	}
	current := []model.Tool{
		{Name: "gh", Source: "brew-formula"},
		{Name: "flyctl", Source: "brew-formula"},
	}

	diff := ComputeDiff(old, current)

	if len(diff.Added) != 1 || diff.Added[0].Name != "flyctl" {
		t.Errorf("unexpected Added: %+v", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].Name != "wget" {
		t.Errorf("unexpected Removed: %+v", diff.Removed)
	}
}

func TestComputeDiffSameNameDifferentSourceNotConfused(t *testing.T) {
	old := []model.Tool{{Name: "gh", Source: "brew-formula"}}
	current := []model.Tool{{Name: "gh", Source: "path"}}

	diff := ComputeDiff(old, current)

	if len(diff.Added) != 1 || diff.Added[0].Source != "path" {
		t.Errorf("expected same-name-different-source to count as added: %+v", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].Source != "brew-formula" {
		t.Errorf("expected same-name-different-source to count as removed: %+v", diff.Removed)
	}
}

func TestComputeDiffEmptyBaseline(t *testing.T) {
	current := []model.Tool{{Name: "gh", Source: "brew-formula"}}
	diff := ComputeDiff(nil, current)

	if len(diff.Added) != 1 || len(diff.Removed) != 0 {
		t.Errorf("expected everything to be Added on empty baseline, got %+v", diff)
	}
}

func TestComputeDiffNoChanges(t *testing.T) {
	tools := []model.Tool{{Name: "gh", Source: "brew-formula"}}
	diff := ComputeDiff(tools, tools)

	if len(diff.Added) != 0 || len(diff.Removed) != 0 || len(diff.Updated) != 0 {
		t.Errorf("expected no changes, got %+v", diff)
	}
}

func TestComputeDiffReportsVersionUpgrade(t *testing.T) {
	old := []model.Tool{{Name: "opencode-ai", Source: model.SourceNPM, Version: "1.18.10"}}
	current := []model.Tool{{Name: "opencode-ai", Source: model.SourceNPM, Version: "1.19.0"}}

	diff := ComputeDiff(old, current)
	if len(diff.Added) != 0 || len(diff.Removed) != 0 || len(diff.Updated) != 1 {
		t.Fatalf("expected one update, got %+v", diff)
	}
	if diff.Updated[0].Before.Version != "1.18.10" || diff.Updated[0].After.Version != "1.19.0" {
		t.Errorf("unexpected version change: %+v", diff.Updated[0])
	}
}

func TestComputeDiffReportsEmptyToKnownVersion(t *testing.T) {
	old := []model.Tool{{Name: "tool", Source: model.SourceNPM}}
	current := []model.Tool{{Name: "tool", Source: model.SourceNPM, Version: "1.0.0"}}

	diff := ComputeDiff(old, current)
	if len(diff.Updated) != 1 {
		t.Fatalf("expected empty-to-known update, got %+v", diff)
	}
}

func TestComputeDiffIgnoresKnownToEmptyVersion(t *testing.T) {
	old := []model.Tool{{Name: "tool", Source: model.SourceNPM, Version: "1.0.0"}}
	current := []model.Tool{{Name: "tool", Source: model.SourceNPM}}

	diff := ComputeDiff(old, current)
	if len(diff.Added) != 0 || len(diff.Removed) != 0 || len(diff.Updated) != 0 {
		t.Fatalf("expected no noisy version change, got %+v", diff)
	}
}

func TestComputeDiffKeepsSameNameAtDifferentPathsSeparate(t *testing.T) {
	old := []model.Tool{{Name: "tool", Source: "path", Path: "/one/tool"}}
	current := []model.Tool{{Name: "tool", Source: "path", Path: "/two/tool"}}

	diff := ComputeDiff(old, current)
	if len(diff.Added) != 1 || diff.Added[0].Path != "/two/tool" {
		t.Errorf("expected the new path to be added, got %+v", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].Path != "/one/tool" {
		t.Errorf("expected the old path to be removed, got %+v", diff.Removed)
	}
}

func TestComputeDiffKeepsSameNameAcrossSourcesSeparate(t *testing.T) {
	old := []model.Tool{{Name: "tool", Source: model.SourceBrewFormula}}
	current := []model.Tool{{Name: "tool", Source: model.SourceNPM}}

	diff := ComputeDiff(old, current)
	if len(diff.Added) != 1 || diff.Added[0].Source != model.SourceNPM {
		t.Errorf("expected npm observation to be added, got %+v", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].Source != model.SourceBrewFormula {
		t.Errorf("expected brew observation to be removed, got %+v", diff.Removed)
	}
}
