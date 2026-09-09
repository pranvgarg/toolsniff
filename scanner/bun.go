package scanner

import (
	"fmt"
	"os"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
)

// BunScanner discovers binaries installed by Bun globally. Bun reports its
// actual global bin directory, so this scanner does not assume a fixed home
// directory layout.
type BunScanner struct {
	runner CommandRunner
}

func NewBunScanner(runner CommandRunner) *BunScanner {
	return &BunScanner{runner: runner}
}

func (s *BunScanner) Name() string { return model.SourceBun }

func (s *BunScanner) Scan() ([]model.Tool, error) {
	out, err := runTolerant(s.runner, model.SourceBun, "bun", "pm", "bin", "-g")
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, nil
	}

	binDir := strings.TrimSpace(string(out))
	if binDir == "" {
		return nil, nil
	}
	tools, err := ScanExecutableDir(binDir, model.SourceBun, defaultDirReader, defaultFileStat)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("bun: scanning %s: %w", binDir, err)
	}
	return tools, nil
}

// ScanObservations exposes Bun's resolved global executable directory as v2
// locations without claiming package or version metadata that was not reported.
func (s *BunScanner) ScanObservations() ([]model.Observation, error) {
	tools, err := s.Scan()
	if err != nil {
		return nil, err
	}
	return BunObservationsFromTools(tools), nil
}

func BunObservationsFromTools(tools []model.Tool) []model.Observation {
	observations := make([]model.Observation, 0, len(tools))
	for _, tool := range tools {
		observation := model.Observation{
			DisplayName: tool.Name,
			CommandName: tool.Name,
			Kind:        model.KindExecutable,
			Role:        model.RoleInstalled,
			Origin: model.Origin{
				Provider: "bun",
				Manager:  "global",
			},
			Version:      packageVersionInfo("", "filesystem"),
			Availability: model.AvailabilityInfo{State: model.AvailabilityUnknown},
			Evidence: []model.Evidence{{
				Type:   "filesystem",
				Source: "bun pm bin -g",
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
		observations = append(observations, observation)
	}
	return observations
}
