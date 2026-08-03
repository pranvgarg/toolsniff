package scanner

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/pranvgarg/toolsniff/model"
)

// ExecutableCandidate is filesystem evidence for one command path. Broken
// symlinks are retained so callers can report stale PATH entries instead of
// silently losing them.
type ExecutableCandidate struct {
	Tool     model.Tool
	Location model.Location
	Broken   bool
	Error    string
}

// ScanExecutableDirEvidence returns executable and broken symlink candidates
// from one directory. It retains symlink metadata while checking the target's
// regular-file and executable permissions.
func ScanExecutableDirEvidence(dir, source string) ([]ExecutableCandidate, error) {
	return scanExecutableDirEvidence(dir, source, defaultDirReader, os.Lstat, os.Stat, os.Readlink)
}

func scanExecutableDirEvidence(dir, source string, readDir DirReader, lstat, stat FileStat, readlink func(string) (string, error)) ([]ExecutableCandidate, error) {
	entries, err := readDir(dir)
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	candidates := make([]ExecutableCandidate, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		info, err := lstat(path)
		if err != nil {
			if entry.Type()&os.ModeSymlink != 0 {
				candidates = append(candidates, brokenCandidate(entry.Name(), source, path, err))
			}
			continue
		}

		location := model.Location{
			Path:       path,
			Type:       model.LocationExecutable,
			IsSymlink:  info.Mode()&os.ModeSymlink != 0,
			Executable: false,
		}
		modifiedAt := info.ModTime()
		location.ModifiedAt = &modifiedAt
		if location.IsSymlink {
			location.SymlinkTarget, err = readlink(path)
			if err != nil {
				candidates = append(candidates, ExecutableCandidate{
					Tool:     model.Tool{Name: entry.Name(), Source: source, Role: model.RoleAvailable, Path: path},
					Location: location,
					Broken:   true,
					Error:    err.Error(),
				})
				continue
			}

			targetInfo, targetErr := stat(path)
			if targetErr != nil {
				candidates = append(candidates, ExecutableCandidate{
					Tool:     model.Tool{Name: entry.Name(), Source: source, Role: model.RoleAvailable, Path: path},
					Location: location,
					Broken:   true,
					Error:    targetErr.Error(),
				})
				continue
			}
			if !targetInfo.Mode().IsRegular() || targetInfo.Mode()&0111 == 0 {
				candidates = append(candidates, ExecutableCandidate{
					Tool:     model.Tool{Name: entry.Name(), Source: source, Role: model.RoleAvailable, Path: path},
					Location: location,
					Broken:   true,
					Error:    "symlink target is not an executable regular file",
				})
				continue
			}
			location.Executable = true
			candidates = append(candidates, ExecutableCandidate{
				Tool: toolForCandidate(entry.Name(), source, path), Location: location,
			})
			continue
		}

		if !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
			continue
		}
		location.Executable = true
		candidates = append(candidates, ExecutableCandidate{
			Tool: toolForCandidate(entry.Name(), source, path), Location: location,
		})
	}
	return candidates, nil
}

func toolForCandidate(name, source, path string) model.Tool {
	return model.Tool{Name: name, Source: source, Path: path, Role: model.RoleAvailable}
}

func brokenCandidate(name, source, path string, err error) ExecutableCandidate {
	return ExecutableCandidate{
		Tool:     model.Tool{Name: name, Source: source, Path: path, Role: model.RoleAvailable},
		Location: model.Location{Path: path, Type: model.LocationExecutable, IsSymlink: true},
		Broken:   true,
		Error:    err.Error(),
	}
}

// PathCandidate adds PATH order and active-command state to filesystem
// evidence.
type PathCandidate struct {
	ExecutableCandidate
	PATHIndex int
	Active    bool
}

// PathEvidence groups every matching candidate for one command name. The
// candidate slice remains in PATH order; ActivePath is the first usable entry.
type PathEvidence struct {
	Name            string
	Source          string
	Candidates      []PathCandidate
	ActivePath      string
	ActivePATHIndex int
	ShadowedBy      []string
	Probe           *model.ProbeResult
	ProbeVersion    model.VersionInfo
}

// ScanEvidence discovers all PATH candidates while preserving the configured
// directory order. Missing PATH directories remain tolerant, matching Scan.
func (s *PathScanner) ScanEvidence() ([]PathEvidence, error) {
	return s.ScanEvidenceWithOptions(PathScanOptions{})
}

// ScanEvidenceWithOptions discovers PATH evidence and optionally probes each
// command's active candidate.
func (s *PathScanner) ScanEvidenceWithOptions(options PathScanOptions) ([]PathEvidence, error) {
	byName := make(map[string]*PathEvidence)
	var scanErr error

	for pathIndex, dir := range s.directories {
		dir = filepath.Clean(dir)
		if isPathExcluded(dir, s.excluded) {
			continue
		}

		found, err := scanExecutableDirEvidence(dir, model.SourcePath, s.readDir, s.lstat, s.stat, s.readlink)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			scanErr = joinErrors(scanErr, fmtScanError(model.SourcePath, dir, err))
			continue
		}
		for _, candidate := range found {
			if _, ignored := s.ignoreNames[candidate.Tool.Name]; ignored {
				continue
			}
			evidence := byName[candidate.Tool.Name]
			if evidence == nil {
				evidence = &PathEvidence{Name: candidate.Tool.Name, Source: model.SourcePath, ActivePATHIndex: -1}
				byName[candidate.Tool.Name] = evidence
			}
			evidence.Candidates = append(evidence.Candidates, PathCandidate{
				ExecutableCandidate: candidate,
				PATHIndex:           pathIndex,
			})
		}
	}

	result := make([]PathEvidence, 0, len(byName))
	for _, evidence := range byName {
		sort.SliceStable(evidence.Candidates, func(i, j int) bool {
			if evidence.Candidates[i].PATHIndex != evidence.Candidates[j].PATHIndex {
				return evidence.Candidates[i].PATHIndex < evidence.Candidates[j].PATHIndex
			}
			return evidence.Candidates[i].Tool.Path < evidence.Candidates[j].Tool.Path
		})
		for i := range evidence.Candidates {
			candidate := &evidence.Candidates[i]
			if evidence.ActivePath == "" && !candidate.Broken && candidate.Location.Executable {
				candidate.Active = true
				evidence.ActivePath = candidate.Tool.Path
				evidence.ActivePATHIndex = candidate.PATHIndex
			}
		}
		if evidence.ActivePath != "" {
			for _, candidate := range evidence.Candidates {
				if candidate.PATHIndex > evidence.ActivePATHIndex && !candidate.Broken && candidate.Location.Executable {
					evidence.ShadowedBy = append(evidence.ShadowedBy, candidate.Tool.Path)
				}
			}
		}
		if options.Probe != nil && options.Probe.Enabled && evidence.ActivePath != "" {
			probeResult, probeVersion := ProbeExecutable(evidence.ActivePath, *options.Probe)
			evidence.Probe = &probeResult
			evidence.ProbeVersion = probeVersion
		}
		result = append(result, *evidence)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, scanErr
}

// ScanObservations adapts PATH evidence to the v2 model without changing the
// legacy Scanner interface.
func (s *PathScanner) ScanObservations(options PathScanOptions) ([]model.Observation, error) {
	evidence, err := s.ScanEvidenceWithOptions(options)
	observations := make([]model.Observation, 0, len(evidence))
	for _, item := range evidence {
		observations = append(observations, item.Observation(options.Probe))
	}
	return observations, err
}

// Observation converts source-specific PATH evidence into one executable
// observation containing every candidate location.
func (e PathEvidence) Observation(probe *ProbeOptions) model.Observation {
	locations := make([]model.Location, 0, len(e.Candidates))
	for _, candidate := range e.Candidates {
		if candidate.Active {
			locations = append(locations, candidate.Location)
		}
	}
	for _, candidate := range e.Candidates {
		if !candidate.Active {
			locations = append(locations, candidate.Location)
		}
	}

	availability := model.AvailabilityInfo{
		State:        model.AvailabilityUnavailable,
		PATHIndex:    e.ActivePATHIndex,
		ResolvedPath: e.ActivePath,
		ShadowedBy:   append([]string(nil), e.ShadowedBy...),
	}
	version := model.VersionInfo{
		State:       model.VersionNotReported,
		Scheme:      model.SchemeUnknown,
		Confidence:  model.ConfidenceLow,
		RetrievedBy: "filesystem",
	}
	if e.ActivePath != "" {
		availability.State = model.AvailabilityAvailable
	}
	observation := model.Observation{
		DisplayName:  e.Name,
		CommandName:  e.Name,
		Kind:         model.KindExecutable,
		Role:         model.RoleAvailable,
		Origin:       model.Origin{Provider: "unknown", Manager: "manual-or-unknown"},
		Version:      version,
		Locations:    locations,
		Availability: availability,
		Evidence:     []model.Evidence{{Type: "filesystem", Source: "PATH"}},
	}
	if e.Probe != nil {
		observation.Availability.Probe = e.Probe
		observation.Version = e.ProbeVersion
		observation.Evidence = append(observation.Evidence, model.Evidence{Type: "probe", Source: "probe"})
	} else if probe != nil && probe.Enabled && e.ActivePath != "" {
		probeResult, probeVersion := ProbeExecutable(e.ActivePath, *probe)
		observation.Availability.Probe = &probeResult
		observation.Version = probeVersion
		observation.Evidence = append(observation.Evidence, model.Evidence{Type: "probe", Source: "probe"})
	}
	observation.ID = model.ObservationIdentity(observation)
	return observation
}
