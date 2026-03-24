package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/fatih/color"
	"github.com/u3mur4/eidolon/pkg/common/types"
)

// LogFormatter handles all output formatting for log messages
type LogFormatter struct {
	Colors     *Colors
	SearchText string
	EnvMode    string
	ServerEnv  []string
}

// NewLogFormatter creates a new formatter with the given colors and search text
func NewLogFormatter(colors *Colors, searchText, envMode string, serverEnv []string) *LogFormatter {
	return &LogFormatter{
		Colors:     colors,
		SearchText: searchText,
		EnvMode:    envMode,
		ServerEnv:  serverEnv,
	}
}

// PrintLog formats and prints a complete log message
func (f *LogFormatter) Format(msg *types.LogMessage) string {
	var sb strings.Builder

	// Header
	hColor := f.Colors.Header
	if msg.ExitCode != 0 {
		hColor = f.Colors.ExitCode
	}

	cmdName := msg.Alias
	if msg.Path != "" && msg.Alias != filepath.Base(msg.Path) {
		cmdName = fmt.Sprintf("%s -> %s", msg.Alias, msg.Path)
	}

	statusText := "running"
	if msg.Status == "completed" {
		statusText = fmt.Sprintf("exit(%d)", msg.ExitCode)
	}

	startTime := msg.StartTime.Format("15:04:05.000")
	var elapsed string
	if msg.ExitTime.IsZero() {
		elapsed = f.formatDuration(time.Since(msg.StartTime))
	} else {
		elapsed = f.formatDuration(msg.ExitTime.Sub(msg.StartTime))
	}

	headerText := fmt.Sprintf("PID: %d |PPID: %d |CMD: %s |STATUS: %s |START: %s |ELAPSED: %s\n",
		msg.PID, msg.PPID, cmdName, statusText, startTime, elapsed)

	if f.SearchText != "" {
		sb.WriteString(f.highlightSearch(headerText, hColor))
	} else {
		sb.WriteString(hColor.Sprint(headerText))
	}

	sb.WriteString(f.formatEnv(msg.Env))

	// Arguments
	displayArgs := f.formatArgsForDisplay(msg.Args)
	cmdToDisplay := msg.Alias
	if msg.Path != "" && msg.Alias != filepath.Base(msg.Path) {
		cmdToDisplay = msg.Path
	}

	cmdText := fmt.Sprintf("%s %s\n", cmdToDisplay, displayArgs)
	if f.SearchText != "" {
		sb.WriteString(f.highlightSearch(cmdText, f.Colors.Cmd))
	} else {
		sb.WriteString(f.Colors.Cmd.Sprint(cmdText))
	}

	// Stdin, Stdout, Stderr
	sb.WriteString(f.formatData(f.Colors.Stdin, "STDIN", msg.StdinData))
	sb.WriteString(f.formatData(f.Colors.Stdout, "STDOUT", msg.StdoutData))
	sb.WriteString(f.formatData(f.Colors.Stderr, "STDERR", msg.StderrData))

	sb.WriteString(f.Colors.Header.Sprint(strings.Repeat("-", 80)))
	sb.WriteString("\n")

	return sb.String()
}

func (f *LogFormatter) formatData(c *color.Color, title string, data []byte) string {
	if len(data) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(c.Sprintf("%s (%d bytes)\n", title, len(data)))

	if f.isPrintable(data) {
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			output := f.escapeBytes([]byte(line))
			if f.SearchText != "" {
				sb.WriteString(f.highlightSearch(output, c))
			} else {
				sb.WriteString(c.Sprint(output))
			}

			if i < len(lines)-1 {
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	} else {
		if f.SearchText != "" {
			sb.WriteString(f.highlightSearch(hex.Dump(data), c))
		} else {
			sb.WriteString(c.Sprint(hex.Dump(data)))
		}
	}

	return sb.String()
}

// formatArgsForDisplay formats command arguments with colorization
func (f *LogFormatter) formatArgsForDisplay(args []string) string {
	formattedArgs := make([]string, len(args))
	prevWasFlag := false

	for i, arg := range args {
		if strings.HasPrefix(arg, "-") {
			if idx := strings.Index(arg, "="); idx != -1 {
				// flag=value
				flagPart := arg[:idx]
				valPart := arg[idx+1:]
				formattedArgs[i] = f.Colors.Flag.Sprint(f.escapeBytes([]byte(flagPart))) +
					f.Colors.Stdout.Sprint("=") +
					f.Colors.FlagVal.Sprint(f.escapeBytes([]byte(valPart)))
				prevWasFlag = false
			} else {
				// simple flag
				formattedArgs[i] = f.Colors.Flag.Sprint(f.escapeBytes([]byte(arg)))
				prevWasFlag = true
			}
		} else {
			// not a flag
			display := f.escapeBytes([]byte(arg))
			if prevWasFlag {
				formattedArgs[i] = f.Colors.Arg.Sprint(display)
			} else {
				formattedArgs[i] = f.Colors.Cmd.Sprint(display)
			}
			prevWasFlag = false
		}
	}

	return strings.Join(formattedArgs, " ")
}

// escapeBytes returns a string with non-printable characters escaped and colorized
func (f *LogFormatter) escapeBytes(data []byte) string {
	var processed bytes.Buffer
	for _, b := range data {
		if b >= 32 && b < 127 || unicode.IsSpace(rune(b)) {
			processed.WriteByte(b)
		} else {
			processed.WriteString(f.Colors.SpecialChar.Sprintf("\\x%02x", b))
		}
	}
	return processed.String()
}

// printEnv prints environment variables one line above the command
func (f *LogFormatter) formatEnv(env []string) string {
	if f.EnvMode == "none" || len(env) == 0 {
		return ""
	}

	envToPrint := env

	if f.EnvMode == "diff" {
		serverEnvSet := make(map[string]bool)
		for _, e := range f.ServerEnv {
			if idx := strings.Index(e, "="); idx != -1 {
				serverEnvSet[e[:idx]] = true
			}
		}

		var filtered []string
		for _, e := range env {
			if idx := strings.Index(e, "="); idx != -1 {
				key := e[:idx]
				if !serverEnvSet[key] {
					filtered = append(filtered, e)
				}
			}
		}
		envToPrint = filtered
	}

	if len(envToPrint) == 0 {
		return ""
	}

	envText := strings.Join(envToPrint, " ")
	if f.SearchText != "" {
		envText = f.highlightSearch(envText, f.Colors.Env)
	} else {
		envText = f.Colors.Env.Sprint(envText)
	}

	return envText + "\n"
}

// highlightSearch highlights matching search text with optional base color.
// If baseColor is nil, no base color is applied to non-matching parts.
// If search text not found, returns text with just base color applied.
func (f *LogFormatter) highlightSearch(text string, baseColor *color.Color) string {
	if f.SearchText == "" {
		if baseColor != nil {
			return baseColor.Sprint(text)
		}
		return text
	}

	parts := strings.Split(text, f.SearchText)
	if len(parts) == 1 {
		// Search text not found, just apply base color
		if baseColor != nil {
			return baseColor.Sprint(text)
		}
		return text
	}

	var result strings.Builder
	for i, part := range parts {
		if baseColor != nil {
			result.WriteString(baseColor.Sprint(part))
		} else {
			result.WriteString(part)
		}
		if i < len(parts)-1 {
			result.WriteString(f.Colors.Highlight.Sprint(f.SearchText))
		}
	}
	return result.String()
}

// isPrintable checks if data contains primarily printable characters
func (f *LogFormatter) isPrintable(data []byte) bool {
	if len(data) == 0 {
		return true
	}

	printable := 0
	for _, b := range data {
		if b >= 32 && b < 127 || unicode.IsSpace(rune(b)) {
			printable++
		}
	}
	return float64(printable)/float64(len(data)) > 0.9
}

func (f *LogFormatter) formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		// < 1ms: show microseconds with 1 decimal
		return fmt.Sprintf("%.1fμs", float64(d.Microseconds()))
	} else if d < time.Second {
		// 1ms - 1s: show milliseconds with 1 decimal
		return fmt.Sprintf("%.1fms", d.Seconds()*1000)
	} else if d < 60*time.Second {
		// 1s - 60s: show seconds with 1 decimal
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else {
		// >= 60s: show minutes and seconds
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if mins > 0 && secs > 0 {
			return fmt.Sprintf("%dm%ds", mins, secs)
		} else if mins > 0 {
			return fmt.Sprintf("%dm", mins)
		} else {
			return fmt.Sprintf("%ds", secs)
		}
	}
}
