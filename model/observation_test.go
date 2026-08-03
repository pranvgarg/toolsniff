package model

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestVersionInfoValidateStates(t *testing.T) {
	tests := []struct {
		name    string
		version VersionInfo
		valid   bool
	}{
		{name: "known", version: VersionInfo{Value: "1.2.3", State: VersionKnown, Scheme: SchemeSemver, Comparable: true, Confidence: ConfidenceHigh}, valid: true},
		{name: "unknown", version: VersionInfo{State: VersionUnknown, Confidence: ConfidenceLow}, valid: true},
		{name: "not applicable", version: VersionInfo{State: VersionNotApplicable, Confidence: ConfidenceLow}, valid: true},
		{name: "not reported", version: VersionInfo{State: VersionNotReported, Confidence: ConfidenceLow}, valid: true},
		{name: "known without value", version: VersionInfo{State: VersionKnown, Confidence: ConfidenceHigh}, valid: false},
		{name: "unknown with value", version: VersionInfo{Value: "1", State: VersionUnknown, Confidence: ConfidenceLow}, valid: false},
		{name: "invalid state", version: VersionInfo{State: "guessed", Confidence: ConfidenceLow}, valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.version.Validate() == nil; got != test.valid {
				t.Errorf("Validate() == %v, want %v", got, test.valid)
			}
		})
	}
}

func TestObservationValidateRejectsInvalidHistoryBaseline(t *testing.T) {
	observation := validObservation()
	observation.Kind = KindHistory
	observation.Role = RoleInstalled
	observation.History = &HistoryInfo{}
	if err := observation.Validate(); err == nil {
		t.Fatal("Validate() accepted history observation with installed role")
	}
}

func TestObservationFromToolMovesLegacyHistoryDate(t *testing.T) {
	observation := ObservationFromTool(Tool{
		Name:    "create-vite",
		Source:  SourceNPXHistory,
		Role:    RoleHistory,
		Version: "2026-06-13",
	})

	if observation.Version.State != VersionNotApplicable || observation.Version.Value != "" {
		t.Fatalf("unexpected history version: %+v", observation.Version)
	}
	if observation.Package == nil || observation.Package.Version != "" {
		t.Fatalf("history date was copied into package version: %+v", observation.Package)
	}
	if observation.History == nil || observation.History.LastUsed == nil || observation.History.LastUsed.Format("2006-01-02") != "2026-06-13" {
		t.Fatalf("legacy date was not moved to history: %+v", observation.History)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("converted observation is invalid: %v", err)
	}
}

func TestObservationIdentityComponentsPreferStableIdentity(t *testing.T) {
	first := ObservationFromTool(Tool{Name: "gh", Source: SourceBrewFormula, Path: "/one/gh"})
	second := ObservationFromTool(Tool{Name: "gh", Source: SourceBrewFormula, Path: "/two/gh"})
	if first.ID != second.ID {
		t.Fatalf("package relocation changed identity: %q != %q", first.ID, second.ID)
	}
	if components := first.IdentityComponents(); components.Package != "gh" || components.Path != "" {
		t.Fatalf("unexpected package identity components: %+v", components)
	}

	pathObservation := ObservationFromTool(Tool{Name: "tool", Source: SourcePath, Path: "/bin/tool"})
	if components := pathObservation.IdentityComponents(); components.Path != "/bin/tool" {
		t.Fatalf("expected executable path identity, got %+v", components)
	}
}

func TestObservationJSONRoundTrip(t *testing.T) {
	seen := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	signed := true
	original := validObservation()
	original.FirstSeen = &seen
	original.LastSeen = &seen
	original.Application = &ApplicationInfo{BundleID: "com.example.tool", Signed: &signed}
	original.Evidence = []Evidence{{Type: "bundle-metadata", Source: "Info.plist", RetrievedAt: &seen}}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var decoded Observation
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("round trip mismatch: got %+v, want %+v", decoded, original)
	}
}

func validObservation() Observation {
	observation := Observation{
		ID:          "package\x00npm\x00global\x00tool",
		DisplayName: "tool",
		CommandName: "tool",
		Kind:        KindPackage,
		Role:        RoleInstalled,
		Origin:      Origin{Provider: "npm", Manager: "global", Package: "tool"},
		Version:     VersionInfo{Value: "1.2.3", State: VersionKnown, Scheme: SchemeSemver, Comparable: true, Confidence: ConfidenceHigh},
		Availability: AvailabilityInfo{
			State: AvailabilityUnknown,
		},
	}
	return observation
}
