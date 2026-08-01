package scanner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
)

// PathScanner discovers executable files in the directories supplied by the
// process PATH. It excludes configured system directories, not a curated list
// of executable names.
type PathScanner struct {
	directories []string
	excluded    []string
	ignoreNames map[string]struct{}
	readDir     DirReader
	stat        FileStat
}

func NewPathScanner(directories, excluded, ignoreNames []string) *PathScanner {
	ignored := make(map[string]struct{}, len(ignoreNames))
	for _, name := range ignoreNames {
		name = strings.TrimSpace(name)
		if name != "" {
			ignored[name] = struct{}{}
		}
	}
	return &PathScanner{
		directories: uniquePaths(directories),
		excluded:    uniquePaths(excluded),
		ignoreNames: ignored,
		readDir:     defaultDirReader,
		stat:        defaultFileStat,
	}
}

func (s *PathScanner) Name() string { return model.SourcePath }

func (s *PathScanner) Scan() ([]model.Tool, error) {
	tools := make([]model.Tool, 0)
	seen := make(map[string]struct{})
	var scanErr error

	for _, dir := range s.directories {
		dir = filepath.Clean(dir)
		if isPathExcluded(dir, s.excluded) {
			continue
		}

		found, err := ScanExecutableDir(dir, model.SourcePath, s.readDir, s.stat)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			scanErr = joinErrors(scanErr, fmtScanError(model.SourcePath, dir, err))
			continue
		}
		for _, tool := range found {
			if _, ignored := s.ignoreNames[tool.Name]; ignored {
				continue
			}
			if _, ok := seen[tool.Path]; ok {
				continue
			}
			seen[tool.Path] = struct{}{}
			tools = append(tools, tool)
		}
	}

	sortToolsByNameAndPath(tools)
	return tools, scanErr
}

func joinErrors(existing, next error) error {
	if existing == nil {
		return next
	}
	return errors.Join(existing, next)
}

func fmtScanError(source, path string, err error) error {
	return errors.New(source + ": scanning " + path + ": " + err.Error())
}
