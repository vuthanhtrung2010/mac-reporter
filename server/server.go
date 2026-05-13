package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
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

// --- Messages ---
type clientConnectedMsg struct {
	StudentID string
	MAC       string
}
type clientDisconnectedMsg struct {
	StudentID string
}
type timeoutWarningMsg struct {
	StudentID string
}
type tickMsg time.Time

// --- Model ---
type userState struct {
	StudentID    string
	MAC          string
	IsConnected  bool
	DisconnectTs time.Time
	Warned       bool
}

type model struct {
	table    table.Model
	users    map[string]*userState
	logs     []string
	manager  *UserManager
	upgrader websocket.Upgrader
}

var (
	baseStyle  = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240"))
	logStyle   = lipgloss.NewStyle().MarginTop(1)
	greenText  = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	yellowText = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	redText    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func newModel(manager *UserManager) model {
	columns := []table.Column{
		{Title: "Student ID", Width: 15},
		{Title: "MAC Address", Width: 20},
		{Title: "Status", Width: 15},
	}
	t := table.New(table.WithColumns(columns), table.WithHeight(10))
	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).BorderBottom(true).Bold(false)
	s.Selected = s.Selected.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Bold(false)
	t.SetStyles(s)

	users := make(map[string]*userState)
	for id, mac := range manager.Users {
		users[id] = &userState{StudentID: id, MAC: mac, IsConnected: false}
	}

	return model{
		table:   t,
		users:   users,
		logs:    []string{"Server started on :8080/ws"},
		manager: manager,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case clientConnectedMsg:
		if u, ok := m.users[msg.StudentID]; ok {
			if u.MAC != "" && u.MAC != msg.MAC {
				m.addLog(redText.Render(fmt.Sprintf("[!] WARNING: Client %s reconnected with DIFFERENT MAC! (old: %s, new: %s)", msg.StudentID, u.MAC, msg.MAC)))
			} else {
				m.addLog(greenText.Render(fmt.Sprintf("[+] Client %s connected (MAC: %s)", msg.StudentID, msg.MAC)))
			}
			u.IsConnected = true
			u.MAC = msg.MAC
			u.Warned = false // reset
		} else {
			m.users[msg.StudentID] = &userState{StudentID: msg.StudentID, MAC: msg.MAC, IsConnected: true}
			m.addLog(greenText.Render(fmt.Sprintf("[+] Client %s connected (MAC: %s)", msg.StudentID, msg.MAC)))
		}
		m.manager.Register(msg.StudentID, msg.MAC)
		m.updateTable()

	case clientDisconnectedMsg:
		if u, ok := m.users[msg.StudentID]; ok {
			u.IsConnected = false
			u.DisconnectTs = time.Now()
			u.Warned = false
		}
		m.addLog(yellowText.Render(fmt.Sprintf("[-] Client %s disconnected.", msg.StudentID)))
		m.updateTable()

	case timeoutWarningMsg:
		m.addLog(redText.Render(fmt.Sprintf("[!] WARNING: Client %s has been disconnected for >20s!", msg.StudentID)))

	case tickMsg:
		now := time.Now()
		for id, u := range m.users {
			if !u.IsConnected && !u.DisconnectTs.IsZero() && !u.Warned {
				if now.Sub(u.DisconnectTs) > 20*time.Second {
					u.Warned = true
					cmds = append(cmds, func() tea.Msg { return timeoutWarningMsg{StudentID: id} })
				}
			}
		}
		cmds = append(cmds, tickCmd())
	}

	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m *model) addLog(s string) {
	msg := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), s)
	m.logs = append(m.logs, msg)
	if len(m.logs) > 8 {
		m.logs = m.logs[len(m.logs)-8:]
	}
}

func (m *model) updateTable() {
	var rows []table.Row
	var ids []string
	for id := range m.users {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		u := m.users[id]
		status := "Offline"
		if u.IsConnected {
			status = greenText.Render("Connected")
		} else if !u.DisconnectTs.IsZero() {
			if u.Warned {
				status = redText.Render("Offline (>20s)")
			} else {
				status = yellowText.Render("Disconnected")
			}
		}
		rows = append(rows, table.Row{u.StudentID, u.MAC, status})
	}
	m.table.SetRows(rows)
}

func (m model) View() string {
	s := baseStyle.Render(m.table.View()) + "\n"
	logView := "Server Logs:\n"
	for _, l := range m.logs {
		logView += l + "\n"
	}
	return s + logStyle.Render(logView) + "\n(Press q to quit)"
}

func wsHandler(p *tea.Program, upgrader websocket.Upgrader, manager *UserManager) http.HandlerFunc {
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
			// Decline connection immediately with PolicyViolation close frame
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "unknown user"))
			return
		}

		if uname == "" {
			uname = "unknown"
		}

		p.Send(clientConnectedMsg{StudentID: uname, MAC: mac})

		// Heartbeat loop
		for {
			conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			_, msg, err := conn.ReadMessage()
			if err != nil {
				p.Send(clientDisconnectedMsg{StudentID: uname})
				return
			}
			// (Optional) We could send a heartbeat log, but it clutters the UI. So ignoring here.
			_ = msg
		}
	}
}

func main() {
	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()
	}

	manager := NewUserManager("users.json")
	m := newModel(manager)
	m.updateTable()

	p := tea.NewProgram(m, tea.WithAltScreen())

	http.HandleFunc("/ws", wsHandler(p, m.upgrader, manager))
	go func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
