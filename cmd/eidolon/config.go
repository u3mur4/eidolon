package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type FlagReplacement struct {
	From []string `json:"from"`
	To   []string `json:"to"`
}

type CommandConfig struct {
	Binary string            `json:"binary"`
	Env    map[string]string `json:"env"`
	Flags  []FlagReplacement `json:"flags"`
}

type Config struct {
	Server   string                   `json:"server"`
	Global   CommandConfig            `json:"global"`
	Commands map[string]CommandConfig `json:"commands"`
}

func loadConfig() (Config, error) {
	// Default values
	config := Config{
		Server: "127.0.0.1:9999",
		Global: CommandConfig{
			Env: make(map[string]string),
		},
		Commands: make(map[string]CommandConfig),
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return config, err
	}

	eidolonDir := filepath.Join(configDir, "eidolon")
	if err := os.MkdirAll(eidolonDir, 0755); err != nil {
		return config, err
	}

	path := filepath.Join(eidolonDir, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return config, err
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return config, err
	}

	return config, nil
}
