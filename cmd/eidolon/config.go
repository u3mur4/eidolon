package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Server string            `json:"server"`
	Binary string            `json:"binary"`
	Env    map[string]string `json:"env"`
}

func loadConfig(cmdName string) Config {
	// Default values
	config := Config{
		Server: "127.0.0.1:9999",
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return config
	}

	// Paths to check in order of priority
	paths := []string{
		filepath.Join(home, ".config", "eidolon", cmdName, "config.json"),
		filepath.Join(home, ".config", "eidolon", "config.json"),
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			var loadedConfig Config
			if err := json.Unmarshal(data, &loadedConfig); err == nil {
				// Merge loaded config into default
				if loadedConfig.Server != "" {
					config.Server = loadedConfig.Server
				}
				if loadedConfig.Binary != "" {
					config.Binary = loadedConfig.Binary
				}
				if loadedConfig.Env != nil {
					if config.Env == nil {
						config.Env = make(map[string]string)
					}
					for k, v := range loadedConfig.Env {
						config.Env[k] = v
					}
				}
				return config
			}
		}
	}

	return config
}

func resolveBinary(cmdName string, config Config) (string, error) {
	// 1. If binary is explicitly set in config, use it
	if config.Binary != "" {
		return config.Binary, nil
	}

	// 2. Otherwise, find the real binary in PATH, skipping our own directory to avoid infinite loops
	selfPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	selfDir := filepath.Dir(selfPath)

	pathEnv := os.Getenv("PATH")
	paths := filepath.SplitList(pathEnv)

	for _, p := range paths {
		// Skip our own directory to avoid infinite recursion if we are symlinked in PATH
		if filepath.Clean(p) == filepath.Clean(selfDir) {
			continue
		}

		fullPath := filepath.Join(p, cmdName)
		info, err := os.Stat(fullPath)
		if err == nil && !info.IsDir() && (info.Mode()&0111 != 0) {
			return fullPath, nil
		}
	}

	return "", fmt.Errorf("binary %q not found in PATH (excluding %s)", cmdName, selfDir)
}
