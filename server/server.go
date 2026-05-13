package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
)

// --- Storage ---
type UserManager struct {
	mu       sync.Mutex
	filePath string
	Users    map[string]string // StudentID -> MAC
}

func NewUserManager(path string) *UserManager {
	m := &UserManager{
		filePath: path,
		Users:    make(map[string]string),
	}
	m.Load()
	return m
}

func (m *UserManager) Load() {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := os.ReadFile(m.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Failed to read %s: %v", m.filePath, err)
		}
		return
	}
	json.Unmarshal(b, &m.Users)
}

func (m *UserManager) Save() {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := json.MarshalIndent(m.Users, "", "  ")
	if err == nil {
		os.WriteFile(m.filePath, b, 0644)
	}
}

func (m *UserManager) Register(studentID, mac string) {
	m.mu.Lock()
	changed := false
	if m.Users[studentID] != mac {
		m.Users[studentID] = mac
		changed = true
	}
	m.mu.Unlock()
	if changed {
		m.Save()
	}
}

// --- Tracking ---
type clientState struct {
	isConnected  bool
	disconnectTs time.Time
	warned       bool
	mac          string
}

var (
	clientsMu sync.Mutex
	clients   = make(map[string]*clientState)
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func wsHandler(manager *UserManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		_, firstMsg, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var payload map[string]string
		if err := json.Unmarshal(firstMsg, &payload); err != nil {
			return
		}

		uname := payload["username"]
		mac := payload["MAC"]

		// Check if user exists in predefined users.json
		manager.mu.Lock()
		_, exists := manager.Users[uname]
		manager.mu.Unlock()

		if !exists {
			// Decline connection immediately
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "unknown user"))
			return
		}

		if uname == "" {
			uname = "unknown"
		}

		// Handle Connection Logic
		clientsMu.Lock()
		state, ok := clients[uname]
		if !ok {
			state = &clientState{mac: mac}
			clients[uname] = state
		}

		if state.mac != "" && state.mac != mac {
			log.Printf("%s[!] WARNING: Client %s reconnected with DIFFERENT MAC! (old: %s, new: %s)%s", ColorRed, uname, state.mac, mac, ColorReset)
		} else {
			log.Printf("%s[+] Client %s connected (MAC: %s)%s", ColorGreen, uname, mac, ColorReset)
		}

		state.isConnected = true
		state.mac = mac
		state.warned = false
		clientsMu.Unlock()

		manager.Register(uname, mac)

		// Heartbeat loop
		for {
			conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			_, _, err := conn.ReadMessage()
			if err != nil {
				// Disconnected
				clientsMu.Lock()
				if s, ok := clients[uname]; ok {
					s.isConnected = false
					s.disconnectTs = time.Now()
					s.warned = false
				}
				clientsMu.Unlock()

				log.Printf("%s[-] Client %s disconnected.%s", ColorYellow, uname, ColorReset)
				return
			}
		}
	}
}

func main() {
	manager := NewUserManager("users.json")

	// Load initial state from manager
	manager.mu.Lock()
	for id, mac := range manager.Users {
		clients[id] = &clientState{mac: mac, isConnected: false}
	}
	manager.mu.Unlock()

	// Routine to check for 20s timeouts
	go func() {
		for {
			time.Sleep(1 * time.Second)
			now := time.Now()

			clientsMu.Lock()
			for id, state := range clients {
				if !state.isConnected && !state.disconnectTs.IsZero() && !state.warned {
					if now.Sub(state.disconnectTs) > 20*time.Second {
						state.warned = true
						log.Printf("%s[!] WARNING: Client %s has been disconnected for >20s!%s", ColorRed, id, ColorReset)
					}
				}
			}
			clientsMu.Unlock()
		}
	}()

	http.HandleFunc("/ws", wsHandler(manager))
	log.Println("Server started on :8080/ws")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
