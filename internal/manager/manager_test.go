package manager

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"plugin-executor/internal/contract"
)

func TestLoadReturnsEmptyManagerWhenPluginDirDoesNotExist(t *testing.T) {
	pluginManager, err := Load(filepath.Join(t.TempDir(), "missing"), time.Second)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(pluginManager.Plugins()) != 0 {
		t.Fatalf("expected no plugins, got %d", len(pluginManager.Plugins()))
	}
}

func TestLoadClassifiesPluginStatuses(t *testing.T) {
	pluginDir := filepath.Join(t.TempDir(), "plugins")
	writeExecutable(t, filepath.Join(pluginDir, "enabled", "run"), "#!/bin/sh\nprintf '{\"ok\":true}\\n'\n")
	writeManifest(t, pluginDir, "enabled", `{
		"name": "enabled",
		"version": "1.0.0",
		"entry": "./run",
		"enabled": true,
		"timeout_ms": 250
	}`)
	writeManifest(t, pluginDir, "disabled", `{
		"name": "disabled",
		"version": "1.0.0",
		"entry": "./missing",
		"enabled": false
	}`)
	writeManifest(t, pluginDir, "invalid", `{
		"name": "invalid",
		"version": "1.0.0",
		"entry": "./missing",
		"enabled": true
	}`)

	pluginManager, err := Load(pluginDir, time.Second)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	plugins := pluginManager.Plugins()
	if len(plugins) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(plugins))
	}

	statuses := map[string]string{}
	for _, plugin := range plugins {
		statuses[plugin.Name] = plugin.Status
	}

	if statuses["enabled"] != contract.StatusEnabled {
		t.Fatalf("expected enabled status, got %q", statuses["enabled"])
	}
	if statuses["disabled"] != contract.StatusDisabled {
		t.Fatalf("expected disabled status, got %q", statuses["disabled"])
	}
	if statuses["invalid"] != contract.StatusInvalid {
		t.Fatalf("expected invalid status, got %q", statuses["invalid"])
	}

	enabled := pluginManager.EnabledPlugins()
	if len(enabled) != 1 || enabled[0].Name != "enabled" {
		t.Fatalf("expected only enabled plugin to run, got %#v", enabled)
	}
}

func TestLoadRejectsUnsafeManifestEntry(t *testing.T) {
	pluginDir := filepath.Join(t.TempDir(), "plugins")
	writeManifest(t, pluginDir, "unsafe", `{
		"name": "unsafe",
		"version": "1.0.0",
		"entry": "../outside",
		"enabled": true
	}`)

	pluginManager, err := Load(pluginDir, time.Second)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	plugins := pluginManager.Plugins()
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Status != contract.StatusInvalid {
		t.Fatalf("expected invalid status, got %q", plugins[0].Status)
	}
	if plugins[0].Error == "" {
		t.Fatal("expected unsafe manifest error")
	}
}

func writeManifest(t *testing.T, root, name, body string) {
	t.Helper()

	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("create plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(body), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("shell script plugin tests are not supported on Windows")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create executable dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
}
