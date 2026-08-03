package output

import (
	"strings"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestParseFilterSupportsPlainSearchAndANDedFacets(t *testing.T) {
	state, err := ParseFilter("source:npm kind:package version:known gemini")
	if err != nil {
		t.Fatalf("ParseFilter failed: %v", err)
	}
	if state.Text != "gemini" || !state.Sources[model.SourceNPM] || !state.Kinds[model.KindPackage] || !state.VersionState[model.VersionKnown] {
		t.Fatalf("unexpected parsed state: %+v", state)
	}
	matching := observation("gemini-cli", model.RoleInstalled, model.KindPackage, model.VersionInfo{Value: "1.0.0", State: model.VersionKnown, Confidence: model.ConfidenceHigh})
	nonMatching := observation("gemini-cli", model.RoleInstalled, model.KindApplication, model.VersionInfo{State: model.VersionUnknown, Confidence: model.ConfidenceLow})
	if !MatchObservation(state, matching) || MatchObservation(state, nonMatching) {
		t.Fatalf("facet AND semantics failed")
	}
}

func TestParseFilterRejectsInvalidFacetValues(t *testing.T) {
	for _, input := range []string{"status:nope", "kind:nope", "version:nope", "view:nope", "unknown:value"} {
		if _, err := ParseFilter(input); err == nil {
			t.Errorf("ParseFilter(%q) accepted invalid facet", input)
		}
	}
}

func TestFilterChipsRemoveAndEmptyMessage(t *testing.T) {
	state, err := ParseFilter("source:npm status:updated gemini")
	if err != nil {
		t.Fatal(err)
	}
	chips := FilterChips(state)
	if len(chips) != 3 || chips[0].Key != "search" {
		t.Fatalf("unexpected chips: %+v", chips)
	}
	state = RemoveFilterChip(state, FilterChip{Key: "status", Value: "updated"})
	if len(state.Statuses) != 0 {
		t.Fatalf("status chip was not removed: %+v", state)
	}
	empty := EmptyResultMessage(state, 0)
	for _, want := range []string{"No matches", "source:npm", "search:gemini", "clear filters"} {
		if !strings.Contains(empty, want) {
			t.Errorf("empty message missing %q: %s", want, empty)
		}
	}
	if EmptyResultMessage(state, 1) != "" {
		t.Fatal("non-empty result returned an empty-result message")
	}
}

func TestFilterDrawerAppliesAndClearsDraft(t *testing.T) {
	drawer := NewFilterDrawer(NewFilterState())
	drawer.Draft = "source:npm gemini"
	if err := drawer.Apply(); err != nil {
		t.Fatal(err)
	}
	if drawer.Open || drawer.State.Text != "gemini" || !drawer.State.Sources[model.SourceNPM] {
		t.Fatalf("drawer did not apply draft: %+v", drawer)
	}
	drawer.Clear()
	if drawer.State.Text != "" || len(drawer.State.Sources) != 0 {
		t.Fatalf("drawer did not clear: %+v", drawer.State)
	}
}
