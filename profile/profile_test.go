package profile_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/output"
	"github.com/pranvgarg/toolsniff/profile"
	"github.com/pranvgarg/toolsniff/registry"
)

func TestProfileRoundTripPreservesObservationsAndReport(t *testing.T) {
	seen := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	first := profileObservation("zeta", "1.0.0", "/opt/zeta")
	second := profileObservation("alpha", "2.0.0", "/opt/alpha")
	report := output.NewObservationReport([]model.Observation{first}, []model.Observation{second}, nil, registry.ObservationDiff{}, []string{"warning"})
	value := profile.New([]model.Observation{first, second}, &report)
	value.CreatedAt = seen
	value.Version = "0.5.0"

	data, err := profile.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	got, err := profile.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(got, value) {
		t.Fatalf("profile round trip mismatch: got %+v, want %+v", got, value)
	}
	if !strings.Contains(string(data), `"report"`) || !strings.Contains(string(data), `"created_at"`) {
		t.Fatalf("profile metadata/report missing: %s", data)
	}
}

func TestProfileCorruptionIsRejected(t *testing.T) {
	for name, data := range map[string][]byte{
		"malformed":   []byte("{"),
		"trailing":    []byte(`{"schema_version":1,"created_at":"2026-08-02T12:00:00Z","observations":[]} {}`),
		"unknown":     []byte(`{"schema_version":1,"created_at":"2026-08-02T12:00:00Z","observations":[],"secret":"value"}`),
		"unsupported": []byte(`{"schema_version":99,"created_at":"2026-08-02T12:00:00Z","observations":[]}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := profile.Unmarshal(data); err == nil {
				t.Fatal("Unmarshal accepted corrupt profile")
			}
		})
	}
}

func TestCompareProfilesReportsDeterministicAddRemoveUpdate(t *testing.T) {
	removed := profileObservation("removed", "1.0.0", "/opt/removed")
	updatedBefore := profileObservation("updated", "1.0.0", "/opt/updated")
	updatedAfter := profileObservation("updated", "2.0.0", "/opt/updated")
	added := profileObservation("added", "1.0.0", "/opt/added")
	before := profile.New([]model.Observation{updatedBefore, removed}, nil)
	after := profile.New([]model.Observation{added, updatedAfter}, nil)

	first := profile.CompareProfiles(before, after)
	second := profile.CompareProfiles(before, after)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("comparison is not deterministic: %+v != %+v", first, second)
	}
	if len(first.Added) != 1 || first.Added[0].Identity != added.ID {
		t.Fatalf("unexpected additions: %+v", first.Added)
	}
	if len(first.Removed) != 1 || first.Removed[0].Identity != removed.ID {
		t.Fatalf("unexpected removals: %+v", first.Removed)
	}
	if len(first.Updated) != 1 || first.Updated[0].Identity != updatedAfter.ID || first.Updated[0].Before.Version.Value != "1.0.0" || first.Updated[0].After.Version.Value != "2.0.0" {
		t.Fatalf("unexpected updates: %+v", first.Updated)
	}
}

func TestSupportBundleRedactsHomeAndOmitsSecretsAndRawOutput(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir failed: %v", err)
	}
	path := filepath.Join(home, "private", "tool")
	observation := profileObservation("safe-tool", "1.0.0", path)
	observation.Availability.Probe = &model.ProbeResult{Error: "command output token=top-secret password=hunter2"}
	observation.Evidence = []model.Evidence{{Type: "probe", Source: "command", Description: "raw command output: top-secret"}}
	report := output.NewObservationReport([]model.Observation{observation}, nil, nil, registry.ObservationDiff{}, []string{"raw warning token=top-secret"})
	value := profile.New([]model.Observation{observation}, &report)
	data, err := profile.CreateSupportBundle(value)
	if err != nil {
		t.Fatalf("CreateSupportBundle failed: %v", err)
	}
	text := string(data)
	for _, forbidden := range []string{home, "top-secret", "hunter2", "raw command output", "token=", "password="} {
		if strings.Contains(text, forbidden) {
			t.Errorf("support bundle contains forbidden %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, profile.HomeRedaction) {
		t.Fatalf("support bundle did not include redacted path: %s", text)
	}
	var decoded profile.SupportBundle
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("support bundle is invalid JSON: %v", err)
	}
	if decoded.Report == nil || len(decoded.Report.Warnings) != 0 || decoded.Observations[0].Availability.Probe != nil || len(decoded.Observations[0].Evidence) != 0 {
		t.Fatalf("support bundle retained unsafe report data: %+v", decoded)
	}
}

func TestWriteSupportBundleFailureDoesNotReplaceDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bundle")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("creating target directory: %v", err)
	}
	if err := profile.WriteSupportBundle(path, profile.New(nil, nil)); err == nil {
		t.Fatal("WriteSupportBundle unexpectedly replaced a directory")
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		t.Fatalf("failed write changed target directory: info=%v err=%v", info, err)
	}
}

func profileObservation(name, version, path string) model.Observation {
	observation := model.Observation{
		DisplayName: name,
		CommandName: name,
		Kind:        model.KindPackage,
		Role:        model.RoleInstalled,
		Origin:      model.Origin{Provider: "npm", Manager: "global", Package: name},
		Version:     model.VersionInfo{Value: version, State: model.VersionKnown, Scheme: model.SchemeSemver, Comparable: true, Confidence: model.ConfidenceHigh},
		Locations:   []model.Location{{Path: path, Type: model.LocationPackagePrefix}},
		Availability: model.AvailabilityInfo{
			State: model.AvailabilityAvailable,
		},
	}
	observation.ID = model.ObservationIdentity(observation)
	return observation
}
