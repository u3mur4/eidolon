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
	headerColor = color.New(color.FgCyan, color.Bold)
	cmdColor    = color.New(color.FgYellow)
	stdinColor  = color.New(color.FgGreen)
	stdoutColor = color.New(color.FgWhite)
	stderrColor = color.New(color.FgRed)
	hexColor    = color.New(color.Faint)
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
func printData(c *color.Color, title string, data []byte) {
	if len(data) == 0 {
		return
	}
	c.Printf("%s (%d bytes)\n", title, len(data))
	if isPrintable(data) {
		c.Println(string(data))
	} else {
		hexColor.Println(hex.Dump(data))
	}
}

// formatArgsForDisplay takes a slice of string arguments and formats them for display.
// If an argument contains no printable characters (but is not empty), it is
// converted to a quoted hex string (e.g., "\x01\x02"). Otherwise, it's returned as is.
func formatArgsForDisplay(args []string) string {
	// Create a new slice to hold the formatted arguments.
	formattedArgs := make([]string, len(args))

	for i, arg := range args {
		// An empty string has 0 printable characters, but should be displayed as is.
		// We only want to format non-empty strings that are fully non-printable.
		if len(arg) > 0 && printableCount([]byte(arg)) == 0 {
			// This argument consists entirely of non-printable characters.
			// Format it as a quoted hex string.
			var hexBuilder strings.Builder
			for _, b := range []byte(arg) {
				// Format each byte as \xHH (e.g., \x0a, \x1f)
				hexBuilder.WriteString(fmt.Sprintf("\\x%02x", b))
			}
			formattedArgs[i] = fmt.Sprintf("\"%s\"", hexBuilder.String())
		} else {
			// This argument is either printable or empty. Keep it as is.
			formattedArgs[i] = arg
		}

		formattedArgs[i] = fmt.Sprintf("|%s|", formattedArgs[i])
	}

	// Join the now-formatted arguments with spaces.
	return strings.Join(formattedArgs, " ")
}

func printFormattedLog(msg *types.LogMessage) {
	printMutex.Lock()
	defer printMutex.Unlock()

	// Header
	headerColor.Printf("PID: %d |PPID: %d |CMD: %s |EXIT: %d |TIME: %s\n", msg.PID, msg.PPID, msg.Command, msg.ExitCode, msg.Timestamp.Format("15:04:05.000"))

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
