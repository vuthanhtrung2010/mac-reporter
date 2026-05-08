package main

import (
	"encoding/json"
	"log"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

// These variables can be overridden at build time with -ldflags "-X 'main.websocketURL=...' -X 'main.username=...'
var websocketURL = "ws://localhost:8080/ws"
var username = "devtrung"

func main() {
	// Define the target network interface (adjust if needed)
	interfaceName := "wlp2s0"

	// Look up the interface by its name
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		log.Fatalf("Failed to get interface '%s': %v", interfaceName, err)
	}

	macAddr := iface.HardwareAddr.String()
	if macAddr == "" {
		log.Fatalf("The interface %s does not have a MAC address.", interfaceName)
	}

	payload := map[string]string{
		"username": username,
		"MAC":      macAddr,
	}

	for {
		// Try to connect
		conn, _, err := websocket.DefaultDialer.Dial(websocketURL, nil)
		if err != nil {
			log.Printf("Failed to connect to %s: %v. Retrying in 5s...", websocketURL, err)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Printf("Connected to %s", websocketURL)

		ticker := time.NewTicker(5 * time.Second)
		done := make(chan struct{})

		go func() {
			defer conn.Close()
			for {
				select {
				case <-ticker.C:
					b, _ := json.Marshal(payload)
					if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
						log.Printf("write error: %v", err)
						ticker.Stop()
						close(done)
						return
					}
					log.Printf("Sent heartbeat: %s", string(b))
				}
			}
		}()

		// Block until the writer goroutine signals done (on write error/close)
		<-done
		log.Printf("Disconnected, will attempt reconnect in 2s...")
		time.Sleep(2 * time.Second)
	}
}
