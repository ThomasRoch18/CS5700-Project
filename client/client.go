package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const downloadsDir = "downloads"

func main() {
	conn, err := net.Dial("tcp", "127.0.0.1:5000")

	if err != nil {
		log.Fatal(err)
	}

	defer conn.Close()

	fmt.Println("Successfully connected to server")

	inputReader := bufio.NewReader(os.Stdin)
	serverReader := bufio.NewReader(conn)

	for {
		err := os.MkdirAll(downloadsDir, 0755)

		if err != nil {
			log.Fatal("Failed to create downloads directory:", err)
		}

		fmt.Print("Enter command: ")

		message, err := inputReader.ReadString('\n')

		if err != nil {
			log.Println(err)
			return
		}

		message = strings.TrimSpace(message)

		if message == "EXIT" {
			return
		}

		parts := strings.Split(message, "|")

		// Client side for UPLOAD
		if len(parts) == 3 &&
			parts[0] == "FILE" &&
			parts[1] == "UPLOAD" {

			file, err := os.Open(parts[2])

			if err != nil {
				log.Println("Failed to open file:", err)
				continue
			}

			info, err := file.Stat()

			if err != nil || !info.Mode().IsRegular() {
				file.Close()
				log.Println("Invalid local file")
				continue
			}

			filename := filepath.Base(parts[2])
			size := info.Size()

			_, err = fmt.Fprintf(
				conn,
				"FILE|UPLOAD|%s|%d\n",
				filename,
				size,
			)

			if err != nil {
				file.Close()
				log.Println("Failed to send upload request:", err)
				return
			}

			response, err := serverReader.ReadString('\n')

			if err != nil {
				file.Close()
				log.Println("Failed to receive response:", err)
				return
			}

			if strings.TrimSpace(response) != "ACK|READY" {
				file.Close()
				fmt.Println("Server response:", strings.TrimSpace(response))
				continue
			}

			bytesSent, err := io.CopyN(conn, file, size)

			file.Close()

			if err != nil {
				log.Println("Failed to transfer file:", err)
				return
			}

			fmt.Printf("Sent %d bytes\n", bytesSent)

			response, err = serverReader.ReadString('\n')

			if err != nil {
				log.Println("Failed to receive upload confirmation:", err)
				return
			}

			fmt.Println("Server response:", strings.TrimSpace(response))

			continue
		}

		_, err = fmt.Fprintln(conn, message)

		if err != nil {
			log.Println("Failed to send message:", err)
			return
		}

		response, err := serverReader.ReadString('\n')

		if err != nil {
			log.Println("Failed to receive response:", err)
			return
		}

		responseParts := strings.Split(
			strings.TrimSpace(response), "|",
		)

		// Client side for DOWNLOAD
		if responseParts[0] == "FILE_DATA" {
			if len(responseParts) != 3 {
				log.Println("Invalid download response")
				return
			}

			filename := responseParts[1]

			size, err := strconv.ParseInt(
				responseParts[2], 10, 64,
			)

			if err != nil || size < 0 {
				log.Println("Invalid download size")
				return
			}

			if !isFileNameValid(filename) {
				log.Println("Invalid download filename")
				return
			}

			filePath := filepath.Join(downloadsDir, filename)

			file, err := os.OpenFile(
				filePath,
				os.O_WRONLY|os.O_CREATE|os.O_EXCL,
				0644,
			)

			if err != nil {
				log.Println("Failed to create download file:", err)
				return
			}

			bytesReceived, err := io.CopyN(
				file,
				serverReader,
				size,
			)

			closeErr := file.Close()

			if err != nil || closeErr != nil {
				os.Remove(filePath)
				log.Println("Download failed")
				return
			}

			fmt.Printf(
				"Downloaded %s (%d bytes) successfully\n",
				filename,
				bytesReceived,
			)

			continue
		}

		fmt.Println("Server response:", strings.TrimSpace(response))
	}
}

func isFileNameValid(filename string) bool {
	return filename != "" &&
		filename != "." &&
		filename != ".." &&
		filepath.Base(filename) == filename &&
		!strings.ContainsAny(filename, "/\\|:\r\n\x00")
}
