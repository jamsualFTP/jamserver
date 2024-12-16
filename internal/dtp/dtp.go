package dtp

import (
	"fmt"
	"net"
)

func SendData(conn net.Conn, data string) error {
	defer conn.Close()

	_, err := conn.Write([]byte(data))
	if err != nil {
		fmt.Printf("Error sending data over DTP connection: %v\n", err)
		return err
	}

	fmt.Println("Data sent succesfully over DTP")
	return nil
}
