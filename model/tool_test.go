package model

import (
	"encoding/json"
	"testing"
)

func TestToolMarshalJSON(t *testing.T) {
	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param1": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Verify fields are present and named correctly (snake_case)
	if _, ok := result["name"]; !ok {
		t.Errorf("missing 'name' field in JSON")
	}
	if _, ok := result["description"]; !ok {
		t.Errorf("missing 'description' field in JSON")
	}
	if _, ok := result["input_schema"]; !ok {
		t.Errorf("missing 'input_schema' field in JSON")
	}

	// Verify values
	if result["name"] != "test_tool" {
		t.Errorf("expected name='test_tool', got %v", result["name"])
	}
	if result["description"] != "A test tool" {
		t.Errorf("expected description='A test tool', got %v", result["description"])
	}
}

func TestToolUnmarshalJSON(t *testing.T) {
	jsonStr := `{
		"name": "test_tool",
		"description": "A test tool",
		"input_schema": {
			"type": "object",
			"properties": {
				"param1": {
					"type": "string"
				}
			}
		}
	}`

	var tool Tool
	if err := json.Unmarshal([]byte(jsonStr), &tool); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if tool.Name != "test_tool" {
		t.Errorf("expected Name='test_tool', got %q", tool.Name)
	}
	if tool.Description != "A test tool" {
		t.Errorf("expected Description='A test tool', got %q", tool.Description)
	}
	if tool.InputSchema == nil {
		t.Errorf("expected InputSchema to be non-nil")
	}
}
