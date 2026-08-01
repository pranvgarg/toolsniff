package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
)

// ApplicationsScanner discovers application bundles under configured user
// application roots. It records every bundle instead of maintaining a list of
// known product names.
type ApplicationsScanner struct {
	roots       []string
	ignorePaths []string
}

func NewApplicationsScanner(roots, ignorePaths []string) *ApplicationsScanner {
	return &ApplicationsScanner{roots: uniquePaths(roots), ignorePaths: uniquePaths(ignorePaths)}
}

func (s *ApplicationsScanner) Name() string { return model.SourceApplications }

func (s *ApplicationsScanner) Scan() ([]model.Tool, error) {
	tools := make([]model.Tool, 0)
	var scanErr error
	seen := make(map[string]struct{})

	for _, root := range s.roots {
		root = filepath.Clean(root)
		if _, err := os.Stat(root); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			scanErr = joinErrors(scanErr, fmt.Errorf("applications: checking %s: %w", root, err))
			continue
		}

		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if !entry.IsDir() || !isApplicationBundle(entry.Name()) {
				if entry.IsDir() && isPathExcluded(path, s.ignorePaths) {
					return fs.SkipDir
				}
				return nil
			}
			if isPathExcluded(path, s.ignorePaths) {
				return fs.SkipDir
			}

			path = filepath.Clean(path)
			if _, ok := seen[path]; !ok {
				seen[path] = struct{}{}
				tools = append(tools, model.Tool{
					Name:   entry.Name(),
					Source: model.SourceApplications,
					Path:   path,
				})
			}
			// Application bundles can contain nested helper applications. They
			// are part of the bundle, not separate user-installed applications.
			return fs.SkipDir
		})
		if err != nil {
			scanErr = joinErrors(scanErr, fmt.Errorf("applications: scanning %s: %w", root, err))
		}
	}

	sortToolsByNameAndPath(tools)
	return tools, scanErr
}

func isApplicationBundle(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".app")
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	unique := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(path)
		if path == "." || path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		unique = append(unique, path)
	}
	return unique
}

func sortToolsByNameAndPath(tools []model.Tool) {
	sort.Slice(tools, func(i, j int) bool {
		if tools[i].Name != tools[j].Name {
			return tools[i].Name < tools[j].Name
		}
		return tools[i].Path < tools[j].Path
	})
}
