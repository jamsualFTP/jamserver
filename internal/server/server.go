package server

import (
	"fmt"
	"io"
	"jamserver/internal/jfs"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// NOTE: https://www.rfc-editor.org/rfc/rfc959

type Session struct {
	DTPConnection  net.Conn
	HelpConnection net.Conn
	DTPListener    net.Listener
	Login          string
	Authenticated  bool
	Passive        bool
	mu             sync.Mutex
}

type Client struct {
	Session *Session
	Conn    *net.TCPConn
}

var (
	connectionCounter int
	activeConnections = make(map[int]*Client)
	mu                sync.Mutex // mutex to handle concurrent connections

	globalFileSystem *jfs.FileSystem
)

func Run() error {
	IP_ADDRESS := "0.0.0.0:"
	PORT_TCP := "2121"
	PORT_HELP := "2222"

	tcpAddrStr := IP_ADDRESS + PORT_TCP
	helpAddrStr := IP_ADDRESS + PORT_HELP

	BASE_PATH := "app/jam_filesystem"

	tcpAddr, err := net.ResolveTCPAddr("tcp", tcpAddrStr)
	if err != nil {
		return fmt.Errorf("resolving tcp address error %w", err)
	}

	listener, listenErr := net.ListenTCP("tcp", tcpAddr)
	if listenErr != nil {
		return fmt.Errorf("listening error %w", listenErr)
	}
	defer listener.Close()

	fmt.Println("jamsualFT started!")
	fmt.Printf("Listening on %v at port %v \n", tcpAddr.IP, tcpAddr.Port)

	fmt.Println("File System Initialization...")

	fErr := jfs.InitializeFS(BASE_PATH)
	if fErr != nil {
		return fmt.Errorf("initializing FS error : %v", err)
	}

	globalFileSystem = jfs.NewFileSystem(BASE_PATH)

	helpAddr, helpErr := net.ResolveTCPAddr("tcp", helpAddrStr)
	if helpErr != nil {
		return fmt.Errorf("HELP resolving address error %v ", helpErr)
	}

	helpListener, helpListenErr := net.ListenTCP("tcp", helpAddr)
	if helpListenErr != nil {
		return fmt.Errorf("HELP listening error %v", helpListenErr)
	}
	defer helpListener.Close()

	go handleHelpListener(helpListener)

	for {
		fmt.Print("Waiting for upcoming connections... \n\n")
		conn, acceptErr := listener.AcceptTCP()
		if acceptErr != nil {
			log.Printf("connection error %v", acceptErr)
			continue
		}

		client := &Client{
			Conn:    conn,
			Session: &Session{},
		}

		mu.Lock()
		connectionCounter++
		id := connectionCounter
		activeConnections[id] = client
		mu.Unlock()

		incAddr := conn.RemoteAddr().String()
		fmt.Printf("Accepted new connection: id = %v! %v \n\n", id, incAddr)

		go HandleConnection(client, id)
	}
}

func HandleDisconnect(client *Client, id int, quitChan chan bool) {
	mu.Lock()
	defer mu.Unlock()

	if err := client.Conn.Close(); err != nil {
		fmt.Printf("Error closing connection %v: %v\n", id, err)
	}

	if client.Session != nil && client.Session.HelpConnection != nil {
		helpConn := client.Session.HelpConnection
		client.Session.HelpConnection = nil
		err := helpConn.Close()
		if err != nil {
			fmt.Printf("Error closing help connection: %v\n, %v", id, err)
		}
	}

	close(quitChan)
	client.Session = nil
	delete(activeConnections, id)
	fmt.Printf("Connection %v closed and removed from active list\n", id)
}

func HandleConnection(client *Client, id int) {
	quitChan := make(chan bool)
	defer HandleDisconnect(client, id, quitChan)

	time.Sleep(time.Second)
	fmt.Fprintf(client.Conn, "\033[36m220  \033[0mWelcome to jamsualFT server, user %v! \n\n", id)
	fmt.Fprintf(client.Conn, "Available commands: \n     help, echo, hllo, rgsr, user, pass, quit  \n\n")

	for {
		select {
		case <-quitChan:
			return
		default:
			buffer := make([]byte, 1024) // request buffer
			n, err := client.Conn.Read(buffer)

			if err == io.EOF {
				return
			}

			if err != nil {
				fmt.Printf("Error reading from connection %v: %v\n", id, err)
				return
			}

			str := strings.TrimSpace(string(buffer[:n]))
			part := strings.Split(str, " ")
			command := strings.ToUpper(part[0])
			args := part[1:]
			go HandleCommands(client, command, args)
		}
	}
}
