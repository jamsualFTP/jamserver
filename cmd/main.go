package main

import (
	"jamserver/internal/server"
	"log"
)

func main() {
	if err := server.Run(); err != nil {
		log.Fatalf("Server failed %v", err)
	}
}
