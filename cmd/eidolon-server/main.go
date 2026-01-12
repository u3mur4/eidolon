package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"unicode"

	"gitea.morix.site/u3mur4/eidolon/pkg/common/types"
	"github.com/fatih/color"
)

// Global mutex to prevent interleaved printing from concurrent connections
var printMutex = &sync.Mutex{}

// Global state for interactive commands
var (
	searchText     string
	filterText     string
	onlyNonZero    bool
	highlightColor = color.New(color.BgYellow, color.FgBlack)
)

// Color definitions
var (
	headerColor      = color.New(color.FgCyan, color.Bold)
	cmdColor         = color.New(color.FgYellow)
	stdinColor       = color.New(color.FgGreen)
	stdoutColor      = color.New(color.FgWhite)
	stderrColor      = color.New(color.FgRed)
	hexColor         = color.New(color.Faint)
	exitCodeColor    = color.New(color.BgRed, color.FgYellow, color.Bold)
	flagColor        = color.New(color.FgHiCyan)
	flagValColor     = color.New(color.FgCyan)
	argColor         = color.New(color.FgHiGreen)
	specialCharColor = color.New(color.FgHiMagenta, color.Bold)
)

func main() {
	addr := "0.0.0.0:9999"
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()
	log.Printf("Eidolon server listening on %s", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		// Handle each connection in a new goroutine
		go handleConnection(conn)
	}
}

func init() {
	go handleInput()
}

func handleInput() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "search:") {
			printMutex.Lock()
			searchText = strings.TrimSpace(strings.TrimPrefix(line, "search:"))
			fmt.Printf("Search text set to: %q\n", searchText)
			printMutex.Unlock()
		} else if strings.HasPrefix(line, "filter:") {
			printMutex.Lock()
			filterText = strings.TrimSpace(strings.TrimPrefix(line, "filter:"))
			fmt.Printf("Filter text set to: %q (commands containing this in stdin/out/err will be skipped)\n", filterText)
			printMutex.Unlock()
		} else if line == "clear" {
			printMutex.Lock()
			fmt.Print("\033[H\033[2J") // Clear terminal screen
			printMutex.Unlock()
		} else if line == "exit code" {
			printMutex.Lock()
			onlyNonZero = !onlyNonZero
			fmt.Printf("Only non-zero exit codes: %v\n", onlyNonZero)
			printMutex.Unlock()
		} else if line != "" {
			fmt.Printf("Unknown command: %s (Available: search: <text>, filter: <text>, clear, exit code)\n", line)
		}
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	decoder := gob.NewDecoder(conn)

	for {
		var msg types.LogMessage
		if err := decoder.Decode(&msg); err != nil {
			if err != io.EOF {
				log.Printf("Error decoding message: %v", err)
			}
			break
		}
		printFormattedLog(&msg)
	}
}

func printableCount(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	printable := 0
	for _, b := range data {
		if b >= 32 && b < 127 || unicode.IsSpace(rune(b)) {
			printable++
		}
	}
	return printable
}

// isPrintable checks if a byte slice contains primarily printable characters.
func isPrintable(data []byte) bool {
	printable := printableCount(data)
	// Consider it printable if >90% of characters are printable
	return float64(printable)/float64(len(data)) > 0.9
}

// printData prints data as a string or a hex dump based on its content.
// Non-printable characters in "printable" data are escaped and colorized.
func printData(c *color.Color, title string, data []byte) {
	if len(data) == 0 {
		return
	}
	c.Printf("%s (%d bytes)\n", title, len(data))
	if isPrintable(data) {
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			output := escapeBytes([]byte(line))
			if searchText != "" {
				output = highlightSearch(output)
			}
			c.Print(output)

			if i < len(lines)-1 {
				fmt.Println()
			}
		}
		fmt.Println()
	} else {
		hexColor.Println(hex.Dump(data))
	}
}

// escapeBytes returns a string where non-printable characters are escaped and colorized.
func escapeBytes(data []byte) string {
	var processed bytes.Buffer
	for _, b := range data {
		if b >= 32 && b < 127 || unicode.IsSpace(rune(b)) {
			processed.WriteByte(b)
		} else {
			processed.WriteString(specialCharColor.Sprintf("\\x%02x", b))
		}
	}
	return processed.String()
}

func highlightSearch(text string) string {
	if searchText == "" {
		return text
	}

	parts := strings.Split(text, searchText)
	if len(parts) == 1 {
		return text
	}

	var result strings.Builder
	for i, part := range parts {
		result.WriteString(part)
		if i < len(parts)-1 {
			result.WriteString(highlightColor.Sprint(searchText))
		}
	}
	return result.String()
}

// formatArgsForDisplay takes a slice of string arguments and formats them for display with colors.
func formatArgsForDisplay(args []string) string {
	formattedArgs := make([]string, len(args))
	prevWasFlag := false

	for i, arg := range args {
		// Apply coloring based on the argument structure and context
		if strings.HasPrefix(arg, "-") {
			if idx := strings.Index(arg, "="); idx != -1 {
				// flag=value
				flagPart := arg[:idx]
				equalPart := "="
				valPart := arg[idx+1:]
				formattedArgs[i] = flagColor.Sprint(escapeBytes([]byte(flagPart))) +
					stdoutColor.Sprint(equalPart) +
					flagValColor.Sprint(escapeBytes([]byte(valPart)))
				prevWasFlag = false // It's self-contained
			} else {
				// simple flag
				formattedArgs[i] = flagColor.Sprint(escapeBytes([]byte(arg)))
				prevWasFlag = true
			}
		} else {
			// not a flag
			display := escapeBytes([]byte(arg))
			if prevWasFlag {
				// likely a value for the previous flag
				formattedArgs[i] = argColor.Sprint(display)
			} else {
				// positional argument or subcommand
				formattedArgs[i] = cmdColor.Sprint(display)
			}
			prevWasFlag = false
		}
	}

	return strings.Join(formattedArgs, " ")
}

func printFormattedLog(msg *types.LogMessage) {
	if onlyNonZero && msg.ExitCode == 0 {
		return
	}
	if filterText != "" {
		if bytes.Contains(msg.StdinData, []byte(filterText)) ||
			bytes.Contains(msg.StdoutData, []byte(filterText)) ||
			bytes.Contains(msg.StderrData, []byte(filterText)) {
			return
		}
	}

	printMutex.Lock()
	defer printMutex.Unlock()

	// Header
	hColor := headerColor
	if msg.ExitCode != 0 {
		hColor = exitCodeColor
	}
	headerText := fmt.Sprintf("PID: %d |PPID: %d |CMD: %s |EXIT: %d |TIME: %s\n", msg.PID, msg.PPID, msg.Command, msg.ExitCode, msg.Timestamp.Format("15:04:05.000"))
	if searchText != "" {
		hColor.Print(highlightSearch(headerText))
	} else {
		hColor.Print(headerText)
	}

	// Arguments
	displayArgs := formatArgsForDisplay(msg.Args)
	cmdText := fmt.Sprintf("%s %s\n", msg.Command, displayArgs)
	if searchText != "" {
		// Note: displayArgs already has internal colorization,
		// but highlightSearch will add more colors on top if it matches.
		// Color packages usually handle nested ANSI codes okay, but let's be careful.
		fmt.Print(highlightSearch(cmdText))
	} else {
		cmdColor.Print(cmdText)
	}

	// Stdin, Stdout, Stderr
	printData(stdinColor, "STDIN", msg.StdinData)
	printData(stdoutColor, "STDOUT", msg.StdoutData)
	printData(stderrColor, "STDERR", msg.StderrData)

	headerColor.Print(strings.Repeat("-", 80))
	fmt.Println()
}
