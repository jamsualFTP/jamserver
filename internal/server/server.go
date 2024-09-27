package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

// NOTE: https://www.rfc-editor.org/rfc/rfc959
// TODO : implement auth system,
//                  typical ftp commands

var (
	connectionCounter int
	activeConnections = make(map[int]*net.TCPConn)
	mu                sync.Mutex // mutex for  handle concurrent connections
)

func Run() error {
	address := "127.0.0.1:2121"
	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return fmt.Errorf("resolving address error %w", err)
	}
	listener, listenErr := net.ListenTCP("tcp", tcpAddr)
	if listenErr != nil {
		return fmt.Errorf("listening error %w", listenErr)
	}
	defer listener.Close()

	fmt.Println("jamsualFTP started!")
	fmt.Printf("Listening on %v at port %v \n", tcpAddr.IP, tcpAddr.Port)

	for {
		fmt.Print("Waiting for upcoming connections... \n\n")
		conn, acceptErr := listener.AcceptTCP()
		if acceptErr != nil {
			log.Printf("connection error %v", acceptErr)
			continue
		}

		mu.Lock()
		connectionCounter++
		id := connectionCounter
		activeConnections[id] = conn
		mu.Unlock()

		addr := conn.RemoteAddr().String()
		fmt.Printf("Accepted new connection: id = %v! %v \n\n", id, addr)
		go HandleConnection(conn, id)
	}
}

func HandleConnection(conn *net.TCPConn, id int) {
	defer func() {
		conn.Close()
		mu.Lock()
		delete(activeConnections, id)
		fmt.Printf("Connection %v closed and removed from active list\n", id)
		mu.Unlock()
	}()

	conn.Write([]byte(fmt.Sprintf("220 Welcome to jamsualFTP server, user %v! \n", id)))

	for {
		buffer := make([]byte, 128)
		n, err := conn.Read(buffer)

		if err == io.EOF {
			return
		}

		fmt.Printf("Message from client: %s \n", string(buffer[:n]))
	}
}
