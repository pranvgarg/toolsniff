// scanner/npx.go
package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pranvgarg/toolsniff/model"
)

// NPXScanner reads npm's npx run-cache to find what packages have been
// invoked via `npx` (not npm-installed, just cached from a one-off run).
type NPXScanner struct {
	npxDir string
}

func NewNPXScanner(npxDir string) *NPXScanner {
	return &NPXScanner{npxDir: npxDir}
}

// DefaultNPXDir returns ~/.npm/_npx, npm's actual cache location.
func DefaultNPXDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".npm", "_npx")
}

func (s *NPXScanner) Name() string { return model.SourceNPXHistory }

func (s *NPXScanner) Scan() ([]model.Tool, error) {
	entries, err := os.ReadDir(s.npxDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", model.SourceNPXHistory, err)
	}

	seen := map[string]time.Time{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		hashDir := filepath.Join(s.npxDir, entry.Name())
		binDir := filepath.Join(hashDir, "node_modules", ".bin")
		binEntries, err := os.ReadDir(binDir)
		if err != nil {
			continue
		}

		var modTime time.Time
		if info, err := entry.Info(); err == nil {
			modTime = info.ModTime()
		}

		for _, bin := range binEntries {
			linkPath := filepath.Join(binDir, bin.Name())
			target, err := os.Readlink(linkPath)
			if err != nil {
				continue
			}
			// Relative symlink targets are resolved relative to the
			// directory containing the link, not the current working
			// directory, so join against binDir before inspecting the
			// path for its node_modules/<pkg> segment.
			if !filepath.IsAbs(target) {
				target = filepath.Join(binDir, target)
			}
			pkgName := packageNameFromBinTarget(target)
			if pkgName == "" {
				continue
			}
			if existing, ok := seen[pkgName]; !ok || modTime.After(existing) {
				seen[pkgName] = modTime
			}
		}
	}

	tools := make([]model.Tool, 0, len(seen))
	for name, modTime := range seen {
		version := ""
		if !modTime.IsZero() {
			version = modTime.Format("2006-01-02")
		}
		tools = append(tools, model.Tool{Name: name, Source: model.SourceNPXHistory, Role: model.RoleHistory, Version: version})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}

// packageNameFromBinTarget extracts the owning package name from a
// node_modules/.bin symlink target, handling scoped packages (@scope/pkg).
func packageNameFromBinTarget(target string) string {
	parts := strings.Split(filepath.ToSlash(target), "/")
	for i, p := range parts {
		if p != "node_modules" || i+1 >= len(parts) {
			continue
		}
		if strings.HasPrefix(parts[i+1], "@") && i+2 < len(parts) {
			return parts[i+1] + "/" + parts[i+2]
		}
		return parts[i+1]
	}
	return ""
}
