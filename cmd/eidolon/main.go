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

	"gitea.morix.site/u3mur4/eidolon/pkg/common/types"
)

type SafeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *SafeBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *SafeBuffer) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.buf.Bytes()...)
}

func main() {
	// 1. Get command name and arguments
	executableName := filepath.Base(os.Args[0])
	args := os.Args[1:]

	// 2. Load configuration
	config, _ := loadConfig()

	// 3. Resolve the real binary and transform flags
	realPath, newArgs, err := config.ResolveCommand(executableName, args)
	if err != nil {
		log.Fatalf("eidolon: %v", err)
	}

	// 4. Set up the real command
	cmd := exec.Command(realPath, newArgs...)
	cmd.Env = config.GetEnv(executableName)

	// The buffered data
	var stdinBuf, stdoutBuf, stderrBuf SafeBuffer

	// Create pipes for I/O
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("eidolon: Failed to create stdin pipe: %v", err)
		os.Exit(1)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("eidolon: Failed to create stdout pipe: %v", err)
		os.Exit(1)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("eidolon: Failed to create stderr pipe: %v", err)
		os.Exit(1)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		log.Printf("eidolon: Failed to start real command %q: %v", realPath, err)
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
	err = cmd.Wait()
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
		PPID:       os.Getppid(),
		Command:    executableName,
		Args:       args,
		ExitCode:   exitCode,
		StdinData:  stdinBuf.Bytes(),
		StdoutData: stdoutBuf.Bytes(),
		StderrData: stderrBuf.Bytes(),
	}

	// Send the message to the log server
	sendToServer(config.Server, msg)

	// Finally, exit with the same code as the real command
	os.Exit(exitCode)
}

func sendToServer(addr string, msg types.LogMessage) {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		log.Printf("eidolon: Failed to connect to log server at %s: %v", addr, err)
		return
	}
	defer conn.Close()

	encoder := gob.NewEncoder(conn)
	if err := encoder.Encode(&msg); err != nil {
		log.Printf("eidolon: Failed to send log message: %v", err)
	}
}
