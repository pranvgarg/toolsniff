package model

// SourceRole describes what a scanner observation means. A PATH observation
// proves availability, but not how a tool was installed.
type SourceRole string

const (
	RoleInstalled SourceRole = "installed"
	RoleAvailable SourceRole = "available"
	RoleHistory   SourceRole = "history"
)

const (
	SourceNPM          = "npm"
	SourceNPXHistory   = "npx-history"
	SourceBrewFormula  = "brew-formula"
	SourceBrewCask     = "brew-cask"
	SourcePipx         = "pipx"
	SourceCargo        = "cargo"
	SourceBun          = "bun"
	SourceApplications = "applications"
	SourcePath         = "path"
)
