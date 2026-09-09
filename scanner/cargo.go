// scanner/cargo.go
package scanner

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pranvgarg/toolsniff/model"
)

// CargoScanner lists binaries installed via `cargo install` into ~/.cargo/bin.
type CargoScanner struct {
	binDir string
}

func NewCargoScanner(binDir string) *CargoScanner {
	return &CargoScanner{binDir: binDir}
}

// DefaultCargoBinDir returns ~/.cargo/bin.
func DefaultCargoBinDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cargo", "bin")
}

func (s *CargoScanner) Name() string { return model.SourceCargo }

func (s *CargoScanner) Scan() ([]model.Tool, error) {
	tools, err := ScanExecutableDir(s.binDir, model.SourceCargo, defaultDirReader, defaultFileStat)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cargo: %w", err)
	}
	return tools, nil
}

// ScanObservations adapts cargo's executable discovery to the v2 model. Cargo
// does not provide package metadata in this directory listing, so no package
// identity or version is fabricated.
func (s *CargoScanner) ScanObservations() ([]model.Observation, error) {
	tools, err := s.Scan()
	if err != nil {
		return nil, err
	}
	return CargoObservationsFromTools(tools), nil
}

func CargoObservationsFromTools(tools []model.Tool) []model.Observation {
	observations := make([]model.Observation, 0, len(tools))
	for _, tool := range tools {
		observations = append(observations, cargoObservation(tool))
	}
	return observations
}

func cargoObservation(tool model.Tool) model.Observation {
	observation := model.Observation{
		DisplayName: tool.Name,
		CommandName: tool.Name,
		Kind:        model.KindExecutable,
		Role:        model.RoleInstalled,
		Origin: model.Origin{
			Provider: "cargo",
			Manager:  "install",
		},
		Version:      packageVersionInfo("", "filesystem"),
		Availability: model.AvailabilityInfo{State: model.AvailabilityUnknown},
		Evidence: []model.Evidence{{
			Type:   "filesystem",
			Source: "cargo bin directory",
		}},
	}
	if tool.Path != "" {
		observation.Locations = []model.Location{{
			Path:       tool.Path,
			Type:       model.LocationExecutable,
			Executable: true,
		}}
	}
	observation.ID = model.ObservationIdentity(observation)
	return observation
}
