package output

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pranvgarg/toolsniff/model"
)

// actionFor returns the action with the given kind, or fails the test.
func actionFor(t *testing.T, actions []Action, kind, label string) Action {
	t.Helper()
	for _, action := range actions {
		if action.Kind == kind && action.Label == label {
			return action
		}
	}
	t.Fatalf("no %s/%q action in %s", kind, label, formatActions(actions))
	return Action{}
}

func formatActions(actions []Action) string {
	parts := make([]string, 0, len(actions))
	for _, action := range actions {
		parts = append(parts, action.Kind+"/"+action.Label+"["+strings.Join(action.Command, " ")+"]")
	}
	return strings.Join(parts, ", ")
}

func wantCommand(t *testing.T, action Action, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(action.Command, want) {
		t.Fatalf("%s command = %#v, want %#v", action.Label, action.Command, want)
	}
}

func TestKindActionsForNPMGlobalPackage(t *testing.T) {
	observation := model.Observation{
		DisplayName: "opencode-ai",
		Kind:        model.KindPackage,
		Role:        model.RoleInstalled,
		Origin:      model.Origin{Provider: "npm", Manager: "global", Package: "opencode-ai"},
		Package:     &model.PackageInfo{Name: "opencode-ai", Prefix: "/opt/homebrew/lib/node_modules/opencode-ai"},
		Locations:   []model.Location{{Path: "/opt/homebrew/bin/opencode", Type: model.LocationExecutable, Executable: true}},
	}
	actions := KindActions(observation)

	wantCommand(t, actionFor(t, actions, ActionUpdate, "Update"), "npm", "update", "-g", "opencode-ai")
	wantCommand(t, actionFor(t, actions, ActionUninstall, "Uninstall"), "npm", "uninstall", "-g", "opencode-ai")
	wantCommand(t, actionFor(t, actions, ActionInfo, "Info"), "npm", "ls", "-g", "opencode-ai")
	wantCommand(t, actionFor(t, actions, ActionReveal, "Reveal in Finder"),
		"open", "-R", "/opt/homebrew/lib/node_modules/opencode-ai")

	copyAction := actionFor(t, actions, ActionCopy, "Copy path")
	if copyAction.Runnable() || copyAction.Note != "/opt/homebrew/bin/opencode" {
		t.Fatalf("copy action = %#v", copyAction)
	}
	if note := actions[0].Note; note != "npm global package" {
		t.Fatalf("npm note = %q", note)
	}
}

func TestKindActionsForBrewFormulaAndCask(t *testing.T) {
	formula := model.Observation{
		DisplayName: "wget",
		Kind:        model.KindPackage,
		Role:        model.RoleInstalled,
		Origin:      model.Origin{Provider: "homebrew", Manager: "formula", Package: "wget"},
		Locations:   []model.Location{{Path: "/opt/homebrew/Cellar/wget", Type: model.LocationPackagePrefix}},
	}
	actions := KindActions(formula)
	wantCommand(t, actionFor(t, actions, ActionUpdate, "Upgrade"), "brew", "upgrade", "wget")
	wantCommand(t, actionFor(t, actions, ActionUninstall, "Uninstall"), "brew", "uninstall", "wget")
	wantCommand(t, actionFor(t, actions, ActionReveal, "Reveal in Finder"), "open", "-R", "/opt/homebrew/Cellar/wget")
	if got := ObservationTypeLabel(formula); got != "Homebrew formula" {
		t.Fatalf("formula type label = %q", got)
	}

	cask := model.Observation{
		DisplayName: "visual-studio-code",
		Kind:        model.KindPackage,
		Role:        model.RoleInstalled,
		Origin:      model.Origin{Provider: "homebrew", Manager: "cask", Package: "visual-studio-code"},
		Locations:   []model.Location{{Path: "/Applications/Visual Studio Code.app", Type: model.LocationApplication}},
	}
	actions = KindActions(cask)
	wantCommand(t, actionFor(t, actions, ActionUpdate, "Upgrade"), "brew", "upgrade", "--cask", "visual-studio-code")
	wantCommand(t, actionFor(t, actions, ActionUninstall, "Uninstall"), "brew", "uninstall", "--cask", "visual-studio-code")
	wantCommand(t, actionFor(t, actions, ActionReveal, "Reveal in Finder"),
		"open", "-R", "/Applications/Visual Studio Code.app")
	if got := ObservationTypeLabel(cask); got != "Homebrew cask" {
		t.Fatalf("cask type label = %q", got)
	}
}

func TestKindActionsForPipxAndCargo(t *testing.T) {
	pipx := model.Observation{
		DisplayName: "ruff",
		Kind:        model.KindPackage,
		Role:        model.RoleInstalled,
		Origin:      model.Origin{Provider: "pipx", Manager: "global", Package: "ruff"},
		Package:     &model.PackageInfo{Name: "ruff", Prefix: "/Users/x/.local/pipx/venvs/ruff"},
	}
	actions := KindActions(pipx)
	wantCommand(t, actionFor(t, actions, ActionUninstall, "Uninstall"), "pipx", "uninstall", "ruff")
	wantCommand(t, actionFor(t, actions, ActionReveal, "Reveal in Finder"),
		"open", "-R", "/Users/x/.local/pipx/venvs/ruff")
	for _, action := range actions {
		if action.Kind == ActionUpdate {
			t.Fatalf("pipx must not offer an update command: %#v", action)
		}
	}

	cargo := model.Observation{
		DisplayName: "ripgrep",
		Kind:        model.KindExecutable,
		Role:        model.RoleInstalled,
		Origin:      model.Origin{Provider: "cargo", Manager: "install", Package: "ripgrep"},
		Locations:   []model.Location{{Path: "/Users/x/.cargo/bin/rg", Type: model.LocationExecutable, Executable: true}},
	}
	actions = KindActions(cargo)
	wantCommand(t, actionFor(t, actions, ActionUninstall, "Uninstall"), "cargo", "uninstall", "ripgrep")
	wantCommand(t, actionFor(t, actions, ActionReveal, "Reveal in Finder"), "open", "-R", "/Users/x/.cargo/bin/rg")
	if got := ObservationTypeLabel(cargo); got != "Cargo binary" {
		t.Fatalf("cargo type label = %q", got)
	}
}

func TestKindActionsForApplicationBundle(t *testing.T) {
	observation := model.Observation{
		DisplayName: "Ghostty",
		Kind:        model.KindApplication,
		Role:        model.RoleInstalled,
		Origin:      model.Origin{Provider: "applications"},
		Application: &model.ApplicationInfo{BundleID: "com.mitchellh.ghostty"},
		Locations:   []model.Location{{Path: "/Applications/Ghostty.app", Type: model.LocationApplication}},
	}
	actions := KindActions(observation)
	wantCommand(t, actionFor(t, actions, ActionOpen, "Open"), "open", "/Applications/Ghostty.app")
	wantCommand(t, actionFor(t, actions, ActionReveal, "Reveal in Finder"), "open", "-R", "/Applications/Ghostty.app")
	if got := ObservationTypeLabel(observation); got != "macOS application" {
		t.Fatalf("application type label = %q", got)
	}
}

func TestKindActionsForPathExecutable(t *testing.T) {
	observation := model.Observation{
		DisplayName: "gh",
		Kind:        model.KindExecutable,
		Role:        model.RoleAvailable,
		Origin:      model.Origin{Provider: "unknown", Manager: "manual-or-unknown"},
		Locations:   []model.Location{{Path: "/usr/local/bin/gh", Type: model.LocationExecutable, Executable: true}},
	}
	actions := KindActions(observation)
	wantCommand(t, actionFor(t, actions, ActionReveal, "Reveal in Finder"), "open", "-R", "/usr/local/bin/gh")
	if copyAction := actionFor(t, actions, ActionCopy, "Copy path"); copyAction.Note != "/usr/local/bin/gh" {
		t.Fatalf("copy note = %q", copyAction.Note)
	}
	for _, action := range actions {
		if action.Kind == ActionUpdate || action.Kind == ActionUninstall {
			t.Fatalf("an unmanaged PATH executable must not offer %s: %#v", action.Kind, action)
		}
	}
	if got := ObservationTypeLabel(observation); got != "PATH executable" {
		t.Fatalf("path type label = %q", got)
	}
}

func TestKindActionsForNPXHistoryIsInformational(t *testing.T) {
	lastUsed := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	observation := model.Observation{
		DisplayName: "create-vite",
		Kind:        model.KindHistory,
		Role:        model.RoleHistory,
		Origin:      model.Origin{Provider: "npm", Manager: "npx", Package: "create-vite"},
		History:     &model.HistoryInfo{CachePath: "/Users/x/.npm/_npx/abc123", LastUsed: &lastUsed},
		Locations:   []model.Location{{Path: "/Users/x/.npm/_npx/abc123", Type: model.LocationCache}},
	}
	actions := KindActions(observation)

	info := actionFor(t, actions, ActionInfo, "npx cache entry")
	if info.Runnable() || info.Note != "npx cache entry" {
		t.Fatalf("npx info action = %#v", info)
	}
	wantCommand(t, actionFor(t, actions, ActionReveal, "Reveal in Finder"), "open", "-R", "/Users/x/.npm/_npx/abc123")
	for _, action := range actions {
		if action.Kind == ActionUpdate || action.Kind == ActionUninstall {
			t.Fatalf("a cache entry must not offer %s: %#v", action.Kind, action)
		}
	}
}

func TestKindActionsAreInertAndPickAPrimaryCommand(t *testing.T) {
	// KindActions builds strings only: an observation naming a command that
	// would be destructive if run still produces nothing but an argv slice,
	// and no action is ever pre-split through a shell.
	observation := model.Observation{
		DisplayName: "tool; rm -rf /",
		Kind:        model.KindPackage,
		Role:        model.RoleInstalled,
		Origin:      model.Origin{Provider: "npm", Manager: "global", Package: "tool; rm -rf /"},
	}
	actions := KindActions(observation)
	wantCommand(t, actions[0], "npm", "update", "-g", "tool; rm -rf /")

	command, ok := PrimaryActionCommand(observation)
	if !ok || strings.Join(command, " ") != "npm update -g tool; rm -rf /" {
		t.Fatalf("primary command = %#v ok=%v", command, ok)
	}

	// An observation with no location and no known manager offers nothing
	// runnable rather than inventing a command.
	bare := model.Observation{DisplayName: "mystery", Kind: model.KindExecutable, Role: model.RoleAvailable}
	if actions := KindActions(bare); len(actions) != 0 {
		t.Fatalf("bare observation actions = %s", formatActions(actions))
	}
	if _, ok := PrimaryActionCommand(bare); ok {
		t.Fatal("bare observation reported a primary command")
	}
}

func TestDetailViewSurfacesTypeAndActions(t *testing.T) {
	observation := model.Observation{
		DisplayName: "wget",
		CommandName: "wget",
		Kind:        model.KindPackage,
		Role:        model.RoleInstalled,
		Origin:      model.Origin{Provider: "homebrew", Manager: "formula", Package: "wget"},
		Version:     model.VersionInfo{State: model.VersionKnown, Value: "1.24.5"},
		Locations:   []model.Location{{Path: "/opt/homebrew/Cellar/wget", Type: model.LocationPackagePrefix}},
	}
	detail := BuildDetailViewModel(observation)
	if detail.TypeLabel != "Homebrew formula" || len(detail.Actions) == 0 {
		t.Fatalf("detail type=%q actions=%d", detail.TypeLabel, len(detail.Actions))
	}
	rendered := RenderDetailView(detail)
	for _, want := range []string{"Homebrew formula", "Actions", "brew upgrade wget", "brew uninstall wget", "Type:"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("detail output missing %q: %s", want, rendered)
		}
	}
}

func TestKindActionsOfferInstallForEveryManagerThatHasOne(t *testing.T) {
	tests := []struct {
		name        string
		observation model.Observation
		want        []string
	}{
		{
			name: "npm",
			observation: model.Observation{
				DisplayName: "opencode-ai", Kind: model.KindPackage, Role: model.RoleInstalled,
				Origin: model.Origin{Provider: "npm", Manager: "global", Package: "opencode-ai"},
			},
			want: []string{"npm", "i", "-g", "opencode-ai"},
		},
		{
			name: "brew formula",
			observation: model.Observation{
				DisplayName: "wget", Kind: model.KindPackage, Role: model.RoleInstalled,
				Origin: model.Origin{Provider: "homebrew", Manager: "formula", Package: "wget"},
			},
			want: []string{"brew", "install", "wget"},
		},
		{
			name: "brew cask",
			observation: model.Observation{
				DisplayName: "visual-studio-code", Kind: model.KindPackage, Role: model.RoleInstalled,
				Origin: model.Origin{Provider: "homebrew", Manager: "cask", Package: "visual-studio-code"},
			},
			want: []string{"brew", "install", "--cask", "visual-studio-code"},
		},
		{
			name: "cargo",
			observation: model.Observation{
				DisplayName: "ripgrep", Kind: model.KindExecutable, Role: model.RoleInstalled,
				Origin: model.Origin{Provider: "cargo", Manager: "install", Package: "ripgrep"},
			},
			want: []string{"cargo", "install", "ripgrep"},
		},
		{
			name: "pipx",
			observation: model.Observation{
				DisplayName: "ruff", Kind: model.KindCLI, Role: model.RoleInstalled,
				Origin: model.Origin{Provider: "pipx", Manager: "global", Package: "ruff"},
			},
			want: []string{"pipx", "install", "ruff"},
		},
	}
	for _, test := range tests {
		actions := KindActions(test.observation)
		wantCommand(t, actionFor(t, actions, ActionInstall, "Install"), test.want...)
		// The entry is already installed here, so the install line is a
		// reference and must not displace the command that acts on this copy.
		if actions[0].Kind == ActionInstall {
			t.Errorf("%s: install must not be the primary action for an installed entry: %s",
				test.name, formatActions(actions))
		}
	}
}

func TestKindActionsLeadWithInstallWhenNothingIsInstalledYet(t *testing.T) {
	// An available-but-not-installed npm package is exactly the case where
	// "how do I get this" is the whole question.
	observation := model.Observation{
		DisplayName: "opencode-ai", Kind: model.KindPackage, Role: model.RoleAvailable,
		Origin: model.Origin{Provider: "npm", Manager: "global", Package: "opencode-ai"},
	}
	actions := KindActions(observation)
	wantCommand(t, actions[0], "npm", "i", "-g", "opencode-ai")
	command, ok := PrimaryActionCommand(observation)
	if !ok || strings.Join(command, " ") != "npm i -g opencode-ai" {
		t.Fatalf("primary command = %#v ok=%v", command, ok)
	}
}

func TestKindActionsOfferNoInstallWhereNoManagerCould(t *testing.T) {
	// A .app you dragged in, a program you placed on your PATH, and an npx
	// cache entry have no manager that could be asked to install them.
	cases := map[string]model.Observation{
		"application": {
			DisplayName: "Ghostty", Kind: model.KindApplication, Role: model.RoleInstalled,
			Origin:    model.Origin{Provider: "applications"},
			Locations: []model.Location{{Path: "/Applications/Ghostty.app", Type: model.LocationApplication}},
		},
		"path executable": {
			DisplayName: "gh", Kind: model.KindExecutable, Role: model.RoleAvailable,
			Origin:    model.Origin{Provider: "unknown", Manager: "manual-or-unknown"},
			Locations: []model.Location{{Path: "/usr/local/bin/gh", Type: model.LocationExecutable}},
		},
		"npx cache entry": {
			DisplayName: "create-vite", Kind: model.KindHistory, Role: model.RoleHistory,
			Origin:  model.Origin{Provider: "npm", Manager: "npx", Package: "create-vite"},
			History: &model.HistoryInfo{CachePath: "/Users/x/.npm/_npx/abc123"},
		},
	}
	for name, observation := range cases {
		for _, action := range KindActions(observation) {
			if action.Kind == ActionInstall {
				t.Errorf("%s must not offer an install command: %#v", name, action)
			}
		}
	}
}

func TestDetailViewLeadsWithWhatThisIs(t *testing.T) {
	cask := model.Observation{
		DisplayName: "visual-studio-code", CommandName: "code",
		Kind: model.KindPackage, Role: model.RoleInstalled,
		Origin:    model.Origin{Provider: "homebrew", Manager: "cask", Package: "visual-studio-code"},
		Version:   model.VersionInfo{State: model.VersionKnown, Value: "1.90.0"},
		Locations: []model.Location{{Path: "/Applications/Visual Studio Code.app", Type: model.LocationApplication}},
	}
	detail := BuildDetailViewModel(cask)
	if want := "Homebrew cask — a macOS app installed via Homebrew Cask."; detail.WhatThisIs != want {
		t.Fatalf("detail.WhatThisIs = %q, want %q", detail.WhatThisIs, want)
	}
	rendered := RenderDetailView(detail)
	for _, want := range []string{
		"What this is", "Homebrew cask — a macOS app installed via Homebrew Cask.",
		"Actions", "brew upgrade --cask visual-studio-code", "brew install --cask visual-studio-code",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("detail output missing %q: %s", want, rendered)
		}
	}
	// The plain sentence has to come before the metadata a reader would need
	// the vocabulary to understand.
	if strings.Index(rendered, "What this is") > strings.Index(rendered, "Origin") {
		t.Errorf("the plain-English line is buried below the metadata: %s", rendered)
	}
}

func TestKindMarkerPairsGlyphWithLabelInBothProfiles(t *testing.T) {
	styles := ThemeStyles{Glyph: unicodeGlyphs}
	ascii := ThemeStyles{Glyph: asciiGlyphs}
	for kind, label := range map[model.ObservationKind]string{
		model.KindCLI:         "cli",
		model.KindPackage:     "pkg",
		model.KindApplication: "app",
		model.KindExecutable:  "exe",
		model.KindHistory:     "hist",
	} {
		unicodeMarker := styles.kindMarker(string(kind))
		asciiMarker := ascii.kindMarker(string(kind))
		if !strings.HasSuffix(unicodeMarker, label) || !strings.HasSuffix(asciiMarker, label) {
			t.Errorf("kind %q markers = %q / %q, both must end in %q", kind, unicodeMarker, asciiMarker, label)
		}
		if unicodeMarker == label || asciiMarker == label {
			t.Errorf("kind %q lost its glyph: %q / %q", kind, unicodeMarker, asciiMarker)
		}
	}
	// A change-event row carries no kind and must not be given one.
	if got := styles.kindMarker(""); got != "" {
		t.Fatalf("empty kind marker = %q", got)
	}
}
