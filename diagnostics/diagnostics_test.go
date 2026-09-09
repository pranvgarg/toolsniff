package diagnostics

import (
	"reflect"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

func TestAnalyzeFindsShadowedCommand(t *testing.T) {
	observation := observation("shadowed")
	observation.CommandName = "node"
	observation.Availability.ShadowedBy = []string{"/usr/local/bin/node", "/opt/bin/node", "/usr/local/bin/node"}

	report := Analyze([]model.Observation{observation})
	issue := onlyIssue(t, report, IssueShadowedCommand)
	if !reflect.DeepEqual(issue.ShadowedPaths, []string{"/opt/bin/node", "/usr/local/bin/node"}) {
		t.Fatalf("unexpected shadowed paths: %+v", issue.ShadowedPaths)
	}
}

func TestAnalyzeFindsBrokenLocationOnlyWhenUnavailable(t *testing.T) {
	broken := observation("broken")
	broken.Availability = model.AvailabilityInfo{
		State:        model.AvailabilityUnavailable,
		ResolvedPath: "/bin/missing",
	}
	healthy := observation("healthy")
	healthy.Availability.State = model.AvailabilityUnknown

	report := Analyze([]model.Observation{healthy, broken})
	issue := onlyIssue(t, report, IssueBrokenLocation)
	if issue.LocationPath != "/bin/missing" {
		t.Fatalf("unexpected broken location: %+v", issue)
	}
	if countIssues(report, IssueBrokenLocation) != 1 {
		t.Fatalf("expected one broken-location issue, got %+v", report.Issues)
	}
}

func TestAnalyzeFindsArchitectureMismatchOnlyWithDisjointMetadata(t *testing.T) {
	matching := observation("matching")
	matching.Application = &model.ApplicationInfo{Architectures: []string{"arm64"}}
	matching.Locations = []model.Location{{Path: "/Applications/matching.app", Type: model.LocationApplication, Architectures: []string{"aarch64"}}}

	mismatched := observation("mismatched")
	mismatched.Application = &model.ApplicationInfo{Architectures: []string{"arm64"}}
	mismatched.Locations = []model.Location{{Path: "/Applications/mismatched.app", Type: model.LocationApplication, Architectures: []string{"x86_64"}}}

	missingMetadata := observation("missing-metadata")
	missingMetadata.Application = &model.ApplicationInfo{Architectures: []string{"arm64"}}
	missingMetadata.Locations = []model.Location{{Path: "/Applications/missing.app", Type: model.LocationApplication}}

	report := Analyze([]model.Observation{matching, mismatched, missingMetadata})
	if countIssues(report, IssueArchitectureMismatch) != 1 {
		t.Fatalf("unexpected architecture issues: %+v", report.Issues)
	}
	issue := onlyIssue(t, report, IssueArchitectureMismatch)
	if issue.LocationPath != "/Applications/mismatched.app" {
		t.Fatalf("unexpected mismatch location: %+v", issue)
	}
}

func TestAnalyzeFindsSigningIssuesOnlyWhenApplicationMetadataExists(t *testing.T) {
	unsigned := observation("unsigned")
	signed := false
	unsigned.Application = &model.ApplicationInfo{Signed: &signed}
	unknown := observation("unknown-signing")
	unknown.Application = &model.ApplicationInfo{}
	known := observation("signed")
	knownSigned := true
	known.Application = &model.ApplicationInfo{Signed: &knownSigned}
	noApplication := observation("no-application")

	report := Analyze([]model.Observation{unsigned, unknown, known, noApplication})
	if countIssues(report, IssueUnsignedApplication) != 1 || countIssues(report, IssueUnknownSigningStatus) != 1 {
		t.Fatalf("unexpected signing issues: %+v", report.Issues)
	}
}

func TestAnalyzeFindsMissingExecutableLinks(t *testing.T) {
	linked := observation("linked")
	linked.Package = &model.PackageInfo{Name: "linked-package", Executables: []string{"linked"}}
	linked.Locations = []model.Location{{Path: "/bin/linked", Type: model.LocationExecutable}}

	missing := observation("missing")
	missing.Package = &model.PackageInfo{Name: "missing-package", Executables: []string{"missing", "also-missing"}}
	missing.Locations = []model.Location{{Path: "/bin/missing", Type: model.LocationPackagePrefix}}

	packageOnly := observation("package-only")
	packageOnly.Package = &model.PackageInfo{Name: "package-only"}

	report := Analyze([]model.Observation{linked, missing, packageOnly})
	if countIssues(report, IssueMissingExecutableLink) != 2 {
		t.Fatalf("unexpected missing-link issues: %+v", report.Issues)
	}
	for _, issue := range report.Issues {
		if issue.Kind == IssueMissingExecutableLink && issue.ExecutableName == "missing" {
			continue
		}
		if issue.Kind == IssueMissingExecutableLink && issue.ExecutableName == "also-missing" {
			continue
		}
		if issue.Kind == IssueMissingExecutableLink {
			t.Fatalf("unexpected missing link: %+v", issue)
		}
	}
}

func TestAnalyzeFindsUnknownVersionButNotNotReported(t *testing.T) {
	unknown := observation("unknown")
	unknown.Version = model.VersionInfo{State: model.VersionUnknown}
	notReported := observation("not-reported")
	notReported.Version = model.VersionInfo{State: model.VersionNotReported}
	known := observation("known")
	known.Version = model.VersionInfo{State: model.VersionKnown, Value: "1.0.0"}

	report := Analyze([]model.Observation{notReported, known, unknown})
	issue := onlyIssue(t, report, IssueUnknownVersion)
	if issue.ObservationID != "unknown" || issue.VersionState != model.VersionUnknown {
		t.Fatalf("unexpected version issue: %+v", issue)
	}
}

func TestAnalyzeBuildsExplicitPackageLocationEdges(t *testing.T) {
	packageObservation := observation("package")
	packageObservation.Package = &model.PackageInfo{Name: "tool-package"}
	packageObservation.Locations = []model.Location{
		{Path: "/prefix", Type: model.LocationPackagePrefix},
		{Path: "/bin/tool", Type: model.LocationExecutable},
	}

	nameOnly := observation("name-only")
	nameOnly.DisplayName = "tool-package"
	nameOnly.Locations = []model.Location{{Path: "/bin/tool", Type: model.LocationExecutable}}

	report := Analyze([]model.Observation{nameOnly, packageObservation})
	if len(report.Provenance) != 2 {
		t.Fatalf("unexpected provenance edges: %+v", report.Provenance)
	}
	if report.Provenance[0].PackageName != "tool-package" || report.Provenance[0].LocationPath != "/bin/tool" {
		t.Fatalf("provenance edges were not deterministically ordered: %+v", report.Provenance)
	}
	if report.Provenance[1].LocationPath != "/prefix" {
		t.Fatalf("unexpected second provenance edge: %+v", report.Provenance)
	}
}

func TestAnalyzeIsDeterministicAndReadOnly(t *testing.T) {
	first := observation("first")
	first.Availability.ShadowedBy = []string{"/z", "/a"}
	first.Package = &model.PackageInfo{Name: "first-package", Executables: []string{"first"}}
	first.Locations = []model.Location{{Path: "/prefix", Type: model.LocationPackagePrefix}}
	second := observation("second")
	second.Version.State = model.VersionUnknown

	input := []model.Observation{first, second}
	original := append([]model.Observation(nil), input...)
	reportOne := Analyze(input)
	reportTwo := Analyze([]model.Observation{second, first})
	if !reflect.DeepEqual(reportOne, reportTwo) {
		t.Fatalf("analysis was not deterministic:\n%+v\n%+v", reportOne, reportTwo)
	}
	if !reflect.DeepEqual(input, original) {
		t.Fatalf("Analyze mutated its input: got %+v, want %+v", input, original)
	}
}

func observation(id string) model.Observation {
	return model.Observation{
		ID:          id,
		DisplayName: id,
		CommandName: id,
		Kind:        model.KindCLI,
		Role:        model.RoleInstalled,
		Origin:      model.Origin{Provider: "test"},
		Version:     model.VersionInfo{State: model.VersionKnown, Value: "1.0.0"},
		Availability: model.AvailabilityInfo{
			State: model.AvailabilityAvailable,
		},
	}
}

func onlyIssue(t *testing.T, report Report, kind IssueKind) Issue {
	t.Helper()
	var matches []Issue
	for _, issue := range report.Issues {
		if issue.Kind == kind {
			matches = append(matches, issue)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("expected one %s issue, got %+v", kind, report.Issues)
	}
	return matches[0]
}

func countIssues(report Report, kind IssueKind) int {
	count := 0
	for _, issue := range report.Issues {
		if issue.Kind == kind {
			count++
		}
	}
	return count
}
