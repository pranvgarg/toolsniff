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

func TestComputeDiffSameSourceDifferentNameNotConfused(t *testing.T) {
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

	if len(diff.Added) != 0 || len(diff.Removed) != 0 {
		t.Errorf("expected no changes, got %+v", diff)
	}
}
