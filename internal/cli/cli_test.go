package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/pranvgarg/toolsniff/config"
	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/profile"
	"github.com/pranvgarg/toolsniff/registry"
	"github.com/pranvgarg/toolsniff/scanner"
)

func TestRunVersionUsesProvidedOutput(t *testing.T) {
	var output, errorOutput bytes.Buffer

	if code := Run([]string{"--version"}, strings.NewReader(""), &output, &errorOutput); code != 0 {
		t.Fatalf("Run returned %d, want 0", code)
	}
	if got, want := output.String(), "dev\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	if errorOutput.Len() != 0 {
		t.Fatalf("unexpected error output: %q", errorOutput.String())
	}
}

func TestRunRejectsMultipleReportModes(t *testing.T) {
	var output, errorOutput bytes.Buffer

	if code := Run([]string{"--list", "--json"}, strings.NewReader(""), &output, &errorOutput); code != 2 {
		t.Fatalf("Run returned %d, want 2", code)
	}
	if got, want := errorOutput.String(), "only one of --list, --json, --save, --diff, or --update may be used\n"; got != want {
		t.Fatalf("error output = %q, want %q", got, want)
	}
}

func TestRunRejectsMultipleWaveFiveModesBeforeLoadingConfig(t *testing.T) {
	var output, errorOutput bytes.Buffer

	if code := Run([]string{"--doctor", "--snapshot", "--config", filepath.Join(t.TempDir(), "invalid.toml")}, strings.NewReader(""), &output, &errorOutput); code != 2 {
		t.Fatalf("Run returned %d, want 2", code)
	}
	if !strings.Contains(errorOutput.String(), "only one") {
		t.Fatalf("error output = %q, want mode conflict", errorOutput.String())
	}
}

func TestRunCapabilitiesUsesCurrentReportWithoutChangingJSONReport(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatalf("creating fake bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "display-only"), []byte("#!/bin/sh\nprintf '%s\\n' version-1\n"), 0o700); err != nil {
		t.Fatalf("writing fake command: %v", err)
	}
	configPath := writeCLIConfig(t, root, binDir, filepath.Join(root, "registry.json"))
	t.Setenv("PATH", binDir)

	var output, errorOutput bytes.Buffer
	if code := Run([]string{"--config", configPath, "--capabilities"}, strings.NewReader(""), &output, &errorOutput); code != 0 {
		t.Fatalf("Run returned %d: %s", code, errorOutput.String())
	}
	var result struct {
		Capabilities []struct {
			ObservationID string `json:"observation_id"`
			Capability    struct {
				Kind     string                     `json:"kind"`
				Evidence []model.CapabilityEvidence `json:"evidence"`
			} `json:"capability"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decoding capabilities output: %v\n%s", err, output.String())
	}
	if len(result.Capabilities) != 1 || result.Capabilities[0].Capability.Kind != string(model.CapabilityVersionProbe) {
		t.Fatalf("capabilities = %+v, want one version-probe result", result.Capabilities)
	}
	if len(result.Capabilities[0].Capability.Evidence) != 1 {
		t.Fatalf("default capabilities mode ran a probe: %+v", result.Capabilities[0])
	}
}

func TestValidateModeRejectsCapabilityProbeWithoutCapabilities(t *testing.T) {
	var output, errorOutput bytes.Buffer
	if code := Run([]string{"--capabilities-probe", "--config", filepath.Join(t.TempDir(), "invalid.toml")}, strings.NewReader(""), &output, &errorOutput); code != 2 {
		t.Fatalf("Run returned %d, want 2", code)
	}
	if got := errorOutput.String(); got != "--capabilities-probe may only be used with --capabilities\n" {
		t.Fatalf("error output = %q", got)
	}
}

func TestSnapshotsListsWithoutScanningOrLoadingConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var output, errorOutput bytes.Buffer
	if code := Run([]string{"--snapshots", "--config", filepath.Join(home, "invalid.toml")}, strings.NewReader(""), &output, &errorOutput); code != 0 {
		t.Fatalf("Run returned %d: %s", code, errorOutput.String())
	}
	if got, want := output.String(), "no snapshots\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}

	path := filepath.Join(home, ".toolsniff", "snapshots", "snapshot-fixture.json")
	if err := registry.SaveSnapshot(path, registry.NewSnapshot([]model.Observation{cliFixtureObservation("fixture", model.RoleInstalled)}, "test")); err != nil {
		t.Fatalf("saving fixture snapshot: %v", err)
	}
	output.Reset()
	if code := Run([]string{"--snapshots", "--config", filepath.Join(home, "still-invalid.toml")}, strings.NewReader(""), &output, &errorOutput); code != 0 {
		t.Fatalf("Run returned %d: %s", code, errorOutput.String())
	}
	if !strings.Contains(output.String(), "snapshot-fixture.json") {
		t.Fatalf("snapshot listing = %q", output.String())
	}
}

func TestSnapshotSavesOnlyNonHistoryObservations(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	binDir := filepath.Join(root, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatalf("creating bin directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "fixture-command"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("writing fixture command: %v", err)
	}
	configPath := writeCLIConfig(t, root, binDir, filepath.Join(root, "registry.json"))
	t.Setenv("PATH", binDir)

	var output, errorOutput bytes.Buffer
	if code := Run([]string{"--config", configPath, "--snapshot"}, strings.NewReader(""), &output, &errorOutput); code != 0 {
		t.Fatalf("Run returned %d: %s", code, errorOutput.String())
	}
	snapshots, err := registry.ListSnapshots(filepath.Join(root, ".toolsniff", "snapshots"))
	if err != nil || len(snapshots) != 1 {
		t.Fatalf("snapshots = %+v, err=%v", snapshots, err)
	}
	snapshot, err := registry.LoadSnapshot(snapshots[0].Path)
	if err != nil {
		t.Fatalf("loading snapshot: %v", err)
	}
	for _, observation := range snapshot.Observations {
		if observation.Role == model.RoleHistory || observation.Kind == model.KindHistory {
			t.Fatalf("history observation was saved: %+v", observation)
		}
	}
	if !strings.Contains(output.String(), "saved snapshot:") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestDoctorRendersTypedDiagnosticsAndProvenance(t *testing.T) {
	output := renderDoctorReport([]model.Observation{cliDiagnosticObservation()})
	for _, expected := range []string{
		"[shadowed-command]",
		"[broken-location]",
		"[unsigned-application]",
		"[missing-executable-link]",
		"PROVENANCE: 1",
		"package-name -> /opt/tool",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("doctor output missing %q:\n%s", expected, output)
		}
	}
}

func TestProfileModesUseFixtureObservationsAndSanitizeFiles(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	binDir := filepath.Join(root, "bin")
	commandDir := filepath.Join(root, "commands")
	for _, dir := range []string{binDir, commandDir} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatalf("creating fixture directory: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(binDir, "fixture-command"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("writing fixture command: %v", err)
	}
	npmPath := filepath.Join(commandDir, "npm")
	writeNPMFixture(t, npmPath, root, "1.0.0")
	configPath := writeCLIConfig(t, root, binDir, filepath.Join(root, "registry.json"))
	t.Setenv("PATH", commandDir)

	var output, errorOutput bytes.Buffer
	if code := Run([]string{"--config", configPath, "--doctor"}, strings.NewReader(""), &output, &errorOutput); code != 0 {
		t.Fatalf("doctor Run returned %d: %s", code, errorOutput.String())
	}
	if !strings.Contains(output.String(), "TOOLSNIFF DOCTOR") || !strings.Contains(output.String(), "fixture-package ->") {
		t.Fatalf("doctor output = %q", output.String())
	}
	for _, path := range []string{filepath.Join(root, "registry.json"), registry.AvailabilityPath(filepath.Join(root, "registry.json"))} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("doctor mutated %s: err=%v", path, err)
		}
	}

	exportPath := filepath.Join(root, "exports", "profile.json")
	output.Reset()
	errorOutput.Reset()
	if code := Run([]string{"--config", configPath, "--export-profile", exportPath}, strings.NewReader(""), &output, &errorOutput); code != 0 {
		t.Fatalf("export Run returned %d: %s", code, errorOutput.String())
	}
	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("reading exported profile: %v", err)
	}
	if strings.Contains(string(data), root) {
		t.Fatalf("export contains the home path: %s", data)
	}
	exported, err := profile.Unmarshal(data)
	if err != nil || len(exported.Observations) == 0 {
		t.Fatalf("exported profile = %+v, err=%v", exported, err)
	}

	bundlePath := filepath.Join(root, "bundles", "support.json")
	output.Reset()
	if code := Run([]string{"--config", configPath, "--support-bundle", bundlePath}, strings.NewReader(""), &output, &errorOutput); code != 0 {
		t.Fatalf("support bundle Run returned %d: %s", code, errorOutput.String())
	}
	bundleData, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatalf("reading support bundle: %v", err)
	}
	if strings.Contains(string(bundleData), root) {
		t.Fatalf("support bundle contains the home path: %s", bundleData)
	}

	writeNPMFixture(t, npmPath, root, "2.0.0")
	output.Reset()
	if code := Run([]string{"--config", configPath, "--compare-profile", exportPath}, strings.NewReader(""), &output, &errorOutput); code != 0 {
		t.Fatalf("compare Run returned %d: %s", code, errorOutput.String())
	}
	if !strings.Contains(output.String(), "UPDATED") || !strings.Contains(output.String(), "1.0.0 -> 2.0.0") {
		t.Fatalf("compare output = %q", output.String())
	}
}

func writeNPMFixture(t *testing.T, path, root, version string) {
	t.Helper()
	content := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' '{\"path\":\"%s/npm-global\",\"prefix\":\"%s/npm-global\",\"dependencies\":{\"fixture-package\":{\"version\":\"%s\",\"bin\":{\"fixture-package\":\"%s/npm-global/bin/fixture-package\"}}}}'\n", root, root, version, root)
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("writing npm fixture: %v", err)
	}
}

func cliFixtureObservation(name string, role model.SourceRole) model.Observation {
	observation := model.Observation{
		DisplayName:  name,
		CommandName:  name,
		Kind:         model.KindPackage,
		Role:         role,
		Origin:       model.Origin{Provider: "fixture", Package: name},
		Version:      model.VersionInfo{State: model.VersionNotReported, Confidence: model.ConfidenceLow},
		Availability: model.AvailabilityInfo{State: model.AvailabilityUnknown},
	}
	observation.ID = model.ObservationIdentity(observation)
	return observation
}

func cliDiagnosticObservation() model.Observation {
	signed := false
	observation := cliFixtureObservation("diagnostic-tool", model.RoleInstalled)
	observation.Locations = []model.Location{{Path: "/opt/tool", Type: model.LocationPackagePrefix}}
	observation.Availability = model.AvailabilityInfo{State: model.AvailabilityUnavailable, ShadowedBy: []string{"/usr/local/bin/tool"}}
	observation.Package = &model.PackageInfo{Name: "package-name", Executables: []string{"tool"}}
	observation.Application = &model.ApplicationInfo{Signed: &signed}
	observation.Version.State = model.VersionUnknown
	return observation
}

func TestSplitByRole(t *testing.T) {
	registrations := []scanner.Registration{
		{SourceInfo: scanner.SourceInfo{ID: model.SourceBrewFormula, Role: model.RoleInstalled}},
		{SourceInfo: scanner.SourceInfo{ID: model.SourcePath, Role: model.RoleAvailable}},
		{SourceInfo: scanner.SourceInfo{ID: model.SourceNPXHistory, Role: model.RoleHistory, Informational: true}},
	}
	tests := []struct {
		name           string
		input          []model.Tool
		wantInstalled  []model.Tool
		wantAvailable  []model.Tool
		wantNPXHistory []model.Tool
	}{
		{
			name: "mix of npx-history and other sources",
			input: []model.Tool{
				{Name: "gh", Source: model.SourceBrewFormula},
				{Name: "create-react-app", Source: model.SourceNPXHistory},
				{Name: "wget", Source: model.SourcePath},
				{Name: "cowsay", Source: model.SourceNPXHistory},
			},
			wantInstalled: []model.Tool{{Name: "gh", Source: model.SourceBrewFormula}},
			wantAvailable: []model.Tool{{Name: "wget", Source: model.SourcePath}},
			wantNPXHistory: []model.Tool{
				{Name: "create-react-app", Source: model.SourceNPXHistory},
				{Name: "cowsay", Source: model.SourceNPXHistory},
			},
		},
		{
			name: "all real",
			input: []model.Tool{
				{Name: "gh", Source: "brew-formula"},
				{Name: "wget", Source: "path"},
			},
			wantInstalled: []model.Tool{{Name: "gh", Source: "brew-formula"}},
			wantAvailable: []model.Tool{{Name: "wget", Source: "path"}},
		},
		{
			name: "all npx-history",
			input: []model.Tool{
				{Name: "create-react-app", Source: model.SourceNPXHistory},
				{Name: "cowsay", Source: model.SourceNPXHistory},
			},
			wantNPXHistory: []model.Tool{
				{Name: "create-react-app", Source: model.SourceNPXHistory},
				{Name: "cowsay", Source: model.SourceNPXHistory},
			},
		},
		{
			name: "empty input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotInstalled, gotAvailable, gotNPXHistory := splitByRole(tt.input, registrations)

			if !toolsEqual(gotInstalled, tt.wantInstalled) {
				t.Errorf("installed = %+v, want %+v", gotInstalled, tt.wantInstalled)
			}
			if !toolsEqual(gotAvailable, tt.wantAvailable) {
				t.Errorf("available = %+v, want %+v", gotAvailable, tt.wantAvailable)
			}
			if !toolsEqual(gotNPXHistory, tt.wantNPXHistory) {
				t.Errorf("npxHistory = %+v, want %+v", gotNPXHistory, tt.wantNPXHistory)
			}
		})
	}
}

func TestValidateFlagsRequiresDiffForAvailability(t *testing.T) {
	if err := validateFlags(true, false, false, false, false); err == nil {
		t.Fatal("expected --available without --diff to be rejected")
	}
	if err := validateFlags(true, true, false, false, false); err != nil {
		t.Fatalf("expected --available with --diff to be accepted: %v", err)
	}
	if err := validateFlags(false, false, true, false, false); err != nil {
		t.Fatalf("expected --update to be accepted: %v", err)
	}
	if err := validateFlags(false, false, false, false, true); err == nil {
		t.Fatal("expected --yes without --update or --init-config to be rejected")
	}
	if err := validateFlags(false, false, false, false, false); err != nil {
		t.Fatalf("expected ordinary mode to be accepted: %v", err)
	}
}

func TestRunObservationsPrefersAdaptersAndAnnotatesRegistrationRoles(t *testing.T) {
	adapterObservation := model.Observation{
		DisplayName: "adapter-tool",
		CommandName: "adapter-tool",
		Kind:        model.KindExecutable,
		Role:        model.RoleInstalled,
		Origin:      model.Origin{Provider: "fixture"},
		Version: model.VersionInfo{
			State:      model.VersionNotReported,
			Confidence: model.ConfidenceLow,
		},
		Availability: model.AvailabilityInfo{State: model.AvailabilityUnknown},
	}
	legacyTool := model.Tool{Name: "history-tool", Source: model.SourceNPXHistory, Role: model.RoleInstalled}
	observations, warnings := scanner.RunObservations([]scanner.Registration{
		{SourceInfo: scanner.SourceInfo{ID: "adapter", Role: model.RoleAvailable}, Scanner: fixtureScanner{observation: adapterObservation}},
		{SourceInfo: scanner.SourceInfo{ID: "legacy", Role: model.RoleHistory}, Scanner: legacyFixtureScanner{tool: legacyTool}},
	})
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %+v", warnings)
	}
	if len(observations) != 2 {
		t.Fatalf("observations = %+v, want 2 observations", observations)
	}
	for _, observation := range observations {
		switch observation.DisplayName {
		case "adapter-tool":
			if observation.Role != model.RoleAvailable {
				t.Errorf("adapter role = %q, want available", observation.Role)
			}
		case "history-tool":
			if observation.Role != model.RoleHistory || observation.Kind != model.KindHistory {
				t.Errorf("legacy role/kind = %q/%q, want history/history", observation.Role, observation.Kind)
			}
		default:
			t.Errorf("unexpected observation: %+v", observation)
		}
	}
}

func TestRunJSONUsesV2SchemaAndMigratesV1Baseline(t *testing.T) {
	root := t.TempDir()
	pathDir := filepath.Join(root, "path")
	commandDir := filepath.Join(root, "commands")
	for _, dir := range []string{pathDir, commandDir} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatalf("creating fake directory: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(commandDir, "npm"), []byte("#!/bin/sh\nprintf '%s\\n' '{\"dependencies\":{\"fixture-package\":{\"version\":\"1.0.0\",\"bin\":{\"fixture-package\":\"/opt/bin/fixture-package\"}}}}'\n"), 0o700); err != nil {
		t.Fatalf("writing fake npm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pathDir, "fixture-command"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("writing fake PATH command: %v", err)
	}
	registryPath := filepath.Join(root, "registry.json")
	legacy, err := json.Marshal([]model.Tool{{Name: "fixture-package", Source: model.SourceNPM, Version: "1.0.0", Role: model.RoleInstalled}})
	if err != nil {
		t.Fatalf("marshaling v1 baseline: %v", err)
	}
	if err := os.WriteFile(registryPath, legacy, 0o600); err != nil {
		t.Fatalf("writing v1 baseline: %v", err)
	}
	configPath := writeCLIConfig(t, root, pathDir, registryPath)
	t.Setenv("PATH", commandDir)

	var output, errorOutput bytes.Buffer
	if code := Run([]string{"--config", configPath, "--json"}, strings.NewReader(""), &output, &errorOutput); code != 0 {
		t.Fatalf("Run returned %d: %s", code, errorOutput.String())
	}
	var report struct {
		SchemaVersion int                 `json:"schema_version"`
		Installed     []model.Observation `json:"installed"`
		Available     []model.Observation `json:"available"`
		History       []model.Observation `json:"history"`
		Changes       struct {
			Added   []registry.ChangeEvent `json:"added"`
			Removed []registry.ChangeEvent `json:"removed"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decoding v2 report: %v\n%s", err, output.String())
	}
	if report.SchemaVersion != 2 {
		t.Fatalf("schema version = %d, want 2", report.SchemaVersion)
	}
	if len(report.Installed) != 1 || report.Installed[0].Origin.Provider != "npm" {
		t.Fatalf("installed observations = %+v", report.Installed)
	}
	if len(report.Available) == 0 {
		t.Fatal("expected fake PATH observations in available bucket")
	}
	for _, event := range append(report.Changes.Added, report.Changes.Removed...) {
		if strings.Contains(event.Identity, "fixture-package") {
			t.Fatalf("v1 baseline produced a package add/remove change: %+v", event)
		}
	}
}

func TestRunSaveWritesV2InstalledAndAvailabilityRegistries(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatalf("creating fake bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "fixture-command"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("writing fake command: %v", err)
	}
	registryPath := filepath.Join(root, "registry.json")
	configPath := writeCLIConfig(t, root, binDir, registryPath)
	t.Setenv("PATH", binDir)

	var output, errorOutput bytes.Buffer
	if code := Run([]string{"--config", configPath, "--save"}, strings.NewReader(""), &output, &errorOutput); code != 0 {
		t.Fatalf("Run returned %d: %s", code, errorOutput.String())
	}
	for _, path := range []string{registryPath, registry.AvailabilityPath(registryPath)} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		var envelope registry.Envelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			t.Fatalf("decoding %s: %v", path, err)
		}
		if envelope.SchemaVersion != registry.CurrentSchemaVersion {
			t.Errorf("%s schema version = %d, want %d", path, envelope.SchemaVersion, registry.CurrentSchemaVersion)
		}
	}
}

func writeCLIConfig(t *testing.T, root, binDir, registryPath string) string {
	t.Helper()
	configPath := filepath.Join(root, "config.toml")
	config := strings.Join([]string{
		"[applications]",
		"roots = [" + strconv.Quote(filepath.Join(root, "applications")) + "]",
		"",
		"[path]",
		"directories = [" + strconv.Quote(binDir) + "]",
		"",
		"[npx]",
		"dir = " + strconv.Quote(filepath.Join(root, "npx")),
		"",
		"[cargo]",
		"bin_dir = " + strconv.Quote(filepath.Join(root, "cargo")),
		"",
		"[bun]",
		"enabled = false",
		"",
		"[registry]",
		"path = " + strconv.Quote(registryPath),
		"",
	}, "\n")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	return configPath
}

type fixtureScanner struct {
	observation model.Observation
}

func (s fixtureScanner) Name() string { return "fixture" }

func (s fixtureScanner) Scan() ([]model.Tool, error) {
	return nil, nil
}

func (s fixtureScanner) ScanObservations() ([]model.Observation, error) {
	if s.observation.DisplayName == "" {
		return nil, nil
	}
	return []model.Observation{s.observation}, nil
}

type legacyFixtureScanner struct {
	tool model.Tool
}

func (s legacyFixtureScanner) Name() string { return "legacy-fixture" }

func (s legacyFixtureScanner) Scan() ([]model.Tool, error) {
	return []model.Tool{s.tool}, nil
}

// The --legacy-tabs flag is the rollback switch for the opt-in intent-tabs
// rollout: it pins the TUI to the eight-tab layout even when the config asks
// for intent.
func TestLegacyTabsFlagForcesLegacy(t *testing.T) {
	settings := config.Settings{UI: config.UISettings{Mode: "intent"}}
	if mode := resolveUIMode(settings, true, false); mode != "legacy" {
		t.Fatalf("resolveUIMode(intent, legacyTabs=true) = %q, want legacy", mode)
	}
	if mode := resolveUIMode(settings, false, false); mode != "intent" {
		t.Fatalf("resolveUIMode(intent, legacyTabs=false) = %q, want intent", mode)
	}
}

// --intent-tabs is the inverse of --legacy-tabs: it pins the TUI to the
// intent-based four-tab layout even though legacy is the shipped default.
func TestIntentTabsFlagForcesIntent(t *testing.T) {
	settings := config.Settings{UI: config.UISettings{Mode: "legacy"}}
	if mode := resolveUIMode(settings, false, true); mode != "intent" {
		t.Fatalf("resolveUIMode(legacy, intentTabs=true) = %q, want intent", mode)
	}
	if mode := resolveUIMode(settings, false, false); mode != "legacy" {
		t.Fatalf("resolveUIMode(legacy, no flags) = %q, want legacy", mode)
	}
}

// --legacy-tabs is the rollback switch, so it wins when both flags are passed.
func TestResolveUIModeLegacyTabsWinsOverIntent(t *testing.T) {
	for _, mode := range []string{"legacy", "intent"} {
		settings := config.Settings{UI: config.UISettings{Mode: mode}}
		if got := resolveUIMode(settings, true, true); got != "legacy" {
			t.Fatalf("resolveUIMode(%s, legacyTabs=true, intentTabs=true) = %q, want legacy", mode, got)
		}
	}
}

func TestResolveUIModeKeepsConfiguredLegacyDefault(t *testing.T) {
	settings := config.Settings{UI: config.UISettings{Mode: "legacy"}}
	if mode := resolveUIMode(settings, false, false); mode != "legacy" {
		t.Fatalf("resolveUIMode(legacy, legacyTabs=false) = %q, want legacy", mode)
	}
	if mode := resolveUIMode(settings, true, false); mode != "legacy" {
		t.Fatalf("resolveUIMode(legacy, legacyTabs=true) = %q, want legacy", mode)
	}
}

func TestParseFlagsLegacyTabs(t *testing.T) {
	var errorOutput bytes.Buffer
	options, err := parseFlags([]string{"--legacy-tabs"}, &errorOutput)
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if !options.legacyTabs {
		t.Fatal("--legacy-tabs did not set options.legacyTabs")
	}

	options, err = parseFlags(nil, &errorOutput)
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if options.legacyTabs {
		t.Fatal("options.legacyTabs set without --legacy-tabs")
	}
}

func TestParseFlagsIntentTabs(t *testing.T) {
	var errorOutput bytes.Buffer
	options, err := parseFlags([]string{"--intent-tabs"}, &errorOutput)
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if !options.intentTabs {
		t.Fatal("--intent-tabs did not set options.intentTabs")
	}

	options, err = parseFlags(nil, &errorOutput)
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if options.intentTabs {
		t.Fatal("options.intentTabs set without --intent-tabs")
	}
}

// Both UI-mode overrides have to be discoverable from --help; a rollback switch
// nobody can find is not a rollback switch.
func TestParseFlagsHelpListsUIModeFlags(t *testing.T) {
	var errorOutput bytes.Buffer
	if _, err := parseFlags([]string{"--help"}, &errorOutput); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("parseFlags(--help) error = %v, want flag.ErrHelp", err)
	}
	usage := errorOutput.String()
	for _, name := range []string{"-legacy-tabs", "-intent-tabs"} {
		if !strings.Contains(usage, name) {
			t.Fatalf("--help output missing %s:\n%s", name, usage)
		}
	}
}

func TestInitConfigWritesLoadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	var out, errOut bytes.Buffer
	code := Run([]string{"--init-config", "--config", path}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("Run(--init-config) = %d, stderr: %s", code, errOut.String())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	if !strings.Contains(out.String(), path) {
		t.Fatalf("output missing config path: %s", out.String())
	}
}

func TestInitConfigRefusesToOverwriteWithoutYes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	var out, errOut bytes.Buffer
	if code := Run([]string{"--init-config", "--config", path}, nil, &out, &errOut); code != 0 {
		t.Fatalf("first --init-config failed: %s", errOut.String())
	}
	errOut.Reset()
	if code := Run([]string{"--init-config", "--config", path}, nil, &out, &errOut); code == 0 {
		t.Fatal("second --init-config without --yes should have failed")
	}
}

func TestDiffWithNoBaselineSaysSoInsteadOfListingEverythingAsNew(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.toml")
	regPath := filepath.Join(tmp, "registry.json")
	if err := os.WriteFile(configPath, []byte("[registry]\npath = \""+regPath+"\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var out, errOut bytes.Buffer
	code := Run([]string{"--diff", "--config", configPath}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("Run(--diff, no baseline) = %d, stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "no baseline yet") {
		t.Fatalf("--diff with no baseline should say so, got: %s", out.String())
	}
	if strings.Contains(out.String(), "ADDED") {
		t.Fatalf("--diff with no baseline should not print a change report, got: %s", out.String())
	}
}

func toolsEqual(a, b []model.Tool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
