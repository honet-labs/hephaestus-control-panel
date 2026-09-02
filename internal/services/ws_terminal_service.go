package services

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

type WsTerminalService struct {
	sshService *SSHService
}

func NewWsTerminalService(sshService *SSHService) *WsTerminalService {
	return &WsTerminalService{sshService: sshService}
}

func (s *WsTerminalService) HandleWebSocketSession(ws *websocket.Conn, cfg *domain.RemoteHostConfig, initialCols, initialRows int, userID int) {
	defer ws.Close()

	client, err := s.sshService.Dial(cfg)
	if err != nil {
		_ = ws.WriteJSON(domain.WsTerminalMessage{
			Type:    "error",
			Message: fmt.Sprintf("SSH connection failed: %v", err),
		})
		return
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		_ = ws.WriteJSON(domain.WsTerminalMessage{
			Type:    "error",
			Message: fmt.Sprintf("SSH session allocation failed: %v", err),
		})
		return
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	cols := initialCols
	if cols <= 0 {
		cols = 80
	}
	rows := initialRows
	if rows <= 0 {
		rows = 24
	}

	if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		_ = ws.WriteJSON(domain.WsTerminalMessage{
			Type:    "error",
			Message: fmt.Sprintf("Request PTY failed: %v", err),
		})
		return
	}

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		return
	}
	defer stdinPipe.Close()

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return
	}
	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return
	}

	if err := session.Shell(); err != nil {
		_ = ws.WriteJSON(domain.WsTerminalMessage{
			Type:    "error",
			Message: fmt.Sprintf("Failed to spawn shell: %v", err),
		})
		return
	}

	_ = ws.WriteJSON(domain.WsTerminalMessage{
		Type:    "connected",
		Message: fmt.Sprintf("Connected to %s (%s@%s:%d)", cfg.Name, cfg.Username, cfg.Host, cfg.Port),
	})

	var wsMu sync.Mutex
	writeWs := func(msg domain.WsTerminalMessage) error {
		wsMu.Lock()
		defer wsMu.Unlock()
		return ws.WriteJSON(msg)
	}

	done := make(chan struct{})
	var once sync.Once
	closeDone := func() {
		once.Do(func() {
			close(done)
		})
	}
	defer closeDone()

	// Goroutine 1: OpenSSH KeepAlive ticker (every 15s) to prevent SSH timeouts
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_, _, _ = client.SendRequest("keepalive@openssh.com", true, nil)
			}
		}
	}()

	// Goroutine 2: WebSocket Ping Heartbeat ticker (every 20s) to keep proxy/browser alive
	go func() {
		pingTicker := time.NewTicker(20 * time.Second)
		defer pingTicker.Stop()
		for {
			select {
			case <-done:
				return
			case <-pingTicker.C:
				wsMu.Lock()
				_ = ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second))
				wsMu.Unlock()
			}
		}
	}()

	// Goroutine 3: SSH stdout -> WebSocket
	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				_ = writeWs(domain.WsTerminalMessage{
					Type: "data",
					Data: string(buf[:n]),
				})
			}
			if err != nil {
				break
			}
		}
		_ = writeWs(domain.WsTerminalMessage{Type: "disconnected"})
		closeDone()
		_ = ws.Close()
	}()

	// Goroutine 4: SSH stderr -> WebSocket
	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := stderrPipe.Read(buf)
			if n > 0 {
				_ = writeWs(domain.WsTerminalMessage{
					Type: "data",
					Data: string(buf[:n]),
				})
			}
			if err != nil {
				break
			}
		}
	}()

	// Configure WebSocket Ping/Pong handlers & Read deadline (60s)
	ws.SetReadLimit(65536)
	_ = ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	ws.SetPongHandler(func(string) error {
		_ = ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Main Loop: Read from WebSocket -> SSH stdin
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			break
		}
		_ = ws.SetReadDeadline(time.Now().Add(60 * time.Second))

		var msg domain.WsTerminalMessage
		if err := json.Unmarshal(message, &msg); err == nil {
			switch msg.Type {
			case "input", "stdin":
				_, _ = io.WriteString(stdinPipe, msg.Data)
			case "resize":
				if msg.Cols > 0 && msg.Rows > 0 {
					_ = session.WindowChange(msg.Rows, msg.Cols)
				}
			case "ping":
				_ = writeWs(domain.WsTerminalMessage{Type: "pong"})
			case "disconnect":
				return
			case "auth":
				// Handshake ack
				_ = writeWs(domain.WsTerminalMessage{Type: "connected"})
			}
		} else {
			// Raw input fallback
			_, _ = stdinPipe.Write(message)
		}
	}

	logger.Info("SSH", fmt.Sprintf("Terminal session ended for %s@%s", cfg.Username, cfg.Host))
}
