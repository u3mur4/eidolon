package main

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

var interrupted bool

func main() {
	// 1. Get command name and arguments
	executableName := filepath.Base(os.Args[0])
	args := os.Args[1:]

	// 2. Load configuration
	config, _ := loadConfig()

	// 3. Create context and apply overrides from config
	context := NewCommandContext(executableName, args)

	// Apply binary override
	context.OverrideCmd(config.GetBinary(executableName))

	// Apply environment overrides
	context.AddEnv(config.GetEnv(executableName))

	// Apply flag replacements
	context.OverrideArgs(config.ApplyFlags(executableName, args))

	// 4. Run the proxy command
	proxy := &ProxyCmd{
		ServerAddr: config.Server,
		Context:    context,
		CmdName:    executableName,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		interrupted = true
		proxy.SendInterrupted()
	}()

	exitStatus := proxy.Run()

	if interrupted {
		exitStatus = 1
	}

	// 5. Finally, exit with the same code as the real command
	os.Exit(exitStatus)
}
