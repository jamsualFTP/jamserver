package main

import (
	"jamsual/internal/server"
	"log"
)

func main() {
	if err := server.Run(); err != nil {
		log.Fatalf("Server failed %v", err)
	}
}
