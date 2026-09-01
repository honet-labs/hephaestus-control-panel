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

	// Goroutine: SSH stdout -> WebSocket
	go func() {
		buf := make([]byte, 4096)
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
		_ = ws.Close()
	}()

	// Goroutine: SSH stderr -> WebSocket
	go func() {
		buf := make([]byte, 4096)
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

	// Read from WebSocket -> SSH stdin
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			break
		}

		var msg domain.WsTerminalMessage
		if err := json.Unmarshal(message, &msg); err == nil {
			switch msg.Type {
			case "input":
				_, _ = io.WriteString(stdinPipe, msg.Data)
			case "resize":
				if msg.Cols > 0 && msg.Rows > 0 {
					_ = session.WindowChange(msg.Rows, msg.Cols)
				}
			case "disconnect":
				return
			case "ping":
				_ = writeWs(domain.WsTerminalMessage{Type: "pong"})
			}
		} else {
			// Raw input fallback
			_, _ = stdinPipe.Write(message)
		}
	}

	logger.Info("SSH", fmt.Sprintf("Terminal session ended for %s@%s", cfg.Username, cfg.Host))
}
