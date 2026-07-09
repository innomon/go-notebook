package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-notebook/pkg/bases"
	"go-notebook/pkg/wasm"
)

func main() {
	// Handcrafted CLI flag parsing using stdlib flag package (conforming to pflag/cobra exclusion rule)
	dirPtr := flag.String("dir", ".", "Directory containing Markdown notes")
	configPtr := flag.String("config", "", "Path to the .base configuration file (required)")
	extDirPtr := flag.String("extensions", "extensions/bin", "Directory containing compiled WASM extensions")
	flag.Parse()

	if *configPtr == "" {
		fmt.Fprintf(os.Stderr, "Error: -config parameter is required\n")
		flag.Usage()
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Read and parse Base Config
	configBytes, err := os.ReadFile(*configPtr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
		os.Exit(1)
	}

	config, err := bases.LoadBaseConfig(configBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing base config: %v\n", err)
		os.Exit(1)
	}

	// 2. Read and parse Markdown Notes
	var notes []*bases.Note
	err = filepath.WalkDir(*dirPtr, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			contentBytes, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read note '%s': %w", path, err)
			}
			note, err := bases.ParseNote(path, string(contentBytes))
			if err != nil {
				return fmt.Errorf("failed to parse note '%s': %w", path, err)
			}
			notes = append(notes, note)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning notes: %v\n", err)
		os.Exit(1)
	}

	// 3. Initialize WASM manager and load plugins
	manager, err := wasm.NewManager(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing WASM manager: %v\n", err)
		os.Exit(1)
	}
	defer manager.Close(ctx)

	// Load all .wasm files from extensions dir
	if info, err := os.Stat(*extDirPtr); err == nil && info.IsDir() {
		files, err := os.ReadDir(*extDirPtr)
		if err == nil {
			for _, file := range files {
				if !file.IsDir() && strings.HasSuffix(strings.ToLower(file.Name()), ".wasm") {
					path := filepath.Join(*extDirPtr, file.Name())
					wasmBytes, err := os.ReadFile(path)
					if err == nil {
						// Use file name without .wasm as plugin name
						pluginName := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
						err = manager.LoadPlugin(ctx, pluginName, wasmBytes)
						if err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to load plugin '%s': %v\n", pluginName, err)
						}
					}
				}
			}
		}
	}

	// 4. Define formula evaluator function
	runFormula := func(funcName string, properties map[string]any) (any, error) {
		// Define payload with properties and formula-specific arguments.
		// By default, pass the formula name as the first arg if not specified.
		payload := struct {
			Properties map[string]any `json:"properties"`
			Args       []string       `json:"args"`
		}{
			Properties: properties,
			Args:       []string{""}, // Placeholder for first arg
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		// Execute using the plugin named funcName
		resBytes, err := manager.Execute(ctx, funcName, funcName, payloadBytes)
		if err != nil {
			return nil, fmt.Errorf("plugin execution failed: %w", err)
		}

		// Try unmarshaling dynamically
		var result map[string]any
		if err := json.Unmarshal(resBytes, &result); err != nil {
			return nil, fmt.Errorf("failed to parse plugin result: %w", err)
		}

		// Return the first non-error value found
		if errVal, ok := result["error"]; ok && errVal != nil {
			return nil, fmt.Errorf("plugin error: %v", errVal)
		}

		for k, v := range result {
			if k != "error" {
				return v, nil
			}
		}

		return nil, fmt.Errorf("no output value returned from plugin")
	}

	// 5. Execute bases engine
	response, err := bases.Execute(notes, config, runFormula)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing bases engine: %v\n", err)
		os.Exit(1)
	}

	// 6. Output formatted A2UI JSON to stdout
	outBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(outBytes))
}
