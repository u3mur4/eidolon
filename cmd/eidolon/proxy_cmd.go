package main

import (
	"bytes"
	"encoding/gob"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/u3mur4/eidolon/pkg/common/types"
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

type ProxyCmd struct {
	ServerAddr string
	Context    *CommandContext
	CmdName    string // Original name for logging
}

func (p *ProxyCmd) Run() int {
	cmd, err := p.Context.Command()
	if err != nil {
		log.Printf("eidolon: %v", err)
		return 1
	}

	// The buffered data
	var stdinBuf, stdoutBuf, stderrBuf SafeBuffer

	// Create pipes for I/O
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("eidolon: Failed to create stdin pipe: %v", err)
		return 1
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("eidolon: Failed to create stdout pipe: %v", err)
		return 1
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("eidolon: Failed to create stderr pipe: %v", err)
		return 1
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		log.Printf("eidolon: Failed to start real command %q: %v", cmd.Path, err)
		return 127
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
		Alias:      p.CmdName,
		Path:       p.Context.Path,
		Args:       p.Context.Args,
		Env:        cmd.Env,
		ExitCode:   exitCode,
		StdinData:  stdinBuf.Bytes(),
		StdoutData: stdoutBuf.Bytes(),
		StderrData: stderrBuf.Bytes(),
	}

	// Send the message to the log server
	p.sendToServer(p.ServerAddr, msg)

	return exitCode
}

func (p *ProxyCmd) sendToServer(addr string, msg types.LogMessage) {
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
