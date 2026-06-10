package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"plugin-executor/internal/contract"
)

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(contract.PluginResponse{
			OK:    false,
			Error: err.Error(),
		})
		os.Exit(1)
	}
}

func run(stdin io.Reader, stdout io.Writer) error {
	var request contract.PluginRequest
	if err := json.NewDecoder(stdin).Decode(&request); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}

	message, _ := request.Data["message"].(string)
	if message == "" {
		message = "hello"
	}

	response := contract.PluginResponse{
		OK: true,
		Data: map[string]any{
			"message": message + " from sample plugin",
		},
	}

	return json.NewEncoder(stdout).Encode(response)
}
