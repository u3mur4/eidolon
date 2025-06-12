package main

import (
	"bytes"
	"encoding/gob"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"gitea.morix.site/u3mur4/git-proxy/pkg/common/types"
)

// Configuration
const realGitPath = "/bin/git"
const logServerAddr = "127.0.0.1:9999" // Assuming server runs on the same machine

func main() {
	// The buffered data
	var stdinBuf, stdoutBuf, stderrBuf bytes.Buffer

	// Get command name and arguments
	cmdName := filepath.Base(os.Args[0])
	args := os.Args[1:]

	// Set up the real git command
	cmd := exec.Command(realGitPath, args...)

	// Create pipes for I/O
	stdinPipe, _ := cmd.StdinPipe()
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	// Start the command
	if err := cmd.Start(); err != nil {
		log.Printf("git-proxy: Failed to start real git command: %v", err)
		os.Exit(127)
	}

	// Use a WaitGroup to manage concurrent I/O redirection and buffering
	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Handle stdin
	go func() {
		defer stdinPipe.Close()
		// Copy from our real stdin to both the command's stdin and our buffer
		io.Copy(io.MultiWriter(stdinPipe, &stdinBuf), os.Stdin)
	}()

	// Goroutine 2: Handle stdout
	go func() {
		defer wg.Done()
		// Copy from the command's stdout to both our real stdout and our buffer
		io.Copy(io.MultiWriter(os.Stdout, &stdoutBuf), stdoutPipe)
	}()

	// Goroutine 3: Handle stderr
	go func() {
		defer wg.Done()
		// Copy from the command's stderr to both our real stderr and our buffer
		io.Copy(io.MultiWriter(os.Stderr, &stderrBuf), stderrPipe)
	}()

	// Wait for all I/O to complete
	wg.Wait()

	// Wait for the command to exit and capture its status
	err := cmd.Wait()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		} else {
			exitCode = 1 // Default error code for non-exit-status errors
		}
	}

	// Assemble the log message
	msg := types.LogMessage{
		Timestamp:  time.Now(),
		PID:        os.Getpid(),
		Command:    cmdName,
		Args:       args,
		ExitCode:   exitCode,
		StdinData:  stdinBuf.Bytes(),
		StdoutData: stdoutBuf.Bytes(),
		StderrData: stderrBuf.Bytes(),
	}

	// Send the message to the log server
	sendToServer(msg)

	// Finally, exit with the same code as the real git command
	os.Exit(exitCode)
}

func sendToServer(msg types.LogMessage) {
	conn, err := net.DialTimeout("tcp", logServerAddr, 2*time.Second)
	if err != nil {
		log.Printf("git-proxy: Failed to connect to log server: %v", err)
		return
	}
	defer conn.Close()

	encoder := gob.NewEncoder(conn)
	if err := encoder.Encode(&msg); err != nil {
		log.Printf("git-proxy: Failed to send log message: %v", err)
	}
}
