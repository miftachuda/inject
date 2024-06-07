package main

import (
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gorilla/websocket"
)

func main() {
	// Set up TCP listener
	listener, err := net.Listen("tcp", "127.0.0.1:2020")
	if err != nil {
		log.Fatalf("Error setting up TCP listener: %v", err)
	}
	defer listener.Close()

	log.Println("Listening on 127.0.0.1:2020")

	// Handle incoming connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	// Connect to the WebSocket server
	wsConn, _, err := websocket.DefaultDialer.Dial("wss://wshome.miftachuda.my.id", nil)
	if err != nil {
		log.Printf("Error connecting to WebSocket server: %v", err)
		return
	}
	defer wsConn.Close()

	log.Println("Connected to WebSocket server")

	// Channel to handle exit on EOF
	done := make(chan struct{})
	var once sync.Once
	var wg sync.WaitGroup

	// Read from TCP connection and send to WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		buffer := make([]byte, 1024)
		for {
			n, err := conn.Read(buffer)
			if err != nil {
				if err != io.EOF {
					log.Printf("Error reading from TCP connection: %v", err)
				}
				once.Do(func() { close(done) })
				return
			}
			err = wsConn.WriteMessage(websocket.BinaryMessage, buffer[:n])
			if err != nil {
				log.Printf("Error writing to WebSocket: %v", err)
				once.Do(func() { close(done) })
				return
			}
		}
	}()

	// Read from WebSocket and send to TCP connection
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, message, err := wsConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("Error reading from WebSocket: %v", err)
				}
				once.Do(func() { close(done) })
				return
			}
			_, err = conn.Write(message)
			if err != nil {
				log.Printf("Error writing to TCP connection: %v", err)
				once.Do(func() { close(done) })
				return
			}
		}
	}()

	// Wait for EOF or interrupt signal
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	select {
	case <-done:
	case <-sigc:
	}

	// Wait for goroutines to finish
	wg.Wait()

	log.Println("Connection closed")
}
