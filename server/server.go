package main

import (
	"fmt"
	"log"
	"net"
)

func main() {
	listener, err := net.Listen("tcp", "127.0.0.1:5000")

	if err != nil {
		log.Fatal(err)
	}

	defer listener.Close()

	fmt.Println("Server listening on 127.0.0.1:5000")

	for {
		conn, err := listener.Accept()

		if err != nil {
			log.Println(err)
			continue
		}

		go handleClient(conn)
	}
}

func handleClient(conn net.Conn) {
	defer conn.Close()

	fmt.Println("Client connected:", conn.RemoteAddr())
}
