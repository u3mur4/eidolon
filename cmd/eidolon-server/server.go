package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"sync"

	"github.com/u3mur4/eidolon/pkg/common/types"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// Server coordinates all components of the eidolon log server
type Server struct {
	Config     *Config
	Formatter  *LogFormatter
	ServerEnv  []string
	printMutex sync.Mutex
	running    map[int]*types.LogMessage // PID -> running notification
	lastOutput map[int]outputHash        // PID -> hash of last printed output
	outputFile *os.File
	errorFile  *os.File
}

type outputHash struct {
	stdin  string
	stdout string
	stderr string
}

// NewServer creates a new server instance with the given configuration
func NewServer(cfg *Config) *Server {
	colors := NewColors()
	formatter := NewLogFormatter(colors, cfg.Search, cfg.EnvMode, os.Environ())

	outputFile, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Warning: Could not open output file %s: %v", cfg.Output, err)
		outputFile = nil
	}

	errorFile, err := os.OpenFile(cfg.Error, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Warning: Could not open error file %s: %v", cfg.Error, err)
		errorFile = nil
	}

	return &Server{
		Config:     cfg,
		Formatter:  formatter,
		ServerEnv:  os.Environ(),
		running:    make(map[int]*types.LogMessage),
		lastOutput: make(map[int]outputHash),
		outputFile: outputFile,
		errorFile:  errorFile,
	}
}

// Run starts the server and listens for connections
func (s *Server) Run() error {
	listener, err := net.Listen("tcp", s.Config.Address)
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Printf("Eidolon server listening on %s", s.Config.Address)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		go handleConnection(conn, s.handleMessage)
	}
}

// handleMessage processes a single log message
func (s *Server) handleMessage(msg *types.LogMessage) {
	// Apply filters only for completed messages
	if msg.Status == "completed" {
		if s.Config.OnlyErrors && msg.ExitCode == 0 {
			return
		}

		if s.Config.Filter != "" {
			if bytes.Contains(msg.StdinData, []byte(s.Config.Filter)) ||
				bytes.Contains(msg.StdoutData, []byte(s.Config.Filter)) ||
				bytes.Contains(msg.StderrData, []byte(s.Config.Filter)) {
				return
			}
		}

		// Remove from running map when completed
		s.printMutex.Lock()
		delete(s.running, msg.PID)
		delete(s.lastOutput, msg.PID)
		s.printMutex.Unlock()
	} else if msg.Status == "running" {
		s.printMutex.Lock()
		currentHash := outputHash{
			stdin:  string(msg.StdinData),
			stdout: string(msg.StdoutData),
			stderr: string(msg.StderrData),
		}
		if last, ok := s.lastOutput[msg.PID]; ok && last == currentHash {
			s.printMutex.Unlock()
			return
		}
		s.running[msg.PID] = msg
		s.lastOutput[msg.PID] = currentHash
		s.printMutex.Unlock()
	}

	// Print with mutex to prevent interleaved output
	s.printMutex.Lock()
	defer s.printMutex.Unlock()

	output := s.Formatter.Format(msg)
	fmt.Print(output)

	// Write to files (strip ANSI)
	if s.outputFile != nil {
		s.outputFile.WriteString(stripANSI(output))
	}

	if s.errorFile != nil && msg.ExitCode != 0 {
		s.errorFile.WriteString(stripANSI(output))
	}
}
