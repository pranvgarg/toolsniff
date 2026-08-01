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
