package types

import "time"

type IOSource string

const (
	IOSourceStdin  IOSource = "stdin"
	IOSourceStdout IOSource = "stdout"
	IOSourceStderr IOSource = "stderr"
)

type IOEntry struct {
	Source IOSource
	Data   []byte
}

type LogMessage struct {
	StartTime time.Time
	ExitTime  time.Time
	PID       int
	PPID      int
	Alias     string
	Path      string
	Args      []string
	Env       []string
	ExitCode  int
	IOData    []IOEntry
	Status    string // "running" or "completed"
}
