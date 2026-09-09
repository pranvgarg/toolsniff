package output

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestManagerPriorityOrder(t *testing.T) {
	// Lower rank = earlier in the Manage tab. npm leads; manual PATH trails.
	want := []string{
		model.SourceNPM, model.SourceBrewFormula, model.SourceBrewCask,
		model.SourceCargo, model.SourcePipx, model.SourceBun,
		model.SourceApplications, model.SourceNPXHistory, model.SourcePath,
	}
	got := append([]string(nil), want...)
	sort.Slice(got, func(i, j int) bool {
		return sourceGroupRank(got[i]) < sourceGroupRank(got[j])
	})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("manager order = %v, want %v", got, want)
	}
	// An unknown source must rank after everything known.
	if sourceGroupRank("unknown-provider-thing") <= sourceGroupRank(model.SourcePath) {
		t.Fatal("unknown source should rank after manual PATH")
	}
}

func TestIntentViewsHavePlainDefinitions(t *testing.T) {
	for _, v := range []ViewCategory{ViewManage, ViewDiscover, ViewReview, ViewHealth} {
		if ViewLabel(v) == "" || ViewMeaning(v) == "" {
			t.Fatalf("view %q missing label/meaning", v)
		}
	}
}

func TestEveryKindHasAPlainEnglishDefinition(t *testing.T) {
	for _, kind := range []model.ObservationKind{
		model.KindCLI, model.KindPackage, model.KindApplication, model.KindExecutable, model.KindHistory,
	} {
		definition := KindDefinitionFor(kind)
		if definition.Label == "" || definition.Singular == "" || definition.PlainMeaning == "" {
			t.Errorf("kind %q definition is incomplete: %+v", kind, definition)
			continue
		}
		if !strings.HasSuffix(definition.PlainMeaning, ".") {
			t.Errorf("kind %q meaning is not a sentence: %q", kind, definition.PlainMeaning)
		}
	}
	// An unrecognised kind must not borrow another kind's meaning.
	if got := KindDefinitionFor(model.ObservationKind("nonsense")); got != (KindDefinition{}) {
		t.Fatalf("unknown kind got a definition: %+v", got)
	}
}

func TestOriginDefinitionsNameWhoInstalledTheThing(t *testing.T) {
	for origin, want := range map[string]string{
		originNPM:         "A Node.js package installed globally with npm.",
		originBrewFormula: "A command-line program installed via Homebrew.",
		originBrewCask:    "A macOS app installed via Homebrew Cask.",
		originCargo:       "A Rust binary installed with cargo install.",
		originPipx:        "A Python app in its own environment, installed via pipx.",
		originPath:        "A program on your PATH not installed by a known manager — one you placed yourself.",
		originNPX:         "A package npx ran once and cached; not permanently installed.",
	} {
		if got := KindDefinitionForOrigin(origin).PlainMeaning; got != want {
			t.Errorf("origin %q meaning = %q, want %q", origin, got, want)
		}
	}
}

func TestKindDefinitionForObservationPrefersTheOrigin(t *testing.T) {
	tests := []struct {
		name        string
		observation model.Observation
		wantLabel   string
		wantMeaning string
	}{
		{
			name: "cask",
			observation: model.Observation{
				DisplayName: "visual-studio-code", Kind: model.KindPackage, Role: model.RoleInstalled,
				Origin: model.Origin{Provider: "homebrew", Manager: "cask", Package: "visual-studio-code"},
			},
			wantLabel:   "Homebrew cask",
			wantMeaning: "A macOS app installed via Homebrew Cask.",
		},
		{
			name: "npm cli",
			observation: model.Observation{
				DisplayName: "opencode", Kind: model.KindCLI, Role: model.RoleInstalled,
				Origin: model.Origin{Provider: "npm", Manager: "global", Package: "opencode-ai"},
			},
			wantLabel:   "npm",
			wantMeaning: "A Node.js package installed globally with npm.",
		},
		{
			name: "unmanaged path executable",
			observation: model.Observation{
				DisplayName: "gh", Kind: model.KindExecutable, Role: model.RoleAvailable,
				Origin: model.Origin{Provider: "unknown", Manager: "manual-or-unknown"},
			},
			// Nothing claims an unmanaged executable, so it falls through to the
			// kind's own definition -- which says the same thing in the plural.
			wantLabel:   "PATH executables",
			wantMeaning: "A program on your PATH not installed by a known manager — one you placed yourself.",
		},
		{
			name: "npx cache entry",
			observation: model.Observation{
				DisplayName: "create-vite", Kind: model.KindHistory, Role: model.RoleHistory,
				Origin: model.Origin{Provider: "npm", Manager: "npx", Package: "create-vite"},
			},
			wantLabel:   "npx history",
			wantMeaning: "A package npx ran once and cached; not permanently installed.",
		},
	}
	for _, test := range tests {
		definition := KindDefinitionForObservation(test.observation)
		if definition.Label != test.wantLabel || definition.PlainMeaning != test.wantMeaning {
			t.Errorf("%s definition = %+v, want label %q meaning %q",
				test.name, definition, test.wantLabel, test.wantMeaning)
		}
		if got := PlainMeaning(test.observation); got != test.wantMeaning {
			t.Errorf("%s PlainMeaning = %q", test.name, got)
		}
	}
}

func TestWhatThisIsLineJoinsTypeAndMeaning(t *testing.T) {
	cask := model.Observation{
		DisplayName: "visual-studio-code", Kind: model.KindPackage, Role: model.RoleInstalled,
		Origin: model.Origin{Provider: "homebrew", Manager: "cask", Package: "visual-studio-code"},
	}
	if got, want := WhatThisIsLine(cask), "Homebrew cask — a macOS app installed via Homebrew Cask."; got != want {
		t.Fatalf("WhatThisIsLine = %q, want %q", got, want)
	}
	path := model.Observation{
		DisplayName: "gh", Kind: model.KindExecutable, Role: model.RoleAvailable,
		Origin: model.Origin{Provider: "unknown", Manager: "manual-or-unknown"},
	}
	// The acronym must survive the sentence splice, and the line must never
	// leave the user with the bare word "available".
	line := WhatThisIsLine(path)
	if !strings.HasPrefix(line, "PATH executable — a program on your PATH") {
		t.Fatalf("PATH line = %q", line)
	}
}

func TestNoUserFacingLabelShowsBareScannerJargon(t *testing.T) {
	// "available" is a scanner word. Every user-facing string in the vocabulary
	// has to say what it means instead.
	for _, view := range append([]ViewCategory{ViewOverview, ViewChanges, ViewIssues, ViewAll, ViewInstalled, ViewAvailable, ViewHistory}, kindViews...) {
		label, meaning := ViewLabel(view), ViewMeaning(view)
		if label == "" || meaning == "" {
			t.Errorf("view %q has label %q meaning %q", view, label, meaning)
		}
		if strings.Contains(strings.ToLower(label), "available") {
			t.Errorf("view %q label leaks scanner jargon: %q", view, label)
		}
	}
	for source, label := range sourceGroupLabels {
		if strings.Contains(strings.ToLower(label), "available") {
			t.Errorf("group label for %q leaks scanner jargon: %q", source, label)
		}
	}
}

func TestKindViewsCoverEveryObservation(t *testing.T) {
	// Every observation a scanner can produce has to land in at least one
	// kind-first tab, or reorganising the tabs would have hidden data.
	observations := []model.Observation{
		{DisplayName: "npm-cli", Kind: model.KindCLI, Role: model.RoleInstalled, Origin: model.Origin{Provider: "npm", Manager: "global"}},
		{DisplayName: "npm-lib", Kind: model.KindPackage, Role: model.RoleInstalled, Origin: model.Origin{Provider: "npm", Manager: "global"}},
		{DisplayName: "wget", Kind: model.KindPackage, Role: model.RoleInstalled, Origin: model.Origin{Provider: "homebrew", Manager: "formula"}},
		{DisplayName: "rg", Kind: model.KindExecutable, Role: model.RoleInstalled, Origin: model.Origin{Provider: "cargo", Manager: "install"}},
		{DisplayName: "bun-tool", Kind: model.KindExecutable, Role: model.RoleInstalled, Origin: model.Origin{Provider: "bun", Manager: "global"}},
		{DisplayName: "Ghostty", Kind: model.KindApplication, Role: model.RoleInstalled, Origin: model.Origin{Provider: "applications"}},
		{DisplayName: "gh", Kind: model.KindExecutable, Role: model.RoleAvailable, Origin: model.Origin{Provider: "unknown", Manager: "manual-or-unknown"}},
		{DisplayName: "mystery", Kind: model.KindExecutable, Role: model.RoleInstalled},
		{DisplayName: "create-vite", Kind: model.KindHistory, Role: model.RoleHistory, Origin: model.Origin{Provider: "npm", Manager: "npx"}},
	}
	for _, observation := range observations {
		matched := make([]ViewCategory, 0, len(kindViews))
		for _, view := range kindViews {
			if observationMatchesView(observation, view) {
				matched = append(matched, view)
			}
		}
		if len(matched) == 0 {
			t.Errorf("%q (%s/%s) matches no kind view", observation.DisplayName, observation.Kind, observation.Origin.Provider)
		}
	}

	// Managed binaries belong with the packages, not with the programs the user
	// placed on their PATH themselves.
	cargo := observations[3]
	if !observationMatchesView(cargo, ViewPackages) || observationMatchesView(cargo, ViewPathExecutables) {
		t.Error("a cargo binary must be a package, not a hand-placed PATH executable")
	}
	unmanaged := observations[7]
	if observationMatchesView(unmanaged, ViewPackages) || !observationMatchesView(unmanaged, ViewPathExecutables) {
		t.Error("an executable with no known manager must be a PATH executable")
	}
}

func TestSourceGroupingFoldsUnknownManagerVariants(t *testing.T) {
	for source, want := range map[string]string{
		model.SourceNPM:         "npm packages",
		model.SourceBrewFormula: "Homebrew formulae",
		model.SourceBrewCask:    "Homebrew casks",
		model.SourcePath:        "On your PATH (not installed by a manager)",
		"cargo-install":         "Cargo binaries",
		"bun-global":            "Bun packages",
	} {
		if got := sourceGroupLabel(source); got != want {
			t.Errorf("sourceGroupLabel(%q) = %q, want %q", source, got, want)
		}
	}
	if sourceGroupRank(model.SourceNPM) >= sourceGroupRank(model.SourcePath) {
		t.Error("managed groups must sort above the hand-placed residue")
	}
	if got := sourceGroupLabel("nonesuch"); got != "nonesuch" {
		t.Errorf("unknown source label = %q, want it passed through", got)
	}
}
