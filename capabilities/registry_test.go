package capabilities

import (
	"strings"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/scanner"
)

func TestMetadataAdapterRequiresExplicitPackageMetadata(t *testing.T) {
	adapter := NewMetadataAdapter("fixture", MatchSpec{
		Capability: model.CapabilityMCP,
		Packages:   []string{"fixture-mcp"},
	})

	for name, observation := range map[string]model.Observation{
		"display name only": {DisplayName: "fixture-mcp"},
		"command name only": {DisplayName: "other", CommandName: "fixture-mcp"},
		"explicit package": {
			DisplayName: "unrelated",
			Origin:      model.Origin{Provider: "npm", Package: "fixture-mcp"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, matched := adapter.Match(observation)
			if matched != (name == "explicit package") {
				t.Fatalf("matched = %v for %+v", matched, observation)
			}
		})
	}
}

func TestExplicitCapabilityEvidenceMatchesWithoutDisplayNameInference(t *testing.T) {
	adapter := NewMetadataAdapter("fixture", MatchSpec{Capability: model.CapabilityInteractiveCLI})
	observation := model.Observation{
		DisplayName: "not-the-capability-name",
		Evidence:    []model.Evidence{{Type: "capability", Source: "interactive-cli", Description: "configured"}},
	}
	evidence, matched := adapter.Match(observation)
	if !matched || evidence.Source != "interactive-cli" {
		t.Fatalf("match = %v, evidence = %+v", matched, evidence)
	}
}

func TestDefaultRegistryDoesNotInferFromDisplayNames(t *testing.T) {
	observations := make([]model.Observation, 0, 4)
	for _, name := range []string{"interactive-cli", "mcp", "gh", "gopls"} {
		observations = append(observations, model.Observation{DisplayName: name, CommandName: name})
	}
	if results := DefaultRegistry().Detect(observations); len(results) != 0 {
		t.Fatalf("display names produced capability results: %+v", results)
	}
}

func TestDefaultRegistryOrdersResultsDeterministically(t *testing.T) {
	observations := []model.Observation{
		metadataObservation("z-tool", "gopls"),
		metadataObservation("a-tool", "gh"),
		metadataObservation("m-tool", "mcp"),
	}
	first := DefaultRegistry().Detect(observations)
	second := DefaultRegistry().Detect([]model.Observation{observations[2], observations[0], observations[1]})
	if len(first) != 3 || len(second) != 3 {
		t.Fatalf("results = %d/%d, want 3", len(first), len(second))
	}
	for i := range first {
		if first[i].Capability.Kind != second[i].Capability.Kind || first[i].ObservationID != second[i].ObservationID {
			t.Fatalf("ordering changed: first=%+v second=%+v", first, second)
		}
	}
	firstJSON, err := RenderJSON(first)
	if err != nil {
		t.Fatalf("rendering first results: %v", err)
	}
	secondJSON, err := RenderJSON(second)
	if err != nil {
		t.Fatalf("rendering second results: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("render ordering changed output:\n%s\n%s", firstJSON, secondJSON)
	}
	if first[0].Capability.Kind != model.CapabilityGitHosting || first[1].Capability.Kind != model.CapabilityLSP || first[2].Capability.Kind != model.CapabilityMCP {
		t.Fatalf("unexpected order: %+v", first)
	}
}

func TestVersionProbeIsOptInAndUsesBoundedScannerProbe(t *testing.T) {
	observation := model.Observation{
		ID:           "path\x00fixture",
		DisplayName:  "fixture",
		Kind:         model.KindExecutable,
		Availability: model.AvailabilityInfo{ResolvedPath: "/definitely/not/run"},
		Locations:    []model.Location{{Path: "/definitely/not/run", Executable: true}},
		Evidence:     []model.Evidence{{Type: "filesystem", Source: "PATH"}},
	}
	registry := DefaultRegistry()
	withoutProbe := registry.Detect([]model.Observation{observation})
	if len(withoutProbe) != 1 || len(withoutProbe[0].Capability.Evidence) != 1 {
		t.Fatalf("default detection ran a probe: %+v", withoutProbe)
	}
	withDisabledProbe := registry.DetectWithOptions([]model.Observation{observation}, Options{Probe: &scanner.ProbeOptions{Enabled: false}})
	if len(withDisabledProbe) != 1 || len(withDisabledProbe[0].Capability.Evidence) != 1 {
		t.Fatalf("disabled detection ran a probe: %+v", withDisabledProbe)
	}
	withProbe := registry.DetectWithOptions([]model.Observation{observation}, Options{Probe: &scanner.ProbeOptions{Enabled: true}})
	if len(withProbe) != 1 || len(withProbe[0].Capability.Evidence) != 2 {
		t.Fatalf("enabled detection did not record bounded probe evidence: %+v", withProbe)
	}
	if !strings.HasPrefix(withProbe[0].Capability.Evidence[1].Detail, "error=") {
		t.Fatalf("unexpected probe detail: %+v", withProbe[0])
	}
}

func metadataObservation(name, packageName string) model.Observation {
	return model.Observation{
		ID:          "package\x00" + name,
		DisplayName: name,
		Origin:      model.Origin{Provider: "fixture", Package: packageName},
		Package:     &model.PackageInfo{Name: packageName},
	}
}
