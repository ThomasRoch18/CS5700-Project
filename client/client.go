package main

import (
	"fmt"
	"log"
	"net"
)

func main() {
	conn, err := net.Dial("tcp", "127.0.0.1:5000")

	if err != nil {
		log.Fatal(err)
	}

	defer conn.Close()

	fmt.Println("Successfully connected to server")
}
