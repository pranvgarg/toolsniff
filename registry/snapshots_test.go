package registry

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pranvgarg/toolsniff/model"
)

func TestSnapshotSaveLoadRoundTripAndMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "snapshot.json")
	created := time.Date(2026, 8, 2, 12, 0, 0, 123, time.UTC)
	snapshot := Snapshot{
		Version:      "0.5.0",
		CreatedAt:    created,
		Observations: []model.Observation{snapshotObservation("zeta", "2.0.0"), snapshotObservation("alpha", "1.0.0")},
	}
	if err := SaveSnapshot(path, snapshot); err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	got, err := LoadSnapshot(path)
	if err != nil {
		t.Fatalf("LoadSnapshot failed: %v", err)
	}
	snapshot.SchemaVersion = SnapshotSchemaVersion
	if !reflect.DeepEqual(got, Snapshot{SchemaVersion: SnapshotSchemaVersion, Version: snapshot.Version, CreatedAt: created, Observations: []model.Observation{snapshotObservation("alpha", "1.0.0"), snapshotObservation("zeta", "2.0.0")}}) {
		t.Fatalf("round trip mismatch: got %+v", got)
	}
	mode, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat snapshot: %v", err)
	}
	if mode.Mode().Perm() != 0o600 {
		t.Errorf("snapshot permissions = %o, want 600", mode.Mode().Perm())
	}
}

func TestSnapshotListingIsRetentionSafeAndDeterministic(t *testing.T) {
	dir := t.TempDir()
	older := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	if err := SaveSnapshot(filepath.Join(dir, "snapshot-older.json"), Snapshot{CreatedAt: older, Observations: []model.Observation{snapshotObservation("old", "1")}}); err != nil {
		t.Fatalf("saving older snapshot: %v", err)
	}
	if err := SaveSnapshot(filepath.Join(dir, "snapshot-newer.json"), Snapshot{CreatedAt: newer, Observations: []model.Observation{snapshotObservation("new", "2")}}); err != nil {
		t.Fatalf("saving newer snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "snapshot-corrupt.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("writing corrupt snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".snapshot-partial.tmp"), []byte("{"), 0o600); err != nil {
		t.Fatalf("writing temporary snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("writing unrelated file: %v", err)
	}

	got, err := ListSnapshots(dir, 1)
	if err != nil {
		t.Fatalf("ListSnapshots failed: %v", err)
	}
	if len(got) != 1 || !got[0].CreatedAt.Equal(newer) {
		t.Fatalf("unexpected retained listing: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "snapshot-corrupt.json")); err != nil {
		t.Fatalf("listing removed corrupt snapshot: %v", err)
	}

	all, err := ListSnapshots(dir, 0)
	if err != nil || len(all) != 2 || !all[0].CreatedAt.Equal(newer) || !all[1].CreatedAt.Equal(older) {
		t.Fatalf("unexpected full listing: %+v, err=%v", all, err)
	}
}

func TestSnapshotCorruptionAndUnsupportedVersionAreRejected(t *testing.T) {
	dir := t.TempDir()
	corrupt := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(corrupt, []byte("not json"), 0o600); err != nil {
		t.Fatalf("writing corrupt snapshot: %v", err)
	}
	if _, err := LoadSnapshot(corrupt); err == nil {
		t.Fatal("LoadSnapshot accepted corrupt JSON")
	}
	unsupported := filepath.Join(dir, "unsupported.json")
	data := `{"schema_version":99,"created_at":"2026-08-02T12:00:00Z","observations":[]}`
	if err := os.WriteFile(unsupported, []byte(data), 0o600); err != nil {
		t.Fatalf("writing unsupported snapshot: %v", err)
	}
	if _, err := LoadSnapshot(unsupported); err == nil {
		t.Fatal("LoadSnapshot accepted unsupported schema")
	}
}

func TestSnapshotFailedSavePreservesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	original := Snapshot{CreatedAt: time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC), Observations: []model.Observation{snapshotObservation("stable", "1")}}
	if err := SaveSnapshot(path, original); err != nil {
		t.Fatalf("initial save failed: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading original snapshot: %v", err)
	}
	invalid := original
	invalid.Observations = []model.Observation{{DisplayName: "invalid"}}
	if err := SaveSnapshot(path, invalid); err == nil {
		t.Fatal("invalid snapshot save unexpectedly succeeded")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading preserved snapshot: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("failed snapshot save replaced the existing file")
	}
	if strings.Contains(string(after), "invalid") {
		t.Fatal("failed snapshot data leaked into existing file")
	}

	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatalf("creating blocked target: %v", err)
	}
	if err := SaveSnapshot(blocked, original); err == nil {
		t.Fatal("SaveSnapshot unexpectedly replaced a directory target")
	}
	if info, err := os.Stat(blocked); err != nil || !info.IsDir() {
		t.Fatalf("atomic failure damaged directory target: info=%+v err=%v", info, err)
	}
}

func snapshotObservation(name, version string) model.Observation {
	observation := model.Observation{
		DisplayName: name,
		CommandName: name,
		Kind:        model.KindPackage,
		Role:        model.RoleInstalled,
		Origin:      model.Origin{Provider: "npm", Manager: "global", Package: name},
		Version:     model.VersionInfo{Value: version, State: model.VersionKnown, Scheme: model.SchemeSemver, Comparable: true, Confidence: model.ConfidenceHigh},
		Locations:   []model.Location{{Path: "/opt/" + name, Type: model.LocationPackagePrefix}},
		Availability: model.AvailabilityInfo{
			State: model.AvailabilityAvailable,
		},
	}
	observation.ID = model.ObservationIdentity(observation)
	return observation
}
