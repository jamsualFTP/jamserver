package server

import (
	"fmt"
	"net"
	"strings"
)

func HandleHelpConnection(helpConn *net.TCPConn, client *Client) {
	var availableCommands []string

	globalCommands := []string{
		"help",
		"echo",
		"hllo",
		"rgsr",
		"user",
		"pass",
		"quit",
	}
	availableCommands = append(availableCommands, globalCommands...)

	if client.Session.Authenticated {
		sessionCommands := []string{
			"pasv",
			"list",
		}
		availableCommands = append(availableCommands, sessionCommands...)
	}

	commandList := []byte(strings.Join(availableCommands, " "))

	if _, err := helpConn.Write(commandList); err != nil {
		fmt.Printf("Error writing commands: %v\n", err)
	}

	defer func() {
		if err := helpConn.Close(); err != nil {
			fmt.Printf("Error closing connection: %v\n", err)
		}
	}()
}
