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

func TestManagerReloadsAndUnloadsPlugins(t *testing.T) {
	pluginDir := filepath.Join(t.TempDir(), "plugins")
	pluginManager, err := Load(pluginDir, time.Second)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(pluginManager.Plugins()) != 0 {
		t.Fatalf("expected no plugins before reload, got %d", len(pluginManager.Plugins()))
	}

	writeExecutable(t, filepath.Join(pluginDir, "hot", "run"), "#!/bin/sh\nprintf '{\"ok\":true}\\n'\n")
	writeManifest(t, pluginDir, "hot", `{
		"name": "hot",
		"version": "1.0.0",
		"entry": "./run",
		"enabled": true
	}`)

	if err := pluginManager.Reload(); err != nil {
		t.Fatalf("Reload returned error: %v", err)
	}
	plugins := pluginManager.Plugins()
	if len(plugins) != 1 || plugins[0].Name != "hot" {
		t.Fatalf("expected hot plugin after reload, got %#v", plugins)
	}

	if !pluginManager.Unload("hot") {
		t.Fatal("expected hot plugin to unload")
	}
	if len(pluginManager.Plugins()) != 0 {
		t.Fatalf("expected no plugins after unload, got %#v", pluginManager.Plugins())
	}
	if pluginManager.Unload("missing") {
		t.Fatal("expected missing plugin unload to return false")
	}
}

func TestLoadValidatesDependenciesAndVersionConstraints(t *testing.T) {
	pluginDir := filepath.Join(t.TempDir(), "plugins")
	writeExecutable(t, filepath.Join(pluginDir, "base", "run"), "#!/bin/sh\nprintf '{\"ok\":true}\\n'\n")
	writeManifest(t, pluginDir, "base", `{
		"name": "base",
		"version": "1.2.0",
		"entry": "./run",
		"enabled": true
	}`)
	writeExecutable(t, filepath.Join(pluginDir, "consumer", "run"), "#!/bin/sh\nprintf '{\"ok\":true}\\n'\n")
	writeManifest(t, pluginDir, "consumer", `{
		"name": "consumer",
		"version": "1.0.0",
		"entry": "./run",
		"enabled": true,
		"dependencies": [{"name": "base", "version": ">=1.0.0"}]
	}`)
	writeExecutable(t, filepath.Join(pluginDir, "missing", "run"), "#!/bin/sh\nprintf '{\"ok\":true}\\n'\n")
	writeManifest(t, pluginDir, "missing", `{
		"name": "missing",
		"version": "1.0.0",
		"entry": "./run",
		"enabled": true,
		"dependencies": [{"name": "absent"}]
	}`)
	writeExecutable(t, filepath.Join(pluginDir, "too-new", "run"), "#!/bin/sh\nprintf '{\"ok\":true}\\n'\n")
	writeManifest(t, pluginDir, "too-new", `{
		"name": "too-new",
		"version": "1.0.0",
		"entry": "./run",
		"enabled": true,
		"dependencies": [{"name": "base", "version": ">=2.0.0"}]
	}`)
	writeManifest(t, pluginDir, "disabled-base", `{
		"name": "disabled-base",
		"version": "1.0.0",
		"entry": "./missing",
		"enabled": false
	}`)
	writeExecutable(t, filepath.Join(pluginDir, "needs-disabled", "run"), "#!/bin/sh\nprintf '{\"ok\":true}\\n'\n")
	writeManifest(t, pluginDir, "needs-disabled", `{
		"name": "needs-disabled",
		"version": "1.0.0",
		"entry": "./run",
		"enabled": true,
		"dependencies": [{"name": "disabled-base"}]
	}`)

	pluginManager, err := Load(pluginDir, time.Second)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	statuses := pluginStatuses(pluginManager.Plugins())
	if statuses["base"] != contract.StatusEnabled {
		t.Fatalf("expected base enabled, got %q", statuses["base"])
	}
	if statuses["consumer"] != contract.StatusEnabled {
		t.Fatalf("expected consumer enabled, got %q", statuses["consumer"])
	}
	if statuses["missing"] != contract.StatusInvalid {
		t.Fatalf("expected missing dependency invalid, got %q", statuses["missing"])
	}
	if statuses["too-new"] != contract.StatusInvalid {
		t.Fatalf("expected unsatisfied version invalid, got %q", statuses["too-new"])
	}
	if statuses["needs-disabled"] != contract.StatusInvalid {
		t.Fatalf("expected disabled dependency invalid, got %q", statuses["needs-disabled"])
	}

	enabled := pluginManager.EnabledPlugins()
	if len(enabled) != 2 || enabled[0].Name != "base" || enabled[1].Name != "consumer" {
		t.Fatalf("expected dependency-first enabled order, got %#v", enabled)
	}
}

func TestLoadRejectsDependencyCycles(t *testing.T) {
	pluginDir := filepath.Join(t.TempDir(), "plugins")
	writeExecutable(t, filepath.Join(pluginDir, "a", "run"), "#!/bin/sh\nprintf '{\"ok\":true}\\n'\n")
	writeManifest(t, pluginDir, "a", `{
		"name": "a",
		"version": "1.0.0",
		"entry": "./run",
		"enabled": true,
		"dependencies": [{"name": "b"}]
	}`)
	writeExecutable(t, filepath.Join(pluginDir, "b", "run"), "#!/bin/sh\nprintf '{\"ok\":true}\\n'\n")
	writeManifest(t, pluginDir, "b", `{
		"name": "b",
		"version": "1.0.0",
		"entry": "./run",
		"enabled": true,
		"dependencies": [{"name": "a"}]
	}`)

	pluginManager, err := Load(pluginDir, time.Second)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	statuses := pluginStatuses(pluginManager.Plugins())
	if statuses["a"] != contract.StatusInvalid || statuses["b"] != contract.StatusInvalid {
		t.Fatalf("expected cycle participants invalid, got %#v", statuses)
	}
	if len(pluginManager.EnabledPlugins()) != 0 {
		t.Fatalf("expected no enabled plugins, got %#v", pluginManager.EnabledPlugins())
	}
}

func pluginStatuses(plugins []Plugin) map[string]string {
	statuses := make(map[string]string, len(plugins))
	for _, plugin := range plugins {
		statuses[plugin.Name] = plugin.Status
	}
	return statuses
}

func writeManifest(t *testing.T, root, name, body string) {
	t.Helper()

	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("create plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(body), 0600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("shell script plugin tests are not supported on Windows")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("create executable dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	// #nosec G302 -- Test plugin scripts must be executable so the loader can validate them.
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatalf("make executable: %v", err)
	}
}
