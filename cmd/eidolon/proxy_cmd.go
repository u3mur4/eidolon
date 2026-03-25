package main

import (
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

type IOBuffer struct {
	mu      sync.Mutex
	entries []types.IOEntry
}

func (s *IOBuffer) WriteEntry(source types.IOSource, data []byte) {
	if len(data) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.entries) > 0 && s.entries[len(s.entries)-1].Source == source {
		s.entries[len(s.entries)-1].Data = append(s.entries[len(s.entries)-1].Data, data...)
	} else {
		s.entries = append(s.entries, types.IOEntry{
			Source: source,
			Data:   append([]byte(nil), data...),
		})
	}
}

func (s *IOBuffer) Entries() []types.IOEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]types.IOEntry(nil), s.entries...)
}

type writeFunc func(p []byte)

func (f writeFunc) Write(p []byte) (n int, err error) {
	f(p)
	return len(p), nil
}

type ProxyCmd struct {
	ServerAddr string
	Context    *CommandContext
	CmdName    string // Original name for logging
	Cmd        *exec.Cmd
	StartTime  time.Time
}

func (p *ProxyCmd) Run() int {
	cmd, err := p.Context.Command()
	if err != nil {
		log.Printf("eidolon: %v", err)
		return 1
	}

	p.Cmd = cmd
	// The ordered buffered data
	var ioBuffer IOBuffer

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
	startTime := time.Now()
	p.StartTime = startTime

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
		stdinWriter := io.MultiWriter(stdinPipe, writeFunc(func(p []byte) {
			ioBuffer.WriteEntry(types.IOSourceStdin, p)
		}))
		io.Copy(stdinWriter, os.Stdin)
	}()

	// Goroutine 2: Handle stdout
	go func() {
		defer wg.Done()
		stdoutWriter := io.MultiWriter(os.Stdout, writeFunc(func(p []byte) {
			ioBuffer.WriteEntry(types.IOSourceStdout, p)
		}))
		io.Copy(stdoutWriter, stdoutPipe)
	}()

	// Goroutine 3: Handle stderr
	go func() {
		defer wg.Done()
		stderrWriter := io.MultiWriter(os.Stderr, writeFunc(func(p []byte) {
			ioBuffer.WriteEntry(types.IOSourceStderr, p)
		}))
		io.Copy(stderrWriter, stderrPipe)
	}()

	updateTicker := time.NewTicker(1 * time.Second)
	updateSent := make(chan struct{})
	stopped := false
	sentFirstUpdate := false

	go func() {
		for {
			select {
			case <-updateTicker.C:
				if stopped {
					return
				}
				if !sentFirstUpdate && time.Since(startTime) < 1*time.Second {
					continue
				}
				sentFirstUpdate = true
				p.sendRunningUpdate(&ioBuffer, cmd.Env)
			case <-updateSent:
				return
			}
		}
	}()

	wg.Wait()
	updateTicker.Stop()
	stopped = true
	close(updateSent)

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
		StartTime: startTime,
		ExitTime:  time.Now(),
		PID:       os.Getpid(),
		PPID:      os.Getppid(),
		Alias:     p.CmdName,
		Path:      p.Context.Path,
		Args:      p.Context.Args,
		Env:       cmd.Env,
		ExitCode:  exitCode,
		IOData:    ioBuffer.Entries(),
		Status:    "completed",
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

func (p *ProxyCmd) sendRunningUpdate(ioBuffer *IOBuffer, env []string) {
	msg := types.LogMessage{
		StartTime: p.StartTime,
		ExitTime:  time.Time{}, // zero time for running
		PID:       os.Getpid(),
		PPID:      os.Getppid(),
		Alias:     p.CmdName,
		Path:      p.Context.Path,
		Args:      p.Context.Args,
		Env:       env,
		IOData:    ioBuffer.Entries(),
		Status:    "running",
	}
	p.sendToServer(p.ServerAddr, msg)
}

func (p *ProxyCmd) SendInterrupted() {
	if p.Cmd != nil {
		p.Cmd.Process.Kill()
	}
}
