package model

import "fmt"

// CapabilityName identifies a supported integration or behavior of an
// observed tool. Capability names are deliberately not inferred from a
// display name.
type CapabilityName string

const (
	CapabilityInteractiveCLI CapabilityName = "interactive-cli"
	CapabilityMCP            CapabilityName = "mcp"
	CapabilityGitHosting     CapabilityName = "git-hosting"
	CapabilityLSP            CapabilityName = "lsp"
	CapabilityVersionProbe   CapabilityName = "version-probe"
)

// CapabilityKind is a descriptive alias for callers that use kind-oriented
// terminology.
type CapabilityKind = CapabilityName

func (name CapabilityName) IsValid() bool {
	switch name {
	case CapabilityInteractiveCLI, CapabilityMCP, CapabilityGitHosting, CapabilityLSP, CapabilityVersionProbe:
		return true
	default:
		return false
	}
}

type CapabilityState string

const (
	CapabilityDetected CapabilityState = "detected"
)

func (state CapabilityState) IsValid() bool { return state == CapabilityDetected }

// CapabilityEvidence explains why an adapter matched. It is intentionally
// small and contains no raw command output.
type CapabilityEvidence struct {
	Type   string `json:"type"`
	Source string `json:"source"`
	Detail string `json:"detail,omitempty"`
}

func (e CapabilityEvidence) Validate() error {
	if e.Type == "" {
		return fmt.Errorf("capability evidence: type is required")
	}
	if e.Source == "" {
		return fmt.Errorf("capability evidence: source is required")
	}
	return nil
}

// Capability is the explicit result of matching one capability against an
// observation. A result exists only when supported evidence was found.
type Capability struct {
	Kind     CapabilityName       `json:"kind"`
	State    CapabilityState      `json:"state"`
	Evidence []CapabilityEvidence `json:"evidence,omitempty"`
}

func (c Capability) Validate() error {
	if !c.Kind.IsValid() {
		return fmt.Errorf("capability: invalid kind %q", c.Kind)
	}
	if !c.State.IsValid() {
		return fmt.Errorf("capability: invalid state %q", c.State)
	}
	for i, evidence := range c.Evidence {
		if err := evidence.Validate(); err != nil {
			return fmt.Errorf("capability: evidence %d: %w", i, err)
		}
	}
	return nil
}
