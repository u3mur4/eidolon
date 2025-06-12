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

// isPrintable checks if a byte slice contains primarily printable characters.
func isPrintable(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	printable := 0
	for _, b := range data {
		if b >= 32 && b < 127 || unicode.IsSpace(rune(b)) {
			printable++
		}
	}
	// Consider it printable if >90% of characters are printable
	return float64(printable)/float64(len(data)) > 0.9
}

// printData prints data as a string or a hex dump based on its content.
func printData(c *color.Color, title string, data []byte) {
	if len(data) == 0 {
		return
	}
	c.Printf("--- %s (%d bytes) ---\n", title, len(data))
	if isPrintable(data) {
		c.Println(string(data))
	} else {
		hexColor.Println(hex.Dump(data))
	}
}

func printFormattedLog(msg *types.LogMessage) {
	printMutex.Lock()
	defer printMutex.Unlock()

	// Header
	headerColor.Println("==============================================================================")
	headerColor.Printf(" PID: %d  |  CMD: %s  |  EXIT: %d  |  TIME: %s\n", msg.PID, msg.Command, msg.ExitCode, msg.Timestamp.Format("15:04:05.000"))
	headerColor.Println("------------------------------------------------------------------------------")

	// Arguments
	cmdColor.Printf("$ %s %s\n", msg.Command, strings.Join(msg.Args, " "))

	// Stdin, Stdout, Stderr
	printData(stdinColor, "STDIN", msg.StdinData)
	printData(stdoutColor, "STDOUT", msg.StdoutData)
	printData(stderrColor, "STDERR", msg.StderrData)

	headerColor.Println("==============================================================================")
	fmt.Println()
}
