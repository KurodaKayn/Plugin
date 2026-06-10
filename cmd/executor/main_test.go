package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadInputFromInlineJSON(t *testing.T) {
	data, err := readInput(`{"message":"hello"}`, "")
	if err != nil {
		t.Fatalf("readInput returned error: %v", err)
	}

	if data["message"] != "hello" {
		t.Fatalf("expected message field, got %#v", data)
	}
}

func TestReadInputFromFile(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(inputPath, []byte(`{"message":"from-file"}`), 0600); err != nil {
		t.Fatalf("write input file: %v", err)
	}

	data, err := readInput("", inputPath)
	if err != nil {
		t.Fatalf("readInput returned error: %v", err)
	}

	if data["message"] != "from-file" {
		t.Fatalf("expected file message, got %#v", data)
	}
}

func TestRunWithMissingPluginDirectoryReturnsEmptySummary(t *testing.T) {
	var stdout bytes.Buffer

	err := run([]string{
		"run",
		"--plugins", filepath.Join(t.TempDir(), "missing"),
		"--request-id", "req-empty",
		"--input", `{"message":"hello"}`,
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	var summary struct {
		RequestID string `json:"request_id"`
		Results   []any  `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
		t.Fatalf("parse summary JSON: %v", err)
	}
	if summary.RequestID != "req-empty" {
		t.Fatalf("expected request id, got %q", summary.RequestID)
	}
	if len(summary.Results) != 0 {
		t.Fatalf("expected no results, got %#v", summary.Results)
	}
}

func TestReloadWithMissingPluginDirectoryReturnsEmptyList(t *testing.T) {
	var stdout bytes.Buffer

	err := run([]string{
		"reload",
		"--plugins", filepath.Join(t.TempDir(), "missing"),
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("reload returned error: %v", err)
	}

	var plugins []any
	if err := json.Unmarshal(stdout.Bytes(), &plugins); err != nil {
		t.Fatalf("parse plugins JSON: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected no plugins, got %#v", plugins)
	}
}

func TestUnloadRemovesLoadedPlugin(t *testing.T) {
	pluginDir := filepath.Join(t.TempDir(), "plugins")
	pluginPath := filepath.Join(pluginDir, "disabled")
	if err := os.MkdirAll(pluginPath, 0700); err != nil {
		t.Fatalf("create plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginPath, "plugin.json"), []byte(`{
		"name": "disabled",
		"version": "1.0.0",
		"entry": "./missing",
		"enabled": false
	}`), 0600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	var stdout bytes.Buffer
	err := run([]string{
		"unload",
		"--plugins", pluginDir,
		"--name", "disabled",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("unload returned error: %v", err)
	}

	var plugins []any
	if err := json.Unmarshal(stdout.Bytes(), &plugins); err != nil {
		t.Fatalf("parse plugins JSON: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("expected no plugins after unload, got %#v", plugins)
	}
}
