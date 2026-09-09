package model

import "fmt"

// VersionState describes what is known about an observation's version.
type VersionState string

const (
	VersionKnown         VersionState = "known"
	VersionUnknown       VersionState = "unknown"
	VersionNotApplicable VersionState = "not-applicable"
	VersionNotReported   VersionState = "not-reported"
)

func (s VersionState) IsValid() bool {
	switch s {
	case VersionKnown, VersionUnknown, VersionNotApplicable, VersionNotReported:
		return true
	default:
		return false
	}
}

// VersionScheme describes how a known version should be compared.
type VersionScheme string

const (
	SchemeSemver  VersionScheme = "semver"
	SchemeCalver  VersionScheme = "calver"
	SchemeNumeric VersionScheme = "numeric"
	SchemeGit     VersionScheme = "git"
	SchemeOpaque  VersionScheme = "opaque"
	SchemeUnknown VersionScheme = "unknown"
)

func (s VersionScheme) IsValid() bool {
	switch s {
	case SchemeSemver, SchemeCalver, SchemeNumeric, SchemeGit, SchemeOpaque, SchemeUnknown:
		return true
	default:
		return false
	}
}

// Confidence describes the quality of the evidence, not whether a tool is
// trusted.
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

func (c Confidence) IsValid() bool {
	switch c {
	case ConfidenceHigh, ConfidenceMedium, ConfidenceLow:
		return true
	default:
		return false
	}
}

type VersionInfo struct {
	Value       string        `json:"value,omitempty"`
	State       VersionState  `json:"state"`
	Scheme      VersionScheme `json:"scheme,omitempty"`
	Comparable  bool          `json:"comparable"`
	Confidence  Confidence    `json:"confidence"`
	RetrievedBy string        `json:"retrieved_by,omitempty"`
}

// Validate enforces the distinction between a missing version and a known
// version. A scheme is optional because many sources report opaque values.
func (v VersionInfo) Validate() error {
	if !v.State.IsValid() {
		return fmt.Errorf("version: invalid state %q", v.State)
	}
	if v.Scheme != "" && !v.Scheme.IsValid() {
		return fmt.Errorf("version: invalid scheme %q", v.Scheme)
	}
	if !v.Confidence.IsValid() {
		return fmt.Errorf("version: invalid confidence %q", v.Confidence)
	}

	switch v.State {
	case VersionKnown:
		if v.Value == "" {
			return fmt.Errorf("version: known state requires a value")
		}
		if v.Comparable && (v.Scheme == "" || v.Scheme == SchemeUnknown || v.Scheme == SchemeOpaque) {
			return fmt.Errorf("version: comparable known value requires a comparable scheme")
		}
	default:
		if v.Value != "" {
			return fmt.Errorf("version: %s state cannot have a value", v.State)
		}
		if v.Comparable {
			return fmt.Errorf("version: %s state cannot be comparable", v.State)
		}
	}
	return nil
}
