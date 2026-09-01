package output

import (
	"github.com/pranvgarg/toolsniff/model"
)

// This file answers "what can I actually do with this thing?" for one
// observation. Everything here is pure string construction over metadata the
// scanners already captured: no process is started, no shell is involved, and
// no filesystem is touched. The caller decides whether to display, copy, or
// execute what it gets back. See DESIGN.md > Components > "Action row".

// Action kinds. These are the stable machine-readable discriminators; Label is
// the human string and may be reworded freely.
const (
	ActionInstall   = "install"
	ActionUpdate    = "update"
	ActionUninstall = "uninstall"
	ActionReveal    = "reveal"
	ActionOpen      = "open"
	ActionCopy      = "copy"
	ActionInfo      = "info"
)

// Action is one offered operation for an observation. A nil Command means the
// action carries information rather than something runnable — the renderer
// shows Note instead of a command line.
type Action struct {
	Label   string   // "Update", "Uninstall", "Reveal in Finder", "Open", "Copy path"
	Command []string // argv, already split; nil when not applicable
	Kind    string   // one of the Action* constants
	Note    string   // short why/what, e.g. "npm global package"
}

// Runnable reports whether the action carries an executable command line.
func (a Action) Runnable() bool { return len(a.Command) > 0 }

// KindActions returns the operations that make sense for an observation, in
// the order a user would most likely want them: change it, then find it, then
// copy it. It never executes anything.
func KindActions(observation model.Observation) []Action {
	name := actionPackageName(observation)
	note := ObservationTypeLabel(observation)
	provider, manager := observation.Origin.Provider, observation.Origin.Manager

	var actions []Action
	run := func(label, kind string, command ...string) {
		actions = append(actions, Action{Label: label, Command: command, Kind: kind, Note: note})
	}
	// The reveal target is chosen per kind but appended once, after the install
	// action has been placed, so "where is it" always sits below "change it".
	revealPath := ""

	switch {
	// npx history is a cache entry, not an installation: there is nothing to
	// update or uninstall, only somewhere to look.
	case observation.Kind == model.KindHistory || (provider == "npm" && manager == "npx"):
		actions = append(actions, Action{Label: "npx cache entry", Kind: ActionInfo, Note: note})
		if observation.History != nil {
			revealPath = observation.History.CachePath
		}

	case provider == "npm":
		run("Update", ActionUpdate, "npm", "update", "-g", name)
		run("Uninstall", ActionUninstall, "npm", "uninstall", "-g", name)
		run("Info", ActionInfo, "npm", "ls", "-g", name)
		revealPath = packagePrefixOrExecutable(observation)

	case provider == "pipx":
		run("Uninstall", ActionUninstall, "pipx", "uninstall", name)
		revealPath = packagePrefixOrExecutable(observation)

	case provider == "cargo":
		run("Uninstall", ActionUninstall, "cargo", "uninstall", name)
		revealPath = locationPath(observation, model.LocationExecutable, model.LocationPackagePrefix)

	case provider == "homebrew" && manager == "cask":
		run("Upgrade", ActionUpdate, "brew", "upgrade", "--cask", name)
		run("Uninstall", ActionUninstall, "brew", "uninstall", "--cask", name)
		revealPath = bundleOrLocationPath(observation)

	case provider == "homebrew":
		run("Upgrade", ActionUpdate, "brew", "upgrade", name)
		run("Uninstall", ActionUninstall, "brew", "uninstall", name)
		revealPath = locationPath(observation, model.LocationPackagePrefix, model.LocationExecutable)

	case provider == "applications" || observation.Kind == model.KindApplication:
		// The bundle path is the handle for both actions; the bundle ID is
		// shown in the detail pane and is not needed to open the app.
		bundle := bundleOrLocationPath(observation)
		if bundle != "" {
			run("Open", ActionOpen, "open", bundle)
		}
		revealPath = bundle

	// Everything else — a PATH executable, an unrecognised provider, or a
	// merely-available command — gets locate-and-copy, which always applies.
	default:
		revealPath = locationPath(observation, model.LocationExecutable, model.LocationPackagePrefix, model.LocationApplication, model.LocationCache)
	}

	if command := installCommand(observation, name); len(command) > 0 {
		install := Action{Label: "Install", Command: command, Kind: ActionInstall, Note: note}
		if observation.Role == model.RoleInstalled {
			// Already installed: the install line is a reference (what to run on
			// another machine, or after an uninstall), so it goes below the
			// commands that act on the copy actually present here.
			actions = append(actions, install)
		} else {
			// Not installed: this is the whole point of looking at the entry.
			actions = append([]Action{install}, actions...)
		}
	}

	if command, err := RevealLocationCommand(revealPath); err == nil {
		actions = append(actions, Action{Label: "Reveal in Finder", Command: command, Kind: ActionReveal, Note: note})
	}
	if path := CopySelectedPath(observation); path != "" {
		actions = append(actions, Action{Label: "Copy path", Kind: ActionCopy, Note: path})
	}
	return actions
}

// installCommand returns the manager's install invocation, or nil when the
// entry has no install path at all: a macOS .app you dragged in, a program you
// placed on your PATH yourself, and an npx cache entry are not things any
// manager can be asked to install.
func installCommand(observation model.Observation, name string) []string {
	return installCommandForOrigin(originKey(observation), name)
}

// installCommandForOrigin is the same table addressed by origin key alone, for
// the caller that has a manager in mind but no observation it installed: the
// Discover pane suggesting how an unmanaged binary could be brought under the
// manager that already owns most of the machine.
func installCommandForOrigin(origin, name string) []string {
	if name == "" {
		return nil
	}
	switch origin {
	case originNPM:
		return []string{"npm", "i", "-g", name}
	case originBrewFormula:
		return []string{"brew", "install", name}
	case originBrewCask:
		return []string{"brew", "install", "--cask", name}
	case originCargo:
		return []string{"cargo", "install", name}
	case originPipx:
		return []string{"pipx", "install", name}
	default:
		return nil
	}
}

// updatable reports whether any manager offers an in-place upgrade for this
// entry. It is the predicate behind ViewUpdates and the Health dashboard's
// first card, and it reads the action table rather than the observation because
// that is where the fact actually lives: model.VersionInfo records whether a
// version could be read, never whether a newer one exists, so "an update is
// waiting" is not a question anything on this machine can answer today. What
// can be answered -- "brew upgrade would work on this one" -- is this.
func updatable(observation model.Observation) bool {
	_, ok := PrimaryUpdateCommand(observation)
	return ok
}

// commandForKind returns the first runnable command KindActions offers under a
// given action kind. It is the shared body behind the verb-specific accessors
// below, so a key binding, the detail pane, and the updatable predicate all read
// the same table and cannot drift apart.
func commandForKind(observation model.Observation, kind string) ([]string, bool) {
	for _, action := range KindActions(observation) {
		if action.Kind == kind && action.Runnable() {
			return action.Command, true
		}
	}
	return nil, false
}

// PrimaryUpdateCommand is the in-place upgrade this entry's manager offers, or
// false when no manager on this machine can upgrade it -- an npx cache entry, a
// dragged-in .app, or anything a user put on their PATH themselves.
func PrimaryUpdateCommand(observation model.Observation) ([]string, bool) {
	return commandForKind(observation, ActionUpdate)
}

// PrimaryRemoveCommand is the uninstall this entry's manager offers. It is
// deliberately narrower than PrimaryActionCommand: "remove this" must never
// resolve to an upgrade or an open just because no uninstall exists.
func PrimaryRemoveCommand(observation model.Observation) ([]string, bool) {
	return commandForKind(observation, ActionUninstall)
}

// PrimaryAction returns the first runnable action KindActions offers. It is the
// single definition of "the one thing this entry is for", so the inline ACTION
// column, the detail pane, and the copy-the-command keybinding cannot drift.
func PrimaryAction(observation model.Observation) (Action, bool) {
	for _, action := range KindActions(observation) {
		if action.Runnable() {
			return action, true
		}
	}
	return Action{}, false
}

// PrimaryActionCommand returns the first runnable command KindActions offers,
// which is the one a "copy the command" keybinding should yank.
func PrimaryActionCommand(observation model.Observation) ([]string, bool) {
	action, ok := PrimaryAction(observation)
	if !ok {
		return nil, false
	}
	return action.Command, true
}

// PrimaryActionLabel is the human string for that same action, or "" when the
// entry has nothing runnable at all. List views show it verbatim rather than
// wording their own, so an inline label always names the command behind it.
func PrimaryActionLabel(observation model.Observation) string {
	action, ok := PrimaryAction(observation)
	if !ok {
		return ""
	}
	return action.Label
}

// ObservationTypeLabel is the human-readable "what is this" sentence derived
// from kind, origin, and role. It is what the detail pane leads with, and what
// every Action carries as its note.
func ObservationTypeLabel(observation model.Observation) string {
	provider, manager := observation.Origin.Provider, observation.Origin.Manager
	switch {
	case observation.Kind == model.KindHistory || (provider == "npm" && manager == "npx"):
		return "npx cache entry"
	case provider == "npm":
		return "npm global package"
	case provider == "pipx":
		return "pipx application"
	case provider == "cargo":
		return "Cargo binary"
	case provider == "bun":
		return "Bun global package"
	case provider == "homebrew" && manager == "cask":
		return "Homebrew cask"
	case provider == "homebrew" && manager == "formula":
		return "Homebrew formula"
	case provider == "homebrew":
		return "Homebrew package"
	case provider == "applications" || observation.Kind == model.KindApplication:
		return "macOS application"
	case observation.Role == model.RoleAvailable || observation.Kind == model.KindExecutable:
		return "PATH executable"
	case provider != "" && provider != "unknown":
		return provider + " " + string(observation.Kind)
	default:
		return string(observation.Kind)
	}
}

// actionPackageName picks the identifier a package manager would accept,
// preferring what the scanner recorded over the display name.
func actionPackageName(observation model.Observation) string {
	if observation.Origin.Package != "" {
		return observation.Origin.Package
	}
	if observation.Package != nil && observation.Package.Name != "" {
		return observation.Package.Name
	}
	if observation.CommandName != "" {
		return observation.CommandName
	}
	return observation.DisplayName
}

// locationPath returns the first recorded location matching any of types, in
// the order the caller listed them.
func locationPath(observation model.Observation, types ...model.LocationType) string {
	for _, want := range types {
		for _, location := range observation.Locations {
			if location.Type == want && location.Path != "" {
				return location.Path
			}
		}
	}
	return ""
}

// packagePrefixOrExecutable is the "where does this package live" answer used
// by the manager-backed reveal actions.
func packagePrefixOrExecutable(observation model.Observation) string {
	if observation.Package != nil && observation.Package.Prefix != "" {
		return observation.Package.Prefix
	}
	if path := locationPath(observation, model.LocationPackagePrefix, model.LocationVirtualEnv, model.LocationExecutable); path != "" {
		return path
	}
	if observation.Package != nil && len(observation.Package.Executables) > 0 {
		return observation.Package.Executables[0]
	}
	return ""
}

// bundleOrLocationPath resolves an application bundle path, falling back to
// any recorded location so a cask with only a prefix still reveals.
func bundleOrLocationPath(observation model.Observation) string {
	if path := locationPath(observation, model.LocationApplication); path != "" {
		return path
	}
	return locationPath(observation, model.LocationExecutable, model.LocationPackagePrefix)
}
