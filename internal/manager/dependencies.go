package manager

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"plugin-executor/internal/contract"
)

func validateDependencies(plugins []Plugin) {
	byName := pluginsByName(plugins)

	for i := range plugins {
		if plugins[i].Status != contract.StatusEnabled {
			continue
		}
		for _, dependency := range plugins[i].Dependencies {
			if err := validateDependency(plugins[i], dependency, byName); err != nil {
				plugins[i].Status = contract.StatusInvalid
				plugins[i].Error = err.Error()
				break
			}
		}
	}

	markDependencyCycles(plugins, byName)
	markBrokenDependencyChains(plugins, byName)
}

func validateDependency(plugin Plugin, dependency Dependency, byName map[string]*Plugin) error {
	if dependency.Name == "" {
		return fmt.Errorf("plugin %q has dependency without name", plugin.Name)
	}

	target, ok := byName[dependency.Name]
	if !ok {
		return fmt.Errorf("plugin %q depends on missing plugin %q", plugin.Name, dependency.Name)
	}
	if target.Status != contract.StatusEnabled {
		return fmt.Errorf("plugin %q depends on unavailable plugin %q", plugin.Name, dependency.Name)
	}
	if dependency.Version == "" {
		return nil
	}

	ok, err := versionSatisfies(target.Version, dependency.Version)
	if err != nil {
		return fmt.Errorf("plugin %q has invalid dependency constraint for %q: %w", plugin.Name, dependency.Name, err)
	}
	if !ok {
		return fmt.Errorf("plugin %q requires %q %s, got %s", plugin.Name, dependency.Name, dependency.Version, target.Version)
	}
	return nil
}

func pluginsByName(plugins []Plugin) map[string]*Plugin {
	byName := make(map[string]*Plugin, len(plugins))
	for i := range plugins {
		if plugins[i].Name != "" {
			byName[plugins[i].Name] = &plugins[i]
		}
	}
	return byName
}

func markDependencyCycles(plugins []Plugin, byName map[string]*Plugin) {
	state := make(map[string]int, len(plugins))
	var stack []string
	cycles := make(map[string]struct{})

	var visit func(string)
	visit = func(name string) {
		plugin := byName[name]
		if plugin == nil || plugin.Status != contract.StatusEnabled {
			return
		}
		state[name] = 1
		stack = append(stack, name)
		defer func() {
			stack = stack[:len(stack)-1]
			state[name] = 2
		}()

		for _, dependency := range plugin.Dependencies {
			target := byName[dependency.Name]
			if target == nil || target.Status != contract.StatusEnabled {
				continue
			}
			switch state[target.Name] {
			case 0:
				visit(target.Name)
			case 1:
				for _, name := range slices.Backward(stack) {
					cycles[name] = struct{}{}
					if name == target.Name {
						break
					}
				}
			}
		}
	}

	for i := range plugins {
		if plugins[i].Status == contract.StatusEnabled && state[plugins[i].Name] == 0 {
			visit(plugins[i].Name)
		}
	}

	for name := range cycles {
		plugin := byName[name]
		plugin.Status = contract.StatusInvalid
		plugin.Error = "dependency cycle detected"
	}
}

func markBrokenDependencyChains(plugins []Plugin, byName map[string]*Plugin) {
	changed := true
	for changed {
		changed = false
		for i := range plugins {
			if plugins[i].Status != contract.StatusEnabled {
				continue
			}
			for _, dependency := range plugins[i].Dependencies {
				target := byName[dependency.Name]
				if target != nil && target.Status == contract.StatusEnabled {
					continue
				}
				plugins[i].Status = contract.StatusInvalid
				plugins[i].Error = fmt.Sprintf("plugin %q depends on unavailable plugin %q", plugins[i].Name, dependency.Name)
				changed = true
				break
			}
		}
	}
}

func versionSatisfies(version string, constraint string) (bool, error) {
	operator, target := splitConstraint(constraint)
	comparison, err := compareVersions(version, target)
	if err != nil {
		return false, err
	}

	switch operator {
	case "=", "==":
		return comparison == 0, nil
	case ">":
		return comparison > 0, nil
	case ">=":
		return comparison >= 0, nil
	case "<":
		return comparison < 0, nil
	case "<=":
		return comparison <= 0, nil
	default:
		return false, fmt.Errorf("unsupported operator %q", operator)
	}
}

func splitConstraint(constraint string) (string, string) {
	constraint = strings.TrimSpace(constraint)
	for _, operator := range []string{">=", "<=", "==", ">", "<", "="} {
		if target, ok := strings.CutPrefix(constraint, operator); ok {
			return operator, strings.TrimSpace(target)
		}
	}
	return "=", constraint
}

func compareVersions(left string, right string) (int, error) {
	leftParts, err := parseVersion(left)
	if err != nil {
		return 0, err
	}
	rightParts, err := parseVersion(right)
	if err != nil {
		return 0, err
	}

	for i := range leftParts {
		if leftParts[i] > rightParts[i] {
			return 1, nil
		}
		if leftParts[i] < rightParts[i] {
			return -1, nil
		}
	}
	return 0, nil
}

func parseVersion(version string) ([3]int, error) {
	var parsed [3]int
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if version == "" {
		return parsed, fmt.Errorf("version is empty")
	}

	parts := strings.Split(version, ".")
	if len(parts) > len(parsed) {
		return parsed, fmt.Errorf("version %q has too many parts", version)
	}

	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil {
			return parsed, fmt.Errorf("version %q is invalid", version)
		}
		parsed[i] = value
	}
	return parsed, nil
}
