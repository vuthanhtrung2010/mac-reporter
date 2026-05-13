package main

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	ServerURL string `env:"SERVER_URL" env-required:"true"`
	StudentID string `env:"SBD" env-required:"true"`
}

func main() {
	// Read config from /root/dtth.conf using cleanenv
	cfg := Config{}
	if err := cleanenv.ReadConfig("/root/dtth.conf", &cfg); err != nil {
		log.Fatalf("Failed to read config from /root/dtth.conf: %v", err)
	}

	// Delete the config file immediately after reading
	if err := os.Remove("/root/dtth.conf"); err != nil {
		log.Printf("Warning: failed to delete /root/dtth.conf: %v", err)
	}

	// Find the first network interface that starts with "wl"
	var interfaceName string
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Fatalf("Failed to list interfaces: %v", err)
	}

	for _, iface := range ifaces {
		if strings.HasPrefix(iface.Name, "wl") {
			interfaceName = iface.Name
			break
		}
	}

	if interfaceName == "" {
		log.Fatalf("No wireless interface (wl*) found")
	}

	// Look up the interface by its name
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		log.Fatalf("Failed to get interface '%s': %v", interfaceName, err)
	}

	macAddr := iface.HardwareAddr.String()
	if macAddr == "" {
		log.Fatalf("The interface %s does not have a MAC address.", interfaceName)
	}

	log.Printf("Using interface %s with MAC %s and student ID %s", interfaceName, macAddr, cfg.StudentID)

	payload := map[string]string{
		"username": cfg.StudentID,
		"MAC":      macAddr,
	}

	for {
		// Try to connect
		conn, _, err := websocket.DefaultDialer.Dial(cfg.ServerURL, nil)
		if err != nil {
			log.Printf("Failed to connect to %s: %v. Retrying in 5s...", cfg.ServerURL, err)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Printf("Connected to %s", cfg.ServerURL)

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
