package server

import (
	"fmt"
	"net"
	"strings"
	"time"
)

func handleHelpListener(helpListener *net.TCPListener) {
	for {
		// Accept the help connection
		helpConn, helpAcceptErr := helpListener.AcceptTCP()
		if helpAcceptErr != nil {
			fmt.Printf("HELP connection error %v\n", helpAcceptErr)
			continue
		}
		// Extract IP address from helpConn
		helpIP, _, err := net.SplitHostPort(helpConn.RemoteAddr().String())
		if err != nil {
			fmt.Printf("Error extracting IP from help connection: %v\n", err)
			helpConn.Close()
			continue
		}

		var associatedClient *Client
		mu.Lock()
		for _, client := range activeConnections {
			clientIP, _, err := net.SplitHostPort(client.Conn.RemoteAddr().String())
			if err != nil {
				continue
			}
			if helpIP == clientIP {
				associatedClient = client
				break
			}
		}
		mu.Unlock()

		// No matching client found
		if associatedClient == nil {
			fmt.Printf("No matching client session found for help connection: %v\n", helpConn.RemoteAddr().String())
			helpConn.Close()
			continue
		}

		// Assign the help connection to the session
		associatedClient.Session.HelpConnection = helpConn

		// Launch a goroutine to handle the help connection
		go HandleHelpConnection(helpConn, associatedClient)
	}
}

func HandleHelpConnection(helpConn *net.TCPConn, associatedClient *Client) {
	defer func() {
		if err := helpConn.Close(); err != nil {
			fmt.Printf("Error closing help connection: %v\n", err)
		}
	}()

	for {
		if associatedClient == nil {
			fmt.Println("Associated client is nil, closing help connection.")
			return
		}

		if associatedClient.Session == nil {
			fmt.Println("Associated Session is nil, closing help connection.")
			return
		}

		// Get available commands for the associated client
		availableCommands := getAvailableCommands(associatedClient)
		commandList := strings.Join(availableCommands, " ") + "\n"

		// Write commands to the help connection
		if _, err := helpConn.Write([]byte(commandList)); err != nil {
			fmt.Printf("Error writing commands to help connection: %v\n", err)
			return
		}

		// Update every 15 seconds
		time.Sleep(15 * time.Second)
	}
}

func getAvailableCommands(client *Client) []string {
	globalCommands := []string{"help", "echo", "hllo", "rgsr", "user", "pass", "quit"}

	if client == nil || client.Session == nil {
		return globalCommands
	}
	client.Session.mu.Lock()
	defer client.Session.mu.Unlock()

	if client.Session.Authenticated {
		sessionCommands := []string{"pasv", "list", "retr", "stor"}
		return append(globalCommands, sessionCommands...)
	}

	return globalCommands
}
