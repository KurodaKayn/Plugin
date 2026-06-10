package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"plugin-executor/internal/contract"
	"plugin-executor/internal/manager"
)

type ProcessRunner struct {
	DefaultTimeout time.Duration
}

func (r ProcessRunner) Run(ctx context.Context, plugin manager.Plugin, request contract.PluginRequest) contract.PluginResult {
	timeout := plugin.Timeout
	if timeout <= 0 {
		timeout = r.DefaultTimeout
	}

	result := contract.PluginResult{
		Plugin:  plugin.Name,
		Version: plugin.Version,
		Status:  contract.StatusFailed,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		result.Error = fmt.Sprintf("encode request: %v", err)
		return result
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// #nosec G204 -- The plugin entry path comes from a validated manifest and is the execution model.
	cmd := exec.CommandContext(runCtx, plugin.EntryPath)
	cmd.Dir = plugin.Dir
	cmd.Stdin = bytes.NewReader(payload)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			result.Status = contract.StatusTimeout
			result.Error = fmt.Sprintf("plugin timed out after %s", timeout)
			return result
		}

		result.Error = joinError("plugin execution failed", err.Error(), stderr.String())
		return result
	}

	output := bytes.TrimSpace(stdout.Bytes())
	if len(output) == 0 {
		result.Error = "plugin produced empty output"
		return result
	}

	var response contract.PluginResponse
	if err := json.Unmarshal(output, &response); err != nil {
		result.Error = fmt.Sprintf("invalid plugin output: %v", err)
		return result
	}

	if !response.OK {
		result.Error = strings.TrimSpace(response.Error)
		if result.Error == "" {
			result.Error = "plugin returned ok=false"
		}
		return result
	}

	result.Status = contract.StatusSuccess
	result.Data = response.Data
	return result
}

func joinError(prefix, errText, stderrText string) string {
	parts := []string{prefix}
	if strings.TrimSpace(errText) != "" {
		parts = append(parts, strings.TrimSpace(errText))
	}
	if strings.TrimSpace(stderrText) != "" {
		parts = append(parts, "stderr: "+strings.TrimSpace(stderrText))
	}
	return strings.Join(parts, ": ")
}
