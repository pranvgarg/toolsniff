// Package capabilities matches explicit capability evidence against
// observations. It does not inspect display names and does not execute
// commands unless a caller explicitly supplies enabled probe options.
package capabilities

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
	"github.com/pranvgarg/toolsniff/scanner"
)

// Adapter is a side-effect-free capability matcher. Match must use explicit
// observation metadata or evidence; display names are not sufficient proof.
type Adapter interface {
	Name() string
	Capability() model.CapabilityName
	Match(model.Observation) (model.CapabilityEvidence, bool)
}

// ProbeAdapter is an optional extension for an adapter that can enrich a
// matched result. Registry detection never calls Probe unless options contain
// an enabled scanner probe configuration.
type ProbeAdapter interface {
	Adapter
	Probe(model.Observation, scanner.ProbeOptions) model.CapabilityEvidence
}

// MatchSpec describes exact metadata accepted by a MetadataAdapter. Empty
// fields do not match; this prevents a partially described adapter from
// becoming a display-name heuristic.
type MatchSpec struct {
	Capability model.CapabilityName
	Providers  []string
	Managers   []string
	Packages   []string
}

// NewMetadataAdapter creates an adapter that matches exact source/package
// metadata or explicit capability evidence. Package metadata is read from
// Origin.Package or Package.Name, never DisplayName.
func NewMetadataAdapter(name string, spec MatchSpec) Adapter {
	return metadataAdapter{name: name, spec: normalizeSpec(spec)}
}

type metadataAdapter struct {
	name string
	spec MatchSpec
}

func (a metadataAdapter) Name() string                     { return a.name }
func (a metadataAdapter) Capability() model.CapabilityName { return a.spec.Capability }
func (a metadataAdapter) Match(observation model.Observation) (model.CapabilityEvidence, bool) {
	if evidence, ok := explicitEvidence(observation, a.spec.Capability); ok {
		return evidence, true
	}
	if len(a.spec.Packages) == 0 && len(a.spec.Managers) == 0 && len(a.spec.Providers) == 0 {
		return model.CapabilityEvidence{}, false
	}
	if !matchesAny(a.spec.Providers, observation.Origin.Provider) ||
		!matchesAny(a.spec.Managers, observation.Origin.Manager) ||
		!matchesAny(a.spec.Packages, packageName(observation)) {
		return model.CapabilityEvidence{}, false
	}
	return model.CapabilityEvidence{
		Type:   "metadata",
		Source: a.name,
		Detail: metadataDetail(observation),
	}, true
}

func normalizeSpec(spec MatchSpec) MatchSpec {
	spec.Providers = normalizeValues(spec.Providers)
	spec.Managers = normalizeValues(spec.Managers)
	spec.Packages = normalizeValues(spec.Packages)
	return spec
}

func normalizeValues(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func matchesAny(values []string, actual string) bool {
	if len(values) == 0 {
		return true
	}
	for _, value := range values {
		if strings.EqualFold(value, actual) {
			return true
		}
	}
	return false
}

func packageName(observation model.Observation) string {
	if observation.Origin.Package != "" {
		return observation.Origin.Package
	}
	if observation.Package != nil {
		return observation.Package.Name
	}
	return ""
}

func metadataDetail(observation model.Observation) string {
	parts := make([]string, 0, 3)
	if observation.Origin.Provider != "" {
		parts = append(parts, observation.Origin.Provider)
	}
	if observation.Origin.Manager != "" {
		parts = append(parts, observation.Origin.Manager)
	}
	if name := packageName(observation); name != "" {
		parts = append(parts, name)
	}
	return strings.Join(parts, "/")
}

func explicitEvidence(observation model.Observation, capability model.CapabilityName) (model.CapabilityEvidence, bool) {
	for _, evidence := range observation.Evidence {
		if evidence.Type == "capability" && strings.EqualFold(evidence.Source, string(capability)) {
			return model.CapabilityEvidence{Type: evidence.Type, Source: evidence.Source, Detail: evidence.Description}, true
		}
		if evidence.Type == "capability:"+string(capability) {
			return model.CapabilityEvidence{Type: evidence.Type, Source: evidence.Source, Detail: evidence.Description}, true
		}
	}
	return model.CapabilityEvidence{}, false
}

type versionProbeAdapter struct{}

func (versionProbeAdapter) Name() string                     { return "version-probe" }
func (versionProbeAdapter) Capability() model.CapabilityName { return model.CapabilityVersionProbe }
func (versionProbeAdapter) Match(observation model.Observation) (model.CapabilityEvidence, bool) {
	if observation.Kind != model.KindExecutable || observation.Availability.ResolvedPath == "" {
		return model.CapabilityEvidence{}, false
	}
	for _, location := range observation.Locations {
		if location.Path == observation.Availability.ResolvedPath && location.Executable {
			for _, evidence := range observation.Evidence {
				if evidence.Type == "filesystem" && evidence.Source == "PATH" {
					return model.CapabilityEvidence{Type: evidence.Type, Source: evidence.Source, Detail: "active executable"}, true
				}
			}
		}
	}
	return model.CapabilityEvidence{}, false
}

func (versionProbeAdapter) Probe(observation model.Observation, options scanner.ProbeOptions) model.CapabilityEvidence {
	result, _ := scanner.ProbeExecutable(observation.Availability.ResolvedPath, options)
	detail := ""
	if result.Version != "" {
		detail = "version=" + result.Version
	} else if result.Error != "" {
		detail = "error=" + result.Error
	}
	return model.CapabilityEvidence{Type: "probe", Source: observation.Availability.ResolvedPath, Detail: detail}
}

// Result associates a detected capability with the observation that supplied
// its evidence. Results are intentionally not added to ObservationReport, so
// existing report JSON remains unchanged.
type Result struct {
	ObservationID string           `json:"observation_id"`
	DisplayName   string           `json:"display_name"`
	Capability    model.Capability `json:"capability"`
	adapterName   string
}

// Options controls optional enrichment. A nil Probe or a disabled Probe is
// guaranteed not to execute a process.
type Options struct {
	Probe *scanner.ProbeOptions
}

// Registry contains a deterministic set of adapters.
type Registry struct {
	adapters []Adapter
}

func NewRegistry(adapters ...Adapter) *Registry {
	registry := &Registry{}
	for _, adapter := range adapters {
		_ = registry.Register(adapter)
	}
	return registry
}

func (r *Registry) Register(adapter Adapter) error {
	if adapter == nil {
		return errors.New("capabilities: nil adapter")
	}
	if strings.TrimSpace(adapter.Name()) == "" {
		return errors.New("capabilities: adapter name is required")
	}
	if !adapter.Capability().IsValid() {
		return fmt.Errorf("capabilities: invalid capability %q", adapter.Capability())
	}
	r.adapters = append(r.adapters, adapter)
	sort.SliceStable(r.adapters, func(i, j int) bool {
		if r.adapters[i].Capability() != r.adapters[j].Capability() {
			return r.adapters[i].Capability() < r.adapters[j].Capability()
		}
		return r.adapters[i].Name() < r.adapters[j].Name()
	})
	return nil
}

func (r *Registry) Adapters() []Adapter { return append([]Adapter(nil), r.adapters...) }

// DefaultRegistry returns only adapters whose matching rules use explicit
// metadata or evidence. It has no network adapter and no command execution
// path unless DetectWithOptions receives enabled probe options.
func DefaultRegistry() *Registry {
	return NewRegistry(
		NewMetadataAdapter("interactive-cli-metadata", MatchSpec{
			Capability: model.CapabilityInteractiveCLI,
			Managers:   []string{"interactive-cli"},
		}),
		NewMetadataAdapter("mcp-package", MatchSpec{
			Capability: model.CapabilityMCP,
			Packages:   []string{"mcp", "@modelcontextprotocol/sdk", "@modelcontextprotocol/server-filesystem"},
		}),
		NewMetadataAdapter("git-hosting-package", MatchSpec{
			Capability: model.CapabilityGitHosting,
			Packages:   []string{"gh", "glab", "hub"},
		}),
		NewMetadataAdapter("lsp-package", MatchSpec{
			Capability: model.CapabilityLSP,
			Packages:   []string{"clangd", "gopls", "pyright", "rust-analyzer", "sourcekit-lsp", "typescript-language-server", "zls"},
		}),
		versionProbeAdapter{},
	)
}

// Detect is the safe default and never executes a process.
func (r *Registry) Detect(observations []model.Observation) []Result {
	return r.DetectWithOptions(observations, Options{})
}

func (r *Registry) DetectWithOptions(observations []model.Observation, options Options) []Result {
	results := make([]Result, 0)
	for _, observation := range observations {
		for _, adapter := range r.adapters {
			evidence, ok := adapter.Match(observation)
			if !ok {
				continue
			}
			result := Result{
				ObservationID: observation.ID,
				DisplayName:   observation.DisplayName,
				adapterName:   adapter.Name(),
				Capability: model.Capability{
					Kind:     adapter.Capability(),
					State:    model.CapabilityDetected,
					Evidence: []model.CapabilityEvidence{evidence},
				},
			}
			if options.Probe != nil && options.Probe.Enabled {
				if probeAdapter, ok := adapter.(ProbeAdapter); ok {
					result.Capability.Evidence = append(result.Capability.Evidence, probeAdapter.Probe(observation, *options.Probe))
				}
			}
			results = append(results, result)
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Capability.Kind != results[j].Capability.Kind {
			return results[i].Capability.Kind < results[j].Capability.Kind
		}
		if results[i].ObservationID != results[j].ObservationID {
			return results[i].ObservationID < results[j].ObservationID
		}
		if results[i].DisplayName != results[j].DisplayName {
			return results[i].DisplayName < results[j].DisplayName
		}
		return results[i].adapterName < results[j].adapterName
	})
	return results
}

// RenderJSON returns stable machine-readable capability output. Results must
// already be created by Detect or DetectWithOptions, which provide ordering.
func RenderJSON(results []Result) ([]byte, error) {
	if results == nil {
		results = []Result{}
	}
	return json.MarshalIndent(struct {
		Capabilities []Result `json:"capabilities"`
	}{Capabilities: results}, "", "  ")
}
