package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"gitea.morix.site/u3mur4/eidolon/pkg/common/types"
	"github.com/fatih/color"
)

// LogFormatter handles all output formatting for log messages
type LogFormatter struct {
	Colors     *Colors
	SearchText string
}

// NewLogFormatter creates a new formatter with the given colors and search text
func NewLogFormatter(colors *Colors, searchText string) *LogFormatter {
	return &LogFormatter{
		Colors:     colors,
		SearchText: searchText,
	}
}

// PrintLog formats and prints a complete log message
func (f *LogFormatter) PrintLog(msg *types.LogMessage) {
	// Header
	hColor := f.Colors.Header
	if msg.ExitCode != 0 {
		hColor = f.Colors.ExitCode
	}

	cmdName := msg.Alias
	if msg.Path != "" && msg.Alias != filepath.Base(msg.Path) {
		cmdName = fmt.Sprintf("%s -> %s", msg.Alias, msg.Path)
	}

	headerText := fmt.Sprintf("PID: %d |PPID: %d |CMD: %s |EXIT: %d |TIME: %s\n",
		msg.PID, msg.PPID, cmdName, msg.ExitCode, msg.Timestamp.Format("15:04:05.000"))

	if f.SearchText != "" {
		hColor.Print(f.highlightSearch(headerText))
	} else {
		hColor.Print(headerText)
	}

	// Arguments
	displayArgs := f.formatArgsForDisplay(msg.Args)
	cmdToDisplay := msg.Alias
	if msg.Path != "" && msg.Alias != filepath.Base(msg.Path) {
		cmdToDisplay = msg.Path
	}

	cmdText := fmt.Sprintf("%s %s\n", cmdToDisplay, displayArgs)
	if f.SearchText != "" {
		fmt.Print(f.highlightSearch(cmdText))
	} else {
		f.Colors.Cmd.Print(cmdText)
	}

	// Stdin, Stdout, Stderr
	f.printData(f.Colors.Stdin, "STDIN", msg.StdinData)
	f.printData(f.Colors.Stdout, "STDOUT", msg.StdoutData)
	f.printData(f.Colors.Stderr, "STDERR", msg.StderrData)

	f.Colors.Header.Print(strings.Repeat("-", 80))
	fmt.Println()
}

// printData prints data as a string or a hex dump based on its content
func (f *LogFormatter) printData(c *color.Color, title string, data []byte) {
	if len(data) == 0 {
		return
	}

	c.Printf("%s (%d bytes)\n", title, len(data))

	if f.isPrintable(data) {
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			output := f.escapeBytes([]byte(line))
			if f.SearchText != "" {
				output = f.highlightSearch(output)
			}
			c.Print(output)

			if i < len(lines)-1 {
				fmt.Println()
			}
		}
		fmt.Println()
	} else {
		f.Colors.Hex.Println(hex.Dump(data))
	}
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

// highlightSearch highlights matching search text
func (f *LogFormatter) highlightSearch(text string) string {
	if f.SearchText == "" {
		return text
	}

	parts := strings.Split(text, f.SearchText)
	if len(parts) == 1 {
		return text
	}

	var result strings.Builder
	for i, part := range parts {
		result.WriteString(part)
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
