package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"plugin-executor/internal/config"
	"plugin-executor/internal/contract"
	"plugin-executor/internal/manager"
	"plugin-executor/internal/runner"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		writeUsage(stderr)
		return errors.New("missing command")
	}

	switch args[0] {
	case "list":
		return runList(args[1:], stdout)
	case "run":
		return runPlugins(args[1:], stdout)
	case "help", "-h", "--help":
		writeUsage(stdout)
		return nil
	default:
		writeUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runList(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("list", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	pluginDir := flags.String("plugins", config.DefaultPluginDir, "plugin directory")
	if err := flags.Parse(args); err != nil {
		return err
	}

	pluginManager, err := manager.Load(*pluginDir, config.DefaultTimeout)
	if err != nil {
		return err
	}

	return writeJSON(stdout, pluginManager.Plugins())
}

func runPlugins(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	pluginDir := flags.String("plugins", config.DefaultPluginDir, "plugin directory")
	input := flags.String("input", "", "input JSON object")
	inputFile := flags.String("file", "", "path to input JSON file")
	requestID := flags.String("request-id", "", "request id")
	if err := flags.Parse(args); err != nil {
		return err
	}

	data, err := readInput(*input, *inputFile)
	if err != nil {
		return err
	}

	if *requestID == "" {
		*requestID = fmt.Sprintf("req-%d", time.Now().UnixNano())
	}

	pluginManager, err := manager.Load(*pluginDir, config.DefaultTimeout)
	if err != nil {
		return err
	}

	request := contract.PluginRequest{
		RequestID: *requestID,
		Data:      data,
		Context: map[string]any{
			"source": "executor",
		},
	}

	processRunner := runner.ProcessRunner{DefaultTimeout: config.DefaultTimeout}
	summary := contract.ExecutionSummary{
		RequestID: request.RequestID,
		Results:   make([]contract.PluginResult, 0),
	}

	for _, plugin := range pluginManager.EnabledPlugins() {
		summary.Results = append(summary.Results, processRunner.Run(context.Background(), plugin, request))
	}

	return writeJSON(stdout, summary)
}

func readInput(input, filePath string) (map[string]any, error) {
	if input != "" && filePath != "" {
		return nil, errors.New("use only one of --input or --file")
	}
	if input == "" && filePath == "" {
		return nil, errors.New("missing input: use --input or --file")
	}

	var raw []byte
	var err error
	if filePath != "" {
		raw, err = os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read input file: %w", err)
		}
	} else {
		raw = []byte(input)
	}

	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("parse input JSON: %w", err)
	}
	if data == nil {
		return nil, errors.New("input JSON must be an object")
	}
	return data, nil
}

func writeJSON(stdout io.Writer, value any) error {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  executor list [--plugins plugins]")
	fmt.Fprintln(w, "  executor run [--plugins plugins] --input '{\"message\":\"hello\"}'")
	fmt.Fprintln(w, "  executor run [--plugins plugins] --file input.json")
}
