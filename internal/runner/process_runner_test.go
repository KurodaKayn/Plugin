package runner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"plugin-executor/internal/contract"
	"plugin-executor/internal/manager"
)

func TestProcessRunnerReturnsSuccessResult(t *testing.T) {
	plugin := testPlugin(t, "success", "#!/bin/sh\nprintf '{\"ok\":true,\"data\":{\"message\":\"done\"}}\\n'\n", time.Second)

	result := ProcessRunner{DefaultTimeout: time.Second}.Run(context.Background(), plugin, testRequest())

	if result.Status != contract.StatusSuccess {
		t.Fatalf("expected success, got %#v", result)
	}
	if result.Data["message"] != "done" {
		t.Fatalf("expected response data, got %#v", result.Data)
	}
}

func TestProcessRunnerReturnsFailedResultForPluginError(t *testing.T) {
	plugin := testPlugin(t, "failed", "#!/bin/sh\nprintf '{\"ok\":false,\"error\":\"bad input\"}\\n'\n", time.Second)

	result := ProcessRunner{DefaultTimeout: time.Second}.Run(context.Background(), plugin, testRequest())

	if result.Status != contract.StatusFailed {
		t.Fatalf("expected failed, got %#v", result)
	}
	if result.Error != "bad input" {
		t.Fatalf("expected plugin error, got %q", result.Error)
	}
}

func TestProcessRunnerReturnsFailedResultForInvalidJSON(t *testing.T) {
	plugin := testPlugin(t, "invalid-json", "#!/bin/sh\nprintf 'not-json\\n'\n", time.Second)

	result := ProcessRunner{DefaultTimeout: time.Second}.Run(context.Background(), plugin, testRequest())

	if result.Status != contract.StatusFailed {
		t.Fatalf("expected failed, got %#v", result)
	}
	if result.Error == "" {
		t.Fatal("expected invalid output error")
	}
}

func TestProcessRunnerReturnsTimeoutResult(t *testing.T) {
	plugin := testPlugin(t, "timeout", "#!/bin/sh\nsleep 1\nprintf '{\"ok\":true}\\n'\n", 10*time.Millisecond)

	result := ProcessRunner{DefaultTimeout: time.Second}.Run(context.Background(), plugin, testRequest())

	if result.Status != contract.StatusTimeout {
		t.Fatalf("expected timeout, got %#v", result)
	}
	if result.Error == "" {
		t.Fatal("expected timeout error")
	}
}

func testPlugin(t *testing.T, name, body string, timeout time.Duration) manager.Plugin {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("shell script plugin tests are not supported on Windows")
	}

	dir := t.TempDir()
	entryPath := filepath.Join(dir, name)
	if err := os.WriteFile(entryPath, []byte(body), 0755); err != nil {
		t.Fatalf("write plugin script: %v", err)
	}

	return manager.Plugin{
		Name:      name,
		Version:   "1.0.0",
		Dir:       dir,
		EntryPath: entryPath,
		Enabled:   true,
		Timeout:   timeout,
		Status:    contract.StatusEnabled,
	}
}

func testRequest() contract.PluginRequest {
	return contract.PluginRequest{
		RequestID: "req-test",
		Data: map[string]any{
			"message": "hello",
		},
	}
}
