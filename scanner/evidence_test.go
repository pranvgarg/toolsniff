package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pranvgarg/toolsniff/model"
)

func TestPathEvidenceTracksShadowingAndPATHOrder(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	third := t.TempDir()
	writeExecutable(t, filepath.Join(first, "tool"), 0o755)
	writeExecutable(t, filepath.Join(second, "tool"), 0o755)
	if err := os.Symlink(filepath.Join(third, "missing"), filepath.Join(third, "tool")); err != nil {
		t.Fatalf("create broken symlink: %v", err)
	}

	evidence, err := NewPathScanner([]string{first, second, third}, nil, nil).ScanEvidence()
	if err != nil {
		t.Fatalf("ScanEvidence() error: %v", err)
	}
	if len(evidence) != 1 {
		t.Fatalf("expected one command, got %+v", evidence)
	}
	tool := evidence[0]
	if tool.ActivePath != filepath.Join(first, "tool") || tool.ActivePATHIndex != 0 {
		t.Fatalf("unexpected active path: %+v", tool)
	}
	if len(tool.Candidates) != 3 || tool.Candidates[0].PATHIndex != 0 || tool.Candidates[1].PATHIndex != 1 || tool.Candidates[2].PATHIndex != 2 {
		t.Fatalf("candidates lost PATH order: %+v", tool.Candidates)
	}
	if !tool.Candidates[0].Active || tool.Candidates[1].Active || !tool.Candidates[2].Broken {
		t.Fatalf("unexpected candidate states: %+v", tool.Candidates)
	}
	if len(tool.ShadowedBy) != 1 || tool.ShadowedBy[0] != filepath.Join(second, "tool") {
		t.Fatalf("unexpected shadowing: %+v", tool.ShadowedBy)
	}

	observation := tool.Observation(nil)
	if err := observation.Validate(); err != nil {
		t.Fatalf("observation is invalid: %v", err)
	}
	if observation.Availability.State != model.AvailabilityAvailable || observation.Availability.PATHIndex != 0 {
		t.Fatalf("unexpected availability: %+v", observation.Availability)
	}
	if len(observation.Locations) != 3 || !observation.Locations[2].IsSymlink || observation.Locations[2].SymlinkTarget == "" {
		t.Fatalf("symlink metadata was not preserved: %+v", observation.Locations)
	}
}

func TestScanExecutableDirEvidencePreservesSymlinkAndDetectsBrokenTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "tool")
	writeExecutable(t, target, 0o755)
	if err := os.Symlink("target", link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	candidates, err := ScanExecutableDirEvidence(dir, model.SourcePath)
	if err != nil {
		t.Fatalf("ScanExecutableDirEvidence() error: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected target and link, got %+v", candidates)
	}
	var linkCandidate ExecutableCandidate
	for _, candidate := range candidates {
		if candidate.Tool.Name == "tool" {
			linkCandidate = candidate
		}
	}
	if linkCandidate.Broken || !linkCandidate.Location.IsSymlink || linkCandidate.Location.SymlinkTarget != "target" || !linkCandidate.Location.Executable {
		t.Fatalf("unexpected symlink candidate: %+v", linkCandidate)
	}
	if linkCandidate.Location.ModifiedAt == nil {
		t.Fatal("expected symlink modification time")
	}

	broken := filepath.Join(dir, "broken")
	if err := os.Symlink("does-not-exist", broken); err != nil {
		t.Fatalf("create broken symlink: %v", err)
	}
	candidates, err = ScanExecutableDirEvidence(dir, model.SourcePath)
	if err != nil {
		t.Fatalf("second ScanExecutableDirEvidence() error: %v", err)
	}
	for _, candidate := range candidates {
		if candidate.Tool.Name == "broken" {
			if !candidate.Broken || !candidate.Location.IsSymlink || candidate.Location.SymlinkTarget != "does-not-exist" {
				t.Fatalf("unexpected broken candidate: %+v", candidate)
			}
			return
		}
	}
	t.Fatal("broken symlink was not retained")
}

func TestScanExecutableDirEvidenceExcludesNonExecutableFiles(t *testing.T) {
	dir := t.TempDir()
	writeExecutable(t, filepath.Join(dir, "executable"), 0o755)
	writeExecutable(t, filepath.Join(dir, "private"), 0o644)

	candidates, err := ScanExecutableDirEvidence(dir, model.SourcePath)
	if err != nil {
		t.Fatalf("ScanExecutableDirEvidence() error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Tool.Name != "executable" {
		t.Fatalf("permission filtering failed: %+v", candidates)
	}
}

func TestPathScannerObservationsProbeOnlyActiveCandidate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tool")
	if err := os.Symlink(os.Args[0], path); err != nil {
		t.Fatalf("create test executable symlink: %v", err)
	}
	t.Setenv("TOOL_SNIFF_PROBE_HELPER", "success")

	observations, err := NewPathScanner([]string{dir}, nil, nil).ScanObservations(PathScanOptions{
		Probe: &ProbeOptions{
			Enabled:        true,
			Arguments:      []string{"version", "observed"},
			Timeout:        5 * time.Second,
			MaxOutputBytes: 128,
		},
	})
	if err != nil {
		t.Fatalf("ScanObservations() error: %v", err)
	}
	if len(observations) != 1 {
		t.Fatalf("expected one observation, got %+v", observations)
	}
	observation := observations[0]
	if observation.Availability.Probe == nil || observation.Availability.Probe.Version != "observed" || observation.Version.Value != "observed" {
		t.Fatalf("probe result was not attached: %+v", observation)
	}
}

func writeExecutable(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte("placeholder\n"), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
