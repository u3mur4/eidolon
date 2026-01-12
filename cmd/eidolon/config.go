package main

import (
	"encoding/json"
	"fmt"
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

func (c *Config) ResolveCommand(cmdName string, args []string) (string, []string, error) {
	binPath, err := c.resolveBinaryPath(cmdName)
	if err != nil {
		return "", nil, err
	}

	newArgs := c.applyFlagReplacements(cmdName, args)
	return binPath, newArgs, nil
}

func (c *Config) resolveBinaryPath(cmdName string) (string, error) {
	// 1. Global binary override
	if c.Global.Binary != "" {
		return c.Global.Binary, nil
	}

	// 2. Command-specific binary override
	if cmdCfg, ok := c.Commands[cmdName]; ok && cmdCfg.Binary != "" {
		return cmdCfg.Binary, nil
	}

	// 3. Find real binary in PATH (excluding self)
	selfPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	selfDir := filepath.Dir(selfPath)

	pathEnv := os.Getenv("PATH")
	for _, p := range filepath.SplitList(pathEnv) {
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

func (c *Config) applyFlagReplacements(cmdName string, args []string) []string {
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
		for _, r := range replacements {
			if len(r.From) > 0 && i+len(r.From) <= len(args) {
				match := true
				for j := 0; j < len(r.From); j++ {
					if args[i+j] != r.From[j] {
						match = false
						break
					}
				}
				if match {
					newArgs = append(newArgs, r.To...)
					i += len(r.From)
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

func (c *Config) GetEnv(cmdName string) []string {
	res := os.Environ()

	// 2. Global environment variables
	for k, v := range c.Global.Env {
		res = append(res, k+"="+v)
	}

	// 3. Command-specific environment variables
	if cmdCfg, ok := c.Commands[cmdName]; ok {
		for k, v := range cmdCfg.Env {
			res = append(res, k+"="+v)
		}
	}

	return res
}
