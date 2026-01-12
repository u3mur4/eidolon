package main

import (
	"os"
	"path/filepath"
)

func main() {
	// 1. Get command name and arguments
	executableName := filepath.Base(os.Args[0])
	args := os.Args[1:]

	// 2. Load configuration
	config, _ := loadConfig()

	// 3. Run the proxy command
	context := &CommandContext{Config: config, CmdName: executableName, Args: args}
	proxy := &ProxyCmd{Config: config, Context: context}

	exitStatus := proxy.Run()

	// 4. Finally, exit with the same code as the real command
	os.Exit(exitStatus)
}
