package registry

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pranvgarg/toolsniff/model"
)

func TestSaveWritesV2EnvelopeAndLoadPreservesLegacyCompatibility(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "registry.json")
	tools := []model.Tool{
		{Name: "gh", Source: model.SourceBrewFormula, Version: "2.1.0", Role: model.RoleInstalled},
		{Name: "ollama", Source: model.SourceApplications, Path: "/Applications/Ollama.app", Role: model.RoleInstalled},
	}

	if err := Save(path, tools); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved registry: %v", err)
	}
	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("saved registry is not JSON: %v", err)
	}
	if envelope.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", envelope.SchemaVersion, CurrentSchemaVersion)
	}
	if len(envelope.Observations) != len(tools) {
		t.Fatalf("saved %d observations, want %d", len(envelope.Observations), len(tools))
	}

	observations, warning := LoadObservations(path)
	if warning != "" {
		t.Fatalf("LoadObservations warning = %q", warning)
	}
	if !reflect.DeepEqual(observations, observationsFromTools(tools)) {
		t.Fatalf("loaded observations = %+v, want %+v", observations, observationsFromTools(tools))
	}
	loadedTools, warning := Load(path)
	if warning != "" {
		t.Fatalf("Load warning = %q", warning)
	}
	if !reflect.DeepEqual(loadedTools, tools) {
		t.Fatalf("loaded tools = %+v, want %+v", loadedTools, tools)
	}
}

func TestLoadObservationsMigratesV1Deterministically(t *testing.T) {
	tools := []model.Tool{
		{Name: "create-vite", Source: model.SourceNPXHistory, Role: model.RoleHistory, Version: "2026-06-13", Path: "/cache/create-vite"},
		{Name: "tool", Source: model.SourcePath, Role: model.RoleAvailable, Path: "/usr/local/bin/tool"},
		{Name: "gh", Source: model.SourceBrewFormula, Role: model.RoleInstalled, Version: "2.1.0"},
	}
	legacy, err := json.Marshal(tools)
	if err != nil {
		t.Fatalf("marshaling v1 fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatalf("writing v1 fixture: %v", err)
	}

	got, warning := LoadObservations(path)
	if warning != "" {
		t.Fatalf("LoadObservations warning = %q", warning)
	}
	want := MigrateTools(tools)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("migrated observations = %+v, want %+v", got, want)
	}
	if got[0].ID >= got[len(got)-1].ID {
		t.Fatalf("migrated observations are not deterministic: %+v", got)
	}

	bySource := make(map[string]model.Observation)
	for _, observation := range got {
		bySource[observation.Origin.Provider] = observation
	}
	history := bySource["npm"]
	if history.Version.State != model.VersionNotApplicable || history.Version.Value != "" {
		t.Fatalf("history version = %+v, want not-applicable without value", history.Version)
	}
	if history.History == nil || history.History.LastUsed == nil || history.History.LastUsed.Format("2006-01-02") != "2026-06-13" {
		t.Fatalf("history metadata = %+v, want migrated last-used date", history.History)
	}
	pathObservation := bySource["unknown"]
	if pathObservation.Version.State != model.VersionUnknown {
		t.Fatalf("empty executable version state = %q, want %q", pathObservation.Version.State, model.VersionUnknown)
	}
	if pathObservation.Availability.State != model.AvailabilityAvailable {
		t.Fatalf("path availability = %q, want %q", pathObservation.Availability.State, model.AvailabilityAvailable)
	}

	reversed := append([]model.Tool(nil), tools...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	otherPath := filepath.Join(t.TempDir(), "registry.json")
	otherLegacy, err := json.Marshal(reversed)
	if err != nil {
		t.Fatalf("marshaling reversed v1 fixture: %v", err)
	}
	if err := os.WriteFile(otherPath, otherLegacy, 0o600); err != nil {
		t.Fatalf("writing reversed v1 fixture: %v", err)
	}
	other, warning := LoadObservations(otherPath)
	if warning != "" {
		t.Fatalf("reversed LoadObservations warning = %q", warning)
	}
	if !reflect.DeepEqual(other, got) {
		t.Fatalf("migration depends on input order: got %+v, want %+v", other, got)
	}
}

func TestMigratedObservationsUseObservationDiffContract(t *testing.T) {
	oldTools := []model.Tool{{Name: "tool", Source: model.SourceNPM, Version: "1.0.0", Role: model.RoleInstalled}}
	newTools := []model.Tool{{Name: "tool", Source: model.SourceNPM, Version: "2.0.0", Role: model.RoleInstalled}}
	old := MigrateTools(oldTools)
	current := MigrateTools(newTools)

	diff := ComputeObservationDiff(old, current)
	if len(diff.Updated) != 1 || len(diff.Added) != 0 || len(diff.Removed) != 0 {
		t.Fatalf("unexpected migrated observation diff: %+v", diff)
	}
	if diff.Updated[0].Before.Version.Value != "1.0.0" || diff.Updated[0].After.Version.Value != "2.0.0" {
		t.Fatalf("unexpected version update: %+v", diff.Updated[0])
	}
}

func TestSaveObservationsRoundTripPreservesV2Data(t *testing.T) {
	seen := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	observation := model.Observation{
		DisplayName: "tool",
		CommandName: "tool",
		Kind:        model.KindPackage,
		Role:        model.RoleInstalled,
		Origin:      model.Origin{Provider: "npm", Manager: "global", Package: "tool", Manifest: "/tmp/package.json"},
		Version: model.VersionInfo{
			Value:       "1.2.3",
			State:       model.VersionKnown,
			Scheme:      model.SchemeSemver,
			Comparable:  true,
			Confidence:  model.ConfidenceHigh,
			RetrievedBy: "package-manager",
		},
		Locations: []model.Location{{Path: "/opt/tool", Type: model.LocationPackagePrefix, Executable: true}},
		Availability: model.AvailabilityInfo{
			State:        model.AvailabilityAvailable,
			ResolvedPath: "/opt/bin/tool",
			PATHIndex:    2,
			ShadowedBy:   []string{"/usr/local/bin/tool"},
		},
		Package:   &model.PackageInfo{Name: "tool", Version: "1.2.3", Executables: []string{"tool"}, Prefix: "/opt"},
		Evidence:  []model.Evidence{{Type: "package-metadata", Source: "npm ls", Description: "global package", RetrievedAt: &seen}},
		FirstSeen: &seen,
		LastSeen:  &seen,
	}
	observation.ID = model.ObservationIdentity(observation)
	path := filepath.Join(t.TempDir(), "registry.json")

	if err := SaveObservations(path, []model.Observation{observation}); err != nil {
		t.Fatalf("SaveObservations failed: %v", err)
	}
	got, warning := LoadObservations(path)
	if warning != "" {
		t.Fatalf("LoadObservations warning = %q", warning)
	}
	if !reflect.DeepEqual(got, []model.Observation{observation}) {
		t.Fatalf("round-trip observations = %+v, want %+v", got, []model.Observation{observation})
	}
}

func TestRegistrySaveIsIdempotent(t *testing.T) {
	tools := []model.Tool{
		{Name: "z-tool", Source: model.SourceNPM, Version: "1.0.0", Role: model.RoleInstalled},
		{Name: "a-tool", Source: model.SourceBrewFormula, Role: model.RoleInstalled},
	}
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := SaveObservations(path, MigrateTools(tools)); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading first save: %v", err)
	}
	observations, warning := LoadObservations(path)
	if warning != "" {
		t.Fatalf("LoadObservations warning = %q", warning)
	}
	if err := SaveObservations(path, observations); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading second save: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("re-saving a v2 registry changed bytes:\nfirst=%s\nsecond=%s", first, second)
	}
}

func TestLoadWarningsDoNotRewriteUnsupportedOrCorruptFiles(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "corrupt JSON", data: "{not valid json"},
		{name: "unsupported schema", data: `{"schema_version":3,"observations":[]}`},
		{name: "unknown v2 field", data: `{"schema_version":2,"observations":[],"future":true}`},
		{name: "invalid v2 observation", data: `{"schema_version":2,"observations":[{"id":"x"}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "registry.json")
			original := []byte(test.data)
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatalf("writing fixture: %v", err)
			}
			observations, warning := LoadObservations(path)
			if len(observations) != 0 {
				t.Fatalf("loaded observations = %+v, want empty", observations)
			}
			if warning == "" {
				t.Fatal("expected a warning")
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading fixture after load: %v", err)
			}
			if !bytes.Equal(got, original) {
				t.Fatalf("Load rewrote unsupported/corrupt file: got %q, want %q", got, original)
			}
		})
	}
}

func TestSaveObservationsPreservesPermissionsAndOldFileOnAtomicFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "registry.json")
	if err := Save(path, []model.Tool{{Name: "gh", Source: model.SourceBrewFormula, Role: model.RoleInstalled}}); err != nil {
		t.Fatalf("initial Save failed: %v", err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat saved registry: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Errorf("registry permissions = %o, want 600", got)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat registry directory: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("registry directory permissions = %o, want 700", got)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading old registry: %v", err)
	}
	if err := Save(path, []model.Tool{{Name: "new", Source: model.SourceBrewFormula, Role: model.RoleInstalled}}); err != nil {
		t.Fatalf("replacement Save failed: %v", err)
	}
	if after, err := os.ReadFile(path); err != nil {
		t.Fatalf("reading replaced registry: %v", err)
	} else if bytes.Equal(after, before) {
		t.Fatal("successful atomic replacement did not update registry")
	}

	target := filepath.Join(dir, "blocked")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatalf("creating blocked target: %v", err)
	}
	if err := Save(target, []model.Tool{{Name: "should-not-replace", Source: model.SourceNPM, Role: model.RoleInstalled}}); err == nil {
		t.Fatal("Save unexpectedly replaced a directory target")
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Fatalf("atomic failure damaged existing target: info=%+v err=%v", info, err)
	}
}

func TestInstalledAndAvailabilityRegistriesRemainSeparateV2Files(t *testing.T) {
	dir := t.TempDir()
	installedPath := filepath.Join(dir, "registry.json")
	availabilityPath := AvailabilityPath(installedPath)
	installed := model.ObservationFromTool(model.Tool{Name: "gh", Source: model.SourceBrewFormula, Role: model.RoleInstalled})
	available := model.ObservationFromTool(model.Tool{Name: "gh", Source: model.SourcePath, Role: model.RoleAvailable, Path: "/opt/bin/gh"})

	if err := SaveObservations(installedPath, []model.Observation{installed}); err != nil {
		t.Fatalf("saving installed observations: %v", err)
	}
	if err := SaveObservations(availabilityPath, []model.Observation{available}); err != nil {
		t.Fatalf("saving available observations: %v", err)
	}
	gotInstalled, installedWarning := LoadObservations(installedPath)
	gotAvailable, availableWarning := LoadObservations(availabilityPath)
	if installedWarning != "" || availableWarning != "" {
		t.Fatalf("unexpected warnings: installed=%q available=%q", installedWarning, availableWarning)
	}
	if len(gotInstalled) != 1 || gotInstalled[0].Role != model.RoleInstalled {
		t.Fatalf("unexpected installed observations: %+v", gotInstalled)
	}
	if len(gotAvailable) != 1 || gotAvailable[0].Role != model.RoleAvailable {
		t.Fatalf("unexpected availability observations: %+v", gotAvailable)
	}
	if strings.Contains(string(mustReadFile(t, installedPath)), "/opt/bin/gh") {
		t.Fatal("availability observation leaked into installed registry")
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return data
}
