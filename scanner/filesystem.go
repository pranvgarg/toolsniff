package scanner

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pranvgarg/toolsniff/model"
)

// DirReader and FileStat make filesystem scanners hermetic in tests.
type DirReader func(string) ([]os.DirEntry, error)
type FileStat func(string) (os.FileInfo, error)

// ScanExecutableDir returns executable regular files from one directory.
// Symlinks are followed for the executable check so package-manager links are
// included, while directories and non-executable files are ignored.
func ScanExecutableDir(dir, source string, readDir DirReader, stat FileStat) ([]model.Tool, error) {
	entries, err := readDir(dir)
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	tools := make([]model.Tool, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
			continue
		}
		tools = append(tools, model.Tool{
			Name:   entry.Name(),
			Source: source,
			Path:   path,
		})
	}
	return tools, nil
}

func defaultDirReader(path string) ([]os.DirEntry, error) { return os.ReadDir(path) }

func defaultFileStat(path string) (os.FileInfo, error) { return os.Stat(path) }

func isPathExcluded(path string, excluded []string) bool {
	path = filepath.Clean(path)
	for _, root := range excluded {
		root = filepath.Clean(root)
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
