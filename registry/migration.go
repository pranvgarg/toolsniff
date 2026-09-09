package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/pranvgarg/toolsniff/model"
)

const CurrentSchemaVersion = 2

// Envelope is the on-disk representation of a v2 registry.
type Envelope struct {
	SchemaVersion int                 `json:"schema_version"`
	Observations  []model.Observation `json:"observations"`
}

// MigrateTools converts a legacy registry in a stable order. It intentionally
// keeps every input item; identity deduplication belongs to scanner output,
// not persistence.
func MigrateTools(tools []model.Tool) []model.Observation {
	observations := make([]model.Observation, 0, len(tools))
	for _, tool := range tools {
		observations = append(observations, model.ObservationFromTool(tool))
	}
	sortObservations(observations)
	return observations
}

func observationsFromTools(tools []model.Tool) []model.Observation {
	observations := make([]model.Observation, 0, len(tools))
	for _, tool := range tools {
		observations = append(observations, model.ObservationFromTool(tool))
	}
	return observations
}

func decodeRegistry(data []byte) ([]model.Observation, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, fmt.Errorf("empty JSON document")
	}

	switch data[0] {
	case '[':
		var tools []model.Tool
		if err := decodeStrict(data, &tools); err != nil {
			return nil, fmt.Errorf("legacy registry: %w", err)
		}
		if tools == nil {
			return nil, fmt.Errorf("legacy registry must be a JSON array")
		}
		return MigrateTools(tools), nil
	case '{':
		var envelope Envelope
		if err := decodeStrict(data, &envelope); err != nil {
			return nil, fmt.Errorf("v2 envelope: %w", err)
		}
		if envelope.SchemaVersion != CurrentSchemaVersion {
			return nil, fmt.Errorf("unsupported schema version %d", envelope.SchemaVersion)
		}
		if envelope.Observations == nil {
			return nil, fmt.Errorf("v2 envelope observations must be an array")
		}
		for i, observation := range envelope.Observations {
			if err := observation.Validate(); err != nil {
				return nil, fmt.Errorf("v2 envelope observation %d: %w", i, err)
			}
		}
		return envelope.Observations, nil
	default:
		return nil, fmt.Errorf("registry must be a JSON array or v2 object")
	}
}

func decodeStrict(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func sortObservations(observations []model.Observation) {
	sort.SliceStable(observations, func(i, j int) bool {
		left := observationSortKey(observations[i])
		right := observationSortKey(observations[j])
		return left < right
	})
}

func observationSortKey(observation model.Observation) string {
	data, _ := json.Marshal(observation)
	return model.ObservationIdentity(observation) + "\x00" + string(data)
}

func toolsFromObservations(observations []model.Observation) []model.Tool {
	tools := make([]model.Tool, 0, len(observations))
	for _, observation := range observations {
		tools = append(tools, toolFromObservation(observation))
	}
	return tools
}

func toolFromObservation(observation model.Observation) model.Tool {
	tool := model.Tool{
		Name:   observation.DisplayName,
		Role:   observation.Role,
		Source: sourceFromObservation(observation),
	}
	if observation.Version.State == model.VersionKnown {
		tool.Version = observation.Version.Value
	}
	if observation.Kind == model.KindHistory && observation.History != nil && observation.History.LastUsed != nil {
		tool.Version = observation.History.LastUsed.Format("2006-01-02")
	}
	if len(observation.Locations) > 0 {
		tool.Path = observation.Locations[0].Path
	}
	return tool
}

func sourceFromObservation(observation model.Observation) string {
	if observation.Kind == model.KindHistory {
		return model.SourceNPXHistory
	}
	switch {
	case observation.Origin.Provider == "homebrew" && observation.Origin.Manager == "formula":
		return model.SourceBrewFormula
	case observation.Origin.Provider == "homebrew" && observation.Origin.Manager == "cask":
		return model.SourceBrewCask
	case observation.Origin.Provider == "npm" && observation.Origin.Manager == "global":
		return model.SourceNPM
	case observation.Origin.Provider == "pipx" && observation.Origin.Manager == "global":
		return model.SourcePipx
	case observation.Origin.Provider == "applications":
		return model.SourceApplications
	case observation.Origin.Provider == "unknown" && observation.Kind == model.KindExecutable:
		return model.SourcePath
	default:
		return observation.Origin.Provider
	}
}
