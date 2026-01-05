package main

import (
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"unicode"

	"gitea.morix.site/u3mur4/git-proxy/pkg/common/types"
	"github.com/fatih/color"
)

// Global mutex to prevent interleaved printing from concurrent connections
var printMutex = &sync.Mutex{}

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
	log.Printf("Log server listening on %s", addr)

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
		for _, b := range data {
			if b >= 32 && b < 127 || unicode.IsSpace(rune(b)) {
				c.Print(string(b))
			} else {
				specialCharColor.Printf("\\x%02x", b)
			}
		}
		fmt.Println()
	} else {
		hexColor.Println(hex.Dump(data))
	}
}

// formatArgsForDisplay takes a slice of string arguments and formats them for display with colors.
func formatArgsForDisplay(args []string) string {
	formattedArgs := make([]string, len(args))

	for i, arg := range args {
		display := arg
		// An empty string has 0 printable characters, but should be displayed as is.
		// We only want to format non-empty strings that are fully non-printable.
		if len(arg) > 0 && printableCount([]byte(arg)) == 0 {
			var hexBuilder strings.Builder
			for _, b := range []byte(arg) {
				hexBuilder.WriteString(fmt.Sprintf("\\x%02x", b))
			}
			display = fmt.Sprintf("\"%s\"", hexBuilder.String())
		}

		// Apply coloring based on the argument structure
		if strings.HasPrefix(arg, "-") {
			if idx := strings.Index(arg, "="); idx != -1 {
				// flag=value
				flagPart := arg[:idx]
				equalPart := "="
				valPart := arg[idx+1:]
				formattedArgs[i] = flagColor.Sprint(flagPart) +
					stdoutColor.Sprint(equalPart) +
					flagValColor.Sprint(valPart)
			} else {
				// simple flag
				formattedArgs[i] = flagColor.Sprint(display)
			}
		} else {
			// regular argument
			formattedArgs[i] = argColor.Sprint(display)
		}
	}

	return strings.Join(formattedArgs, " ")
}

func printFormattedLog(msg *types.LogMessage) {
	printMutex.Lock()
	defer printMutex.Unlock()

	// Header
	hColor := headerColor
	if msg.ExitCode != 0 {
		hColor = exitCodeColor
	}
	hColor.Printf("PID: %d |PPID: %d |CMD: %s |EXIT: %d |TIME: %s\n", msg.PID, msg.PPID, msg.Command, msg.ExitCode, msg.Timestamp.Format("15:04:05.000"))

	// Arguments
	displayArgs := formatArgsForDisplay(msg.Args)
	cmdColor.Printf("%s %s\n", msg.Command, displayArgs)

	// Stdin, Stdout, Stderr
	printData(stdinColor, "STDIN", msg.StdinData)
	printData(stdoutColor, "STDOUT", msg.StdoutData)
	printData(stderrColor, "STDERR", msg.StderrData)

	headerColor.Print(strings.Repeat("-", 80))
	fmt.Println()
}
