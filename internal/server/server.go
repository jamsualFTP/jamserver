package server

import (
	"fmt"
	"io"
	"log"
	"net"
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
		fmt.Println("Waiting for upcoming connections... \n")
		conn, acceptErr := listener.AcceptTCP()
		if acceptErr != nil {
			log.Printf("connection error %v", acceptErr)
			continue
		}

		fmt.Println("Accepted new connection! \n")
		go HandleConnection(conn)
	}
}

func HandleConnection(conn *net.TCPConn) {
	defer conn.Close()
	conn.Write([]byte("220 Welcome to jamsualFTP server ! \n"))

	for {
		buffer := make([]byte, 128)
		n, err := conn.Read(buffer)
		if err == io.EOF {
			fmt.Println("Client closed the connection")
		}
		fmt.Printf("Message from client: %s \n", string(buffer[:n]))

		// TODO : implement auth system,
		//                  typical ftp commands
		// NOTE: https://www.rfc-editor.org/rfc/rfc959
	}
}
