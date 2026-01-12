package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type CommandContext struct {
	Path string
	Args []string
	Env  []string
}

func NewCommandContext(name string, args []string) *CommandContext {
	ctx := &CommandContext{
		Args: args,
		Env:  os.Environ(),
	}

	// Initial resolution from PATH, skipping self
	selfPath, err := os.Executable()
	if err == nil {
		selfDir := filepath.Dir(selfPath)
		pathEnv := os.Getenv("PATH")
		for _, p := range filepath.SplitList(pathEnv) {
			if filepath.Clean(p) == filepath.Clean(selfDir) {
				continue
			}

			fullPath := filepath.Join(p, name)
			info, err := os.Stat(fullPath)
			if err == nil && !info.IsDir() && (info.Mode()&0111 != 0) {
				ctx.Path = fullPath
				break
			}
		}
	}

	return ctx
}

func (r *CommandContext) OverrideCmd(path string) {
	if path != "" {
		r.Path = path
	}
}

func (r *CommandContext) OverrideArgs(args []string) {
	r.Args = args
}

func (r *CommandContext) AddEnv(env map[string]string) {
	for k, v := range env {
		r.Env = append(r.Env, k+"="+v)
	}
}

func (r *CommandContext) Command() (*exec.Cmd, error) {
	if r.Path == "" {
		return nil, fmt.Errorf("command path not resolved")
	}

	cmd := exec.Command(r.Path, r.Args...)
	cmd.Env = r.Env
	return cmd, nil
}
