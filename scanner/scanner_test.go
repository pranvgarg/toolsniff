// scanner/scanner_test.go
package scanner

import (
	"errors"
	"sort"
	"testing"

	"github.com/pranvgarg/toolsniff/model"
)

type stubScanner struct {
	name  string
	tools []model.Tool
	err   error
}

func (s stubScanner) Name() string                { return s.name }
func (s stubScanner) Scan() ([]model.Tool, error) { return s.tools, s.err }

func TestRunAllCollectsToolsAndWarnings(t *testing.T) {
	scanners := []Scanner{
		stubScanner{name: "ok-a", tools: []model.Tool{{Name: "foo", Source: "ok-a"}}},
		stubScanner{name: "ok-b", tools: []model.Tool{{Name: "bar", Source: "ok-b"}}},
		stubScanner{name: "broken", err: errors.New("boom")},
	}

	tools, warnings := RunAll(scanners)

	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d: %+v", len(tools), tools)
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	if tools[0].Name != "bar" || tools[1].Name != "foo" {
		t.Errorf("unexpected tools: %+v", tools)
	}

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %+v", len(warnings), warnings)
	}
	if warnings[0].Source != "broken" || warnings[0].Err.Error() != "boom" {
		t.Errorf("unexpected warning: %+v", warnings[0])
	}
}

func TestRunAllEmptyInput(t *testing.T) {
	tools, warnings := RunAll(nil)
	if len(tools) != 0 || len(warnings) != 0 {
		t.Errorf("expected empty results for empty input, got tools=%+v warnings=%+v", tools, warnings)
	}
}
