package manager

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"plugin-executor/internal/contract"
)

type Manager struct {
	plugins []Plugin
}

func Load(pluginDir string, defaultTimeout time.Duration) (*Manager, error) {
	if defaultTimeout <= 0 {
		return nil, errors.New("default timeout must be positive")
	}

	entries, err := os.ReadDir(pluginDir)
	if errors.Is(err, os.ErrNotExist) {
		return &Manager{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read plugin directory: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	seenNames := make(map[string]struct{})
	plugins := make([]Plugin, 0, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		plugin := loadOne(pluginDir, entry.Name(), defaultTimeout, seenNames)
		if plugin.Status != contract.StatusInvalid {
			seenNames[plugin.Name] = struct{}{}
		}
		plugins = append(plugins, plugin)
	}

	return &Manager{plugins: plugins}, nil
}

func (m *Manager) Plugins() []Plugin {
	plugins := make([]Plugin, len(m.plugins))
	copy(plugins, m.plugins)
	return plugins
}

func (m *Manager) EnabledPlugins() []Plugin {
	var enabled []Plugin
	for _, plugin := range m.plugins {
		if plugin.Enabled && plugin.Status == contract.StatusEnabled {
			enabled = append(enabled, plugin)
		}
	}
	return enabled
}
