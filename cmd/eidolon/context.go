package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type CommandContext struct {
	Config  Config
	CmdName string
	Args    []string
}

func (r *CommandContext) Command() (*exec.Cmd, error) {
	binPath, newArgs, err := r.ResolveCommand()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(binPath, newArgs...)
	cmd.Env = r.GetEnv()
	return cmd, nil
}

func (r *CommandContext) ResolveCommand() (string, []string, error) {
	binPath, err := r.ResolveBinaryPath()
	if err != nil {
		return "", nil, err
	}

	newArgs := r.applyFlagReplacements()
	return binPath, newArgs, nil
}

func (r *CommandContext) ResolveBinaryPath() (string, error) {
	// 1. Global binary override
	if r.Config.Global.Binary != "" {
		return r.Config.Global.Binary, nil
	}

	// 2. Command-specific binary override
	if cmdCfg, ok := r.Config.Commands[r.CmdName]; ok && cmdCfg.Binary != "" {
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

		fullPath := filepath.Join(p, r.CmdName)
		info, err := os.Stat(fullPath)
		if err == nil && !info.IsDir() && (info.Mode()&0111 != 0) {
			return fullPath, nil
		}
	}

	return "", fmt.Errorf("binary %q not found in PATH (excluding %s)", r.CmdName, selfDir)
}

func (r *CommandContext) applyFlagReplacements() []string {
	replacements := append([]FlagReplacement{}, r.Config.Global.Flags...)
	if cmdCfg, ok := r.Config.Commands[r.CmdName]; ok {
		replacements = append(replacements, cmdCfg.Flags...)
	}

	if len(replacements) == 0 {
		return r.Args
	}

	newArgs := []string{}
	for i := 0; i < len(r.Args); {
		matched := false
		for _, repl := range replacements {
			if len(repl.From) > 0 && i+len(repl.From) <= len(r.Args) {
				match := true
				for j := 0; j < len(repl.From); j++ {
					if r.Args[i+j] != repl.From[j] {
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
			newArgs = append(newArgs, r.Args[i])
			i++
		}
	}

	return newArgs
}

func (r *CommandContext) GetEnv() []string {
	res := os.Environ()

	// 2. Global environment variables
	for k, v := range r.Config.Global.Env {
		res = append(res, k+"="+v)
	}

	// 3. Command-specific environment variables
	if cmdCfg, ok := r.Config.Commands[r.CmdName]; ok {
		for k, v := range cmdCfg.Env {
			res = append(res, k+"="+v)
		}
	}

	return res
}
