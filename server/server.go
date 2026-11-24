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

const storageDir = "storage"
const maxFileSize int64 = 100 * 1024 * 1024

func main() {
	err := os.MkdirAll(storageDir, 0755)

	if err != nil {
		log.Fatal("Failed to create storage directory:", err)
	}

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

	reader := bufio.NewReader(conn)

	authenticated := false

	for {
		message, err := reader.ReadString('\n')

		if err != nil {
			fmt.Println("Client disconnected:", conn.RemoteAddr())
			return
		}

		message = strings.TrimSpace(message)
		parts := strings.Split(message, "|")

		fmt.Println("Received from", conn.RemoteAddr(), ":", message)

		switch parts[0] {

		// Server case for AUTH
		case "AUTH":
			if len(parts) != 3 {
				fmt.Fprintln(conn, "ERROR|INVALID_FORMAT")
				continue
			}

			if parts[1] == "user" && parts[2] == "password" {
				authenticated = true

				fmt.Fprintln(conn, "ACK|AUTH")
				fmt.Println(conn.RemoteAddr(), "authenticated successfully")
			} else {
				authenticated = false

				fmt.Fprintln(conn, "ERROR|INVALID_CREDENTIALS")
				fmt.Println(conn.RemoteAddr(), "authentication failed")
			}

		// Server case for FILE
		case "FILE":
			if !authenticated {
				fmt.Fprintln(conn, "ERROR|UNAUTHORIZED")
				continue
			}

			if len(parts) < 2 {
				fmt.Fprintln(conn, "ERROR|INVALID_FORMAT")
				continue
			}

			switch parts[1] {

			// Server case for FILE|LIST
			case "LIST":
				if len(parts) != 2 {
					fmt.Fprintln(conn, "ERROR|INVALID_FORMAT")
					continue
				}

				entries, err := os.ReadDir(storageDir)

				if err != nil {
					fmt.Fprintln(conn, "ERROR|LIST_FAILED")
				}

				var filenames []string

				for _, entry := range entries {
					if entry.Type().IsRegular() {
						filenames = append(filenames, entry.Name())
					}
				}

				response := strings.Join(
					append([]string{"FILE_LIST"}, filenames...), "|",
				)

				fmt.Fprintln(conn, response)

				log.Printf("[%s] Listed %d files",
					conn.RemoteAddr(), len(filenames))

			// Server case for FILE|UPLOAD|...
			case "UPLOAD":
				if len(parts) != 4 {
					fmt.Fprintln(conn, "ERROR|INVALID_FORMAT")
					continue
				}

				filename := parts[2]

				if !isFileNameValid(filename) {
					fmt.Fprintln(conn, "ERROR|INVALID_FILENAME")
					continue
				}

				size, err := strconv.ParseInt(parts[3], 10, 64)

				if err != nil || size < 0 || size > maxFileSize {
					fmt.Fprintln(conn, "ERROR|INVALID_SIZE")
					continue
				}

				filePath := filepath.Join(storageDir, filename)

				file, err := os.OpenFile(
					filePath,
					os.O_WRONLY|os.O_CREATE|os.O_EXCL,
					0644,
				)

				if err != nil {
					fmt.Fprintln(conn, "ERROR|CREATE_FILE_FAILED")
					continue
				}

				_, err = fmt.Fprintln(conn, "ACK|READY")

				if err != nil {
					file.Close()
					os.Remove(filePath)
					return
				}

				bytesReceived, err := io.CopyN(file, reader, size)

				closeErr := file.Close()

				if err != nil || closeErr != nil {
					os.Remove(filePath)
					fmt.Fprintln(conn, "ERROR|UPLOAD_FAILED")
					return
				}

				_, err = fmt.Fprintln(conn, "ACK|UPLOAD")

				if err != nil {
					log.Println("Failed to send upload acknowledgment:", err)
					return
				}

				log.Printf(
					"[%s] Uploaded %s (%d bytes)",
					conn.RemoteAddr(),
					filename,
					bytesReceived,
				)

			// Server case for FILE|DOWNLOAD|...
			case "DOWNLOAD":
				if len(parts) != 3 {
					fmt.Fprintln(conn, "ERROR|INVALID_FORMAT")
					continue
				}

				filename := parts[2]

				if !isFileNameValid(filename) {
					fmt.Fprintln(conn, "ERROR|INVALID_FILENAME")
					continue
				}

				filePath := filepath.Join(storageDir, filename)

				file, err := os.Open(filePath)

				if err != nil {
					if os.IsNotExist(err) {
						fmt.Fprintln(conn, "ERROR|FILE_NOT_FOUND")
					} else {
						fmt.Fprintln(conn, "ERROR|OPEN_FILE_FAILED")
					}
					continue
				}

				info, err := file.Stat()

				if err != nil || !info.Mode().IsRegular() {
					file.Close()
					fmt.Fprintln(conn, "ERROR|INVALID_FILE")
					continue
				}

				size := info.Size()

				_, err = fmt.Fprintf(
					conn,
					"FILE_DATA|%s|%d\n",
					filename,
					size,
				)

				if err != nil {
					file.Close()
					return
				}

				bytesSent, err := io.CopyN(conn, file, size)

				file.Close()

				if err != nil {
					log.Println("Download transfer failed:", err)
					return
				}

				log.Printf(
					"[%s] Downloaded %s (%d bytes)",
					conn.RemoteAddr(),
					filename,
					bytesSent,
				)

			// Server case for FILE|DELETE|...
			case "DELETE":
				if len(parts) != 3 {
					fmt.Fprintln(conn, "ERROR|INVALID_FORMAT")
					continue
				}

				filename := parts[2]

				if !isFileNameValid(filename) {
					fmt.Fprintln(conn, "ERROR|INVALID_FILENAME")
					continue
				}

				filePath := filepath.Join(storageDir, filename)

				info, err := os.Lstat(filePath)

				if err != nil {
					if os.IsNotExist(err) {
						fmt.Fprintln(conn, "ERROR|FILE_NOT_FOUND")
					} else {
						fmt.Fprintln(conn, "ERROR|DELETE_FAILED")
					}
					continue
				}

				if !info.Mode().IsRegular() {
					fmt.Fprintln(conn, "ERROR|INVALID_FILE")
					continue
				}

				err = os.Remove(filePath)

				if err != nil {
					if os.IsNotExist(err) {
						fmt.Fprintln(conn, "ERROR|FILE_NOT_FOUND")
					} else {
						fmt.Fprintln(conn, "ERROR|DELETE_FAILED")
					}
					continue
				}

				_, err = fmt.Fprintln(conn, "ACK|DELETE")

				if err != nil {
					log.Println("Failed to send delete acknowledgment:", err)
					return
				}

				log.Printf(
					"[%s] Deleted %s",
					conn.RemoteAddr(),
					filename,
				)

			default:
				fmt.Fprintln(conn, "ERROR|UNKNOWN_FILE_OPERATION")
			}
		}
	}
}

func isFileNameValid(filename string) bool {
	return filename != "" &&
		filename != "." &&
		filename != ".." &&
		filepath.Base(filename) == filename &&
		!strings.ContainsAny(filename, "/\\|:\r\n\x00")
}
