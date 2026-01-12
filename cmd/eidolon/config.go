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

func (c *Config) GetBinary(cmdName string) string {
	if c.Global.Binary != "" {
		return c.Global.Binary
	}
	if cmdCfg, ok := c.Commands[cmdName]; ok && cmdCfg.Binary != "" {
		return cmdCfg.Binary
	}
	return ""
}

func (c *Config) GetEnv(cmdName string) map[string]string {
	res := make(map[string]string)

	for k, v := range c.Global.Env {
		res[k] = v
	}

	if cmdCfg, ok := c.Commands[cmdName]; ok {
		for k, v := range cmdCfg.Env {
			res[k] = v
		}
	}

	return res
}

func (c *Config) ApplyFlags(cmdName string, args []string) []string {
	replacements := append([]FlagReplacement{}, c.Global.Flags...)
	if cmdCfg, ok := c.Commands[cmdName]; ok {
		replacements = append(replacements, cmdCfg.Flags...)
	}

	if len(replacements) == 0 {
		return args
	}

	newArgs := []string{}
	for i := 0; i < len(args); {
		matched := false
		for _, repl := range replacements {
			if len(repl.From) > 0 && i+len(repl.From) <= len(args) {
				match := true
				for j := 0; j < len(repl.From); j++ {
					if args[i+j] != repl.From[j] {
						match = false
						break
					}
				}
				if match {
					newArgs = append(newArgs, repl.To...)
					i += len(repl.From)
					matched = true
					break
				}
			}
		}
		if !matched {
			newArgs = append(newArgs, args[i])
			i++
		}
	}

	return newArgs
}
