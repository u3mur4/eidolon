package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type CommandConfig struct {
	Binary string            `json:"binary"`
	Env    map[string]string `json:"env"`
}

type Config struct {
	Server   string                   `json:"server"`
	Global   CommandConfig            `json:"-"`
	Commands map[string]CommandConfig `json:"-"`
}

func (c *Config) UnmarshalJSON(data []byte) error {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}

	c.Commands = make(map[string]CommandConfig)
	c.Global.Env = make(map[string]string)

	for k, v := range all {
		switch k {
		case "server":
			if err := json.Unmarshal(v, &c.Server); err != nil {
				return err
			}
		case "binary":
			if err := json.Unmarshal(v, &c.Global.Binary); err != nil {
				return err
			}
		case "env":
			if err := json.Unmarshal(v, &c.Global.Env); err != nil {
				return err
			}
		default:
			var cmdCfg CommandConfig
			if err := json.Unmarshal(v, &cmdCfg); err == nil {
				c.Commands[k] = cmdCfg
			}
		}
	}
	return nil
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

func (c *Config) ResolveBinary(cmdName string) (string, error) {
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
