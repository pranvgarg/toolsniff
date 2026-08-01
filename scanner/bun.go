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
