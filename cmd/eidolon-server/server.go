package main

import (
	"bytes"
	"log"
	"net"
	"sync"

	"gitea.morix.site/u3mur4/eidolon/pkg/common/types"
)

// Server coordinates all components of the eidolon log server
type Server struct {
	Config     *Config
	Formatter  *LogFormatter
	printMutex sync.Mutex
}

// NewServer creates a new server instance with the given configuration
func NewServer(cfg *Config) *Server {
	colors := NewColors()
	formatter := NewLogFormatter(colors, cfg.Search)

	return &Server{
		Config:    cfg,
		Formatter: formatter,
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
	// Apply filters
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

	// Print with mutex to prevent interleaved output
	s.printMutex.Lock()
	defer s.printMutex.Unlock()

	s.Formatter.PrintLog(msg)
}
