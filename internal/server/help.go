package server

import (
	"fmt"
	"net"
)

func HandleHelpConnection(helpConn *net.TCPConn, client *Client) {
	// mu.Lock()
	// defer mu.Unlock()

	globalCommands := "help echo hllo rgsr user pass quit "
	// TODO: add commands
	sessionCommands := "recv"
	if client.Session.Authenticated {
		helpConn.Write([]byte(globalCommands + sessionCommands))
	} else {
		helpConn.Write([]byte(globalCommands))
	}

	go func() {
		if err := helpConn.Close(); err != nil {
			fmt.Printf("Error closing connection: %v\n", err)
		}
	}()
}
