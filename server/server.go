package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Try to read the first message to learn the username (so we can log "Client <username> connected")
	_, firstMsg, err := conn.ReadMessage()
	if err != nil {
		log.Printf("read error (initial): %v", err)
		log.Printf("Client connected: %s", r.RemoteAddr)
		return
	}

	var firstPayload map[string]string
	if err := json.Unmarshal(firstMsg, &firstPayload); err != nil {
		log.Printf("Client connected (unknown): %s", r.RemoteAddr)
		log.Printf("received (raw initial): %s", string(firstMsg))
	} else {
		uname := firstPayload["username"]
		mac := firstPayload["MAC"]
		if uname == "" {
			uname = "unknown"
		}
		log.Printf("Client %s connected (addr=%s, MAC=%s)", uname, r.RemoteAddr, mac)
		// Also log the first heartbeat
		log.Printf("Heartbeat received from %s: MAC=%s", uname, mac)
	}

	// Continue reading subsequent messages
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("read error: %v", err)
			return
		}

		var payload map[string]string
		if err := json.Unmarshal(msg, &payload); err != nil {
			log.Printf("received (raw): %s", string(msg))
			continue
		}

		log.Printf("Heartbeat received from %s: MAC=%s", payload["username"], payload["MAC"])
	}
}

func main() {
	http.HandleFunc("/ws", wsHandler)
	addr := ":8080"
	log.Printf("WebSocket server listening on %s/ws", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
