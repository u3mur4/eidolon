package types

import "time"

// LogMessage is the data structure sent from the proxy to the server.
type LogMessage struct {
	Timestamp  time.Time
	PID        int
	PPID       int
	Alias      string
	Path       string
	Args       []string
	Env        []string
	ExitCode   int
	StdinData  []byte
	StdoutData []byte
	StderrData []byte
	Status     string // "running" or "completed"
}
