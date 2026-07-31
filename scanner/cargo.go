// scanner/cargo.go
package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

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

func (s *CargoScanner) Name() string { return "cargo" }

func (s *CargoScanner) Scan() ([]model.Tool, error) {
	entries, err := os.ReadDir(s.binDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cargo: %w", err)
	}

	tools := make([]model.Tool, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		tools = append(tools, model.Tool{
			Name:   entry.Name(),
			Source: "cargo",
			Path:   filepath.Join(s.binDir, entry.Name()),
		})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}
