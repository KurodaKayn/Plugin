package manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"plugin-executor/internal/contract"
)

type Manifest struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Entry     string `json:"entry"`
	Enabled   bool   `json:"enabled"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

type Plugin struct {
	Name      string        `json:"name"`
	Version   string        `json:"version"`
	Dir       string        `json:"-"`
	EntryPath string        `json:"-"`
	Enabled   bool          `json:"enabled"`
	Timeout   time.Duration `json:"-"`
	Status    string        `json:"status"`
	Error     string        `json:"error,omitempty"`
}

func loadOne(rootDir, dirName string, defaultTimeout time.Duration, seenNames map[string]struct{}) Plugin {
	pluginDir := filepath.Join(rootDir, dirName)
	plugin := Plugin{
		Name:    dirName,
		Dir:     pluginDir,
		Timeout: defaultTimeout,
		Status:  contract.StatusInvalid,
	}

	raw, err := os.ReadFile(filepath.Join(pluginDir, "plugin.json"))
	if err != nil {
		plugin.Error = fmt.Sprintf("read plugin.json: %v", err)
		return plugin
	}

	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		plugin.Error = fmt.Sprintf("parse plugin.json: %v", err)
		return plugin
	}

	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Entry = strings.TrimSpace(manifest.Entry)

	plugin.Name = manifest.Name
	plugin.Version = manifest.Version
	plugin.Enabled = manifest.Enabled

	if manifest.Name == "" {
		plugin.Name = dirName
		plugin.Error = "manifest name is required"
		return plugin
	}
	if _, exists := seenNames[manifest.Name]; exists {
		plugin.Error = fmt.Sprintf("duplicate plugin name %q", manifest.Name)
		return plugin
	}
	if manifest.Version == "" {
		plugin.Error = "manifest version is required"
		return plugin
	}
	if manifest.Entry == "" {
		plugin.Error = "manifest entry is required"
		return plugin
	}
	if manifest.TimeoutMS > 0 {
		plugin.Timeout = time.Duration(manifest.TimeoutMS) * time.Millisecond
	}

	entryPath, err := resolveEntryPath(pluginDir, manifest.Entry)
	if err != nil {
		plugin.Error = err.Error()
		return plugin
	}
	plugin.EntryPath = entryPath

	if !manifest.Enabled {
		plugin.Status = contract.StatusDisabled
		return plugin
	}

	if err := validateExecutable(entryPath); err != nil {
		plugin.Error = err.Error()
		return plugin
	}

	plugin.Status = contract.StatusEnabled
	return plugin
}

func resolveEntryPath(pluginDir, entry string) (string, error) {
	if filepath.IsAbs(entry) {
		return "", errors.New("manifest entry must be relative to plugin directory")
	}

	cleaned := filepath.Clean(entry)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", errors.New("manifest entry must stay inside plugin directory")
	}

	return filepath.Join(pluginDir, cleaned), nil
}

func validateExecutable(entryPath string) error {
	info, err := os.Stat(entryPath)
	if err != nil {
		return fmt.Errorf("plugin entry is not accessible: %w", err)
	}
	if info.IsDir() {
		return errors.New("plugin entry points to a directory")
	}
	if runtime.GOOS != "windows" && info.Mode()&0111 == 0 {
		return errors.New("plugin entry is not executable")
	}
	return nil
}
