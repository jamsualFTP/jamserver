package dtp

import (
	"fmt"
	"net"
)

type Session struct {
	Login         string
	Authenticated bool
}

type Client struct {
	Session *Session
	Conn    *net.TCPConn
}

func InitDTPConnection() {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		fmt.Printf("Error creating listener: %v\n", err)
		return
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	ip := addr.IP.String()
	port := addr.Port

	port1 := port / 256
	port2 := port % 256

	// return ip, port1, port2

	// for {
	// 	// TODO: init fs
	// }
}
