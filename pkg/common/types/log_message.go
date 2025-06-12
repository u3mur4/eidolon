package types

import "time"

// LogMessage is the data structure sent from the proxy to the server.
type LogMessage struct {
	Timestamp  time.Time
	PID        int
	Command    string
	Args       []string
	ExitCode   int
	StdinData  []byte
	StdoutData []byte
	StderrData []byte
}
