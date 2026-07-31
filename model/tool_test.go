package model

import (
	"encoding/json"
	"testing"
)

func TestToolJSONRoundTrip(t *testing.T) {
	original := Tool{Name: "ollama", Source: "applications", Version: "", Path: "/Applications/Ollama.app"}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Tool
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded != original {
		t.Errorf("round trip mismatch: got %+v, want %+v", decoded, original)
	}

	if got := string(data); got != `{"name":"ollama","source":"applications","version":"","path":"/Applications/Ollama.app"}` {
		t.Errorf("unexpected JSON shape: %s", got)
	}
}
