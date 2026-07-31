package scanner

import "testing"

func TestPipxScannerParsesVenvs(t *testing.T) {
	fixture := []byte(`{
  "venvs": {
    "black": {
      "metadata": { "main_package": { "package_version": "24.1.0" } }
    }
  }
}`)
	runner := func(name string, args ...string) ([]byte, error) {
		if name != "pipx" {
			t.Fatalf("expected command 'pipx', got %q", name)
		}
		return fixture, nil
	}

	s := NewPipxScanner(runner)
	if s.Name() != "pipx" {
		t.Errorf("expected Name() == \"pipx\", got %q", s.Name())
	}
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "black" || tools[0].Version != "24.1.0" || tools[0].Source != "pipx" {
		t.Errorf("unexpected tools: %+v", tools)
	}
}

func TestPipxScannerEmptyVenvs(t *testing.T) {
	runner := func(name string, args ...string) ([]byte, error) {
		return []byte(`{"venvs": {}}`), nil
	}
	s := NewPipxScanner(runner)
	tools, err := s.Scan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected no tools, got %+v", tools)
	}
}
