package main

import (
	"encoding/gob"
	"io"
	"log"
	"net"

	"github.com/u3mur4/eidolon/pkg/common/types"
)

// MessageHandler is called for each decoded log message
type MessageHandler func(*types.LogMessage)

// handleConnection decodes log messages from a connection and calls the handler
func handleConnection(conn net.Conn, handler MessageHandler) {
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
		handler(&msg)
	}
}
