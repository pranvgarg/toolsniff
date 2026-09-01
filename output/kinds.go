package output

import (
	"strings"

	"github.com/pranvgarg/toolsniff/model"
)

// This file is the single source of truth for the plain-English vocabulary the
// TUI speaks. Every user-facing noun ("CLI tool", "package", "On your PATH")
// and every one-sentence explanation of it lives here, keyed by kind and by
// origin, so the tab strip, the overview, the sub-group headers, and the detail
// pane cannot drift into three different names for the same thing.
//
// The rule this file exists to enforce: never show the bare words "installed"
// or "available" without their plain meaning next to them. "available" in
// particular is a scanner word, not a user word -- to a user that set is "on
// your PATH, not installed by a manager". See DESIGN.md > Components >
// "What this is line".

// KindDefinition is what one sort of entry is, in words a user who has never
// heard of a package manager can still read.
type KindDefinition struct {
	// Label is the short plural shown in tabs and in the overview.
	Label string
	// Singular names one entry, for sentences like "1 npm package".
	Singular string
	// PlainMeaning is one jargon-free sentence. It always ends in a period so
	// callers can concatenate it without repairing punctuation.
	PlainMeaning string
}

// kindDefinitions answers the question from the kind alone. It is the fallback
// used when the origin is unknown or unrecognised; originDefinitions below is
// preferred whenever the scanner recorded who installed the thing.
var kindDefinitions = map[model.ObservationKind]KindDefinition{
	model.KindCLI: {
		Label:        "CLI tools",
		Singular:     "CLI tool",
		PlainMeaning: "A command you run in the terminal, installed by a package manager.",
	},
	model.KindPackage: {
		Label:        "Packages",
		Singular:     "package",
		PlainMeaning: "A package a manager installed for you; it may or may not add a terminal command.",
	},
	model.KindApplication: {
		Label:        "Applications",
		Singular:     "application",
		PlainMeaning: "A macOS .app you open from Finder or Spotlight.",
	},
	model.KindExecutable: {
		Label:        "PATH executables",
		Singular:     "PATH executable",
		PlainMeaning: "A program on your PATH not installed by a known manager — one you placed yourself.",
	},
	model.KindHistory: {
		Label:        "npx history",
		Singular:     "npx history entry",
		PlainMeaning: "A package npx ran once and cached; not permanently installed.",
	},
}

// Origin keys. These name "who put this here" and are the second axis of the
// definition table: a Homebrew cask and a Homebrew formula are the same kind
// but are not the same thing to a user.
const (
	originNPM          = "npm"
	originBrewFormula  = "brew-formula"
	originBrewCask     = "brew-cask"
	originCargo        = "cargo"
	originPipx         = "pipx"
	originBun          = "bun"
	originApplications = "applications"
	originPath         = "path"
	originNPX          = "npx"
)

var originDefinitions = map[string]KindDefinition{
	originNPM: {
		Label:        "npm",
		Singular:     "npm package",
		PlainMeaning: "A Node.js package installed globally with npm.",
	},
	originBrewFormula: {
		Label:        "Homebrew formula",
		Singular:     "Homebrew formula",
		PlainMeaning: "A command-line program installed via Homebrew.",
	},
	originBrewCask: {
		Label:        "Homebrew cask",
		Singular:     "Homebrew cask",
		PlainMeaning: "A macOS app installed via Homebrew Cask.",
	},
	originCargo: {
		Label:        "Cargo",
		Singular:     "Cargo binary",
		PlainMeaning: "A Rust binary installed with cargo install.",
	},
	originPipx: {
		Label:        "pipx",
		Singular:     "pipx app",
		PlainMeaning: "A Python app in its own environment, installed via pipx.",
	},
	originBun: {
		Label:        "Bun",
		Singular:     "Bun package",
		PlainMeaning: "A package installed globally with Bun.",
	},
	originApplications: {
		Label:        "Applications",
		Singular:     "application",
		PlainMeaning: "A macOS .app you open from Finder or Spotlight.",
	},
	originPath: {
		Label:        "On your PATH",
		Singular:     "PATH executable",
		PlainMeaning: "A program on your PATH not installed by a known manager — one you placed yourself.",
	},
	originNPX: {
		Label:        "npx history",
		Singular:     "npx history entry",
		PlainMeaning: "A package npx ran once and cached; not permanently installed.",
	},
}

// originKey classifies an observation by who installed it. It mirrors
// ObservationSource's provider/manager reading, but collapses to the nine
// origins the vocabulary actually distinguishes rather than to a source label.
func originKey(observation model.Observation) string {
	provider, manager := observation.Origin.Provider, observation.Origin.Manager
	switch {
	case observation.Kind == model.KindHistory || (provider == "npm" && manager == "npx"):
		return originNPX
	case provider == "npm":
		return originNPM
	case provider == "pipx":
		return originPipx
	case provider == "cargo":
		return originCargo
	case provider == "bun":
		return originBun
	case provider == "homebrew" && manager == "cask":
		return originBrewCask
	case provider == "homebrew":
		return originBrewFormula
	case provider == "applications" || observation.Kind == model.KindApplication:
		return originApplications
	default:
		return originPath
	}
}

// KindDefinitionFor returns the definition for a bare kind. An unrecognised
// kind gets an empty definition rather than another kind's meaning.
func KindDefinitionFor(kind model.ObservationKind) KindDefinition {
	return kindDefinitions[kind]
}

// KindDefinitionForOrigin returns the definition for one of the origin keys.
func KindDefinitionForOrigin(origin string) KindDefinition {
	return originDefinitions[origin]
}

// KindDefinitionForObservation is the precise answer: the origin's definition
// when the scanner knows who installed the thing, the kind's otherwise. An
// unmanaged PATH executable resolves through originPath, which says exactly
// that, so the two tables agree on the one case they share.
func KindDefinitionForObservation(observation model.Observation) KindDefinition {
	key := originKey(observation)
	// A managed provider always wins: "Homebrew cask" is more useful than
	// "package". originPath is only trusted when nothing else claims the entry.
	if key != originPath {
		if definition, ok := originDefinitions[key]; ok {
			return definition
		}
	}
	if definition, ok := kindDefinitions[observation.Kind]; ok {
		return definition
	}
	return originDefinitions[originPath]
}

// PlainMeaning is the one-sentence explanation for an observation.
func PlainMeaning(observation model.Observation) string {
	return KindDefinitionForObservation(observation).PlainMeaning
}

// WhatThisIsLine is the detail pane's lead sentence: the precise type name,
// then the plain meaning, e.g.
//
//	Homebrew cask — a macOS app installed via Homebrew Cask.
//
// The type name comes from ObservationTypeLabel so the detail pane and every
// Action note make the same claim.
func WhatThisIsLine(observation model.Observation) string {
	meaning := PlainMeaning(observation)
	label := ObservationTypeLabel(observation)
	switch {
	case meaning == "":
		return label
	case label == "":
		return meaning
	default:
		return label + " — " + lowerFirst(meaning)
	}
}

// lowerFirst lowercases a leading capital so a standalone sentence can be
// spliced after an em dash. A word that is entirely uppercase (an acronym like
// PATH) is left alone.
func lowerFirst(sentence string) string {
	fields := strings.Fields(sentence)
	if len(fields) == 0 {
		return sentence
	}
	first := fields[0]
	if first == strings.ToUpper(first) && first != strings.ToLower(first) && len([]rune(first)) > 1 {
		return sentence
	}
	runes := []rune(sentence)
	runes[0] = []rune(strings.ToLower(string(runes[0])))[0]
	return string(runes)
}

// managerInstalled reports whether a known package manager put the entry on
// this machine. It is what separates "a program you placed on your PATH
// yourself" from "a binary cargo installed".
func managerInstalled(observation model.Observation) bool {
	provider, manager := observation.Origin.Provider, observation.Origin.Manager
	if provider == "" || provider == "unknown" {
		return false
	}
	return manager != "manual-or-unknown"
}

// observationMatchesView is the single predicate behind both the kind-first
// tabs and the overview counts, so a tab's count can never disagree with the
// rows the tab shows. Every observation matches at least one of the five kind
// views, which is what makes the reorganisation lossless.
func observationMatchesView(observation model.Observation, view ViewCategory) bool {
	switch view {
	case ViewCLI:
		return observation.Kind == model.KindCLI
	case ViewPackages:
		// A cargo or bun binary is recorded as an executable but is emphatically
		// a managed install, so it belongs with the packages rather than with
		// the hand-placed programs.
		if observation.Kind == model.KindPackage {
			return true
		}
		return observation.Kind == model.KindExecutable &&
			managerInstalled(observation) &&
			observation.Role != model.RoleAvailable
	case ViewApplications:
		return observation.Kind == model.KindApplication
	case ViewPathExecutables:
		return observation.Role == model.RoleAvailable ||
			(observation.Kind == model.KindExecutable && !managerInstalled(observation))
	case ViewNpxHistory:
		return observation.Kind == model.KindHistory
	case ViewInstalled:
		return observation.Role == model.RoleInstalled
	case ViewAvailable:
		return observation.Role == model.RoleAvailable
	case ViewHistory:
		return observation.Role == model.RoleHistory || observation.Kind == model.KindHistory
	default:
		return true
	}
}

// kindViews are the primary, "what is this thing" views. Together they cover
// every observation in a report.
var kindViews = []ViewCategory{ViewCLI, ViewPackages, ViewApplications, ViewPathExecutables, ViewNpxHistory}

// ViewLabel is the short human title for a view, used by the tab strip and the
// pane caption.
func ViewLabel(view ViewCategory) string {
	switch view {
	case ViewOverview:
		return "Overview"
	case ViewCLI:
		return "CLI tools"
	case ViewPackages:
		return "Packages"
	case ViewApplications:
		return "Applications"
	case ViewPathExecutables, ViewAvailable:
		return "On your PATH"
	case ViewNpxHistory, ViewHistory:
		return "npx history"
	case ViewChanges:
		return "Changes"
	case ViewIssues:
		return "Issues"
	case ViewInstalled:
		return "Installed"
	case ViewManage:
		return "Manage"
	case ViewDiscover:
		return "Discover"
	case ViewReview:
		return "Review"
	case ViewHealth:
		return "Health"
	default:
		return "Everything"
	}
}

// ViewMeaning is the plain-English gloss shown next to the label. This is the
// mechanism that keeps "available" from ever appearing on its own.
func ViewMeaning(view ViewCategory) string {
	switch view {
	case ViewOverview:
		return "what's on this machine"
	case ViewCLI:
		return "commands you run in the terminal, installed by a package manager"
	case ViewPackages:
		return "installed for you by a package manager"
	case ViewApplications:
		return "macOS apps you open from Finder or Spotlight"
	case ViewPathExecutables, ViewAvailable:
		return "not installed by a manager — you placed these yourself"
	case ViewNpxHistory, ViewHistory:
		return "run once via npx, not permanently installed"
	case ViewChanges:
		return "since the last saved baseline"
	case ViewIssues:
		return "commands that are broken or shadowed by another copy"
	case ViewInstalled:
		return "everything a package manager put on this machine"
	case ViewManage:
		return "everything installed by a manager, in one place"
	case ViewDiscover:
		return "what's on your PATH that no manager installed"
	case ViewReview:
		return "what changed and what needs fixing"
	case ViewHealth:
		return "updates, issues, and reclaimable space at a glance"
	default:
		return "every command, package, and app found"
	}
}

// StatusDisplayLabel is the human wording for a status in the TUI's own
// surfaces. "available" is the scanner's word for "your shell can run this, but
// no manager installed it" -- shown alone in a ten-cell column it reads as
// "available to install", which is the opposite of the truth.
//
// This is display only: ObservationStatus, InventoryRow.Status, the JSON, the
// plain-text table, and the `status:` filter facet all keep the raw vocabulary,
// so nothing machine-readable moves.
func StatusDisplayLabel(status string) string {
	switch Status(status) {
	case StatusAvailable:
		return "on PATH"
	case StatusHistory:
		return "npx cache"
	default:
		return status
	}
}

// ViewCaption is the pane's first line: title, count, meaning.
func ViewCaption(view ViewCategory, count int) string {
	return ViewLabel(view) + " · " + itoa(count) + " · " + ViewMeaning(view)
}

// --- manager sub-groups ----------------------------------------------------
//
// Inside the CLI-tools and Packages panes the rows are grouped by the manager
// that installed them, because "79 packages" is not a fact anyone can act on
// and "60 Homebrew formulae, 13 npm packages" is.

// sourceGroupLabels maps a row's source label to its plural group heading.
var sourceGroupLabels = map[string]string{
	model.SourceNPM:          "npm packages",
	model.SourceBrewFormula:  "Homebrew formulae",
	model.SourceBrewCask:     "Homebrew casks",
	model.SourcePipx:         "pipx apps",
	model.SourceCargo:        "Cargo binaries",
	model.SourceBun:          "Bun packages",
	model.SourceApplications: "Applications",
	model.SourcePath:         "On your PATH (not installed by a manager)",
	model.SourceNPXHistory:   "npx history",
}

// sourceGroupMeanings is the short gloss appended to a group heading when the
// pane is wide enough to carry it.
var sourceGroupMeanings = map[string]string{
	model.SourceNPM:          "Node.js packages installed globally",
	model.SourceBrewFormula:  "command-line programs installed via Homebrew",
	model.SourceBrewCask:     "macOS apps installed via Homebrew Cask",
	model.SourcePipx:         "Python apps in their own environments",
	model.SourceCargo:        "Rust binaries installed with cargo install",
	model.SourceBun:          "packages installed globally with Bun",
	model.SourceApplications: "macOS .app bundles",
	model.SourcePath:         "you placed these yourself",
	model.SourceNPXHistory:   "run once via npx, not installed",
}

// sourceGroupOrder puts the groups in the order a user cares about them:
// the manager that owns the most of a machine first, the residue last.
var sourceGroupOrder = map[string]int{
	model.SourceNPM:          1,
	model.SourceBrewFormula:  2,
	model.SourceBrewCask:     3,
	model.SourceCargo:        4,
	model.SourcePipx:         5,
	model.SourceBun:          6,
	model.SourceApplications: 7,
	model.SourcePath:         8,
	model.SourceNPXHistory:   9,
}

// sourceGroupKey normalizes a row's source to a known group. A provider the
// vocabulary has not been taught (ObservationSource renders those as
// "provider-manager") is folded onto its provider rather than becoming its own
// singleton group for every manager variant.
func sourceGroupKey(source string) string {
	if _, ok := sourceGroupLabels[source]; ok {
		return source
	}
	if provider, _, ok := strings.Cut(source, "-"); ok {
		if _, ok := sourceGroupLabels[provider]; ok {
			return provider
		}
		return provider
	}
	return source
}

func sourceGroupLabel(source string) string {
	key := sourceGroupKey(source)
	if label, ok := sourceGroupLabels[key]; ok {
		return label
	}
	if key == "" {
		return "Other"
	}
	return key
}

func sourceGroupMeaning(source string) string {
	return sourceGroupMeanings[sourceGroupKey(source)]
}

func sourceGroupRank(source string) int {
	if rank, ok := sourceGroupOrder[sourceGroupKey(source)]; ok {
		return rank
	}
	return 99
}

// groupedView reports whether a view renders manager sub-group headings.
func groupedView(view ViewCategory) bool {
	return view == ViewCLI || view == ViewPackages
}
