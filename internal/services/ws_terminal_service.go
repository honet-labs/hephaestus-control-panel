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

type persistentSSHSession struct {
	key         string
	client      *ssh.Client
	session     *ssh.Session
	stdinPipe   io.WriteCloser
	stdoutPipe  io.Reader
	stderrPipe  io.Reader
	cols        int
	rows        int
	activeWs    *websocket.Conn
	wsMu        sync.Mutex
	detachTimer *time.Timer
	closed      bool
	done        chan struct{}
	mu          sync.Mutex
	service     *WsTerminalService
}

func (p *persistentSSHSession) writeWs(msg domain.WsTerminalMessage) error {
	p.wsMu.Lock()
	defer p.wsMu.Unlock()
	if p.activeWs == nil {
		return nil
	}
	_ = p.activeWs.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return p.activeWs.WriteJSON(msg)
}

func (p *persistentSSHSession) closeSession() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	if p.detachTimer != nil {
		p.detachTimer.Stop()
		p.detachTimer = nil
	}
	select {
	case <-p.done:
	default:
		close(p.done)
	}
	if p.stdinPipe != nil {
		_ = p.stdinPipe.Close()
	}
	if p.session != nil {
		_ = p.session.Close()
	}
	if p.client != nil {
		_ = p.client.Close()
	}
	p.mu.Unlock()

	p.service.sessionsMu.Lock()
	delete(p.service.sessions, p.key)
	p.service.sessionsMu.Unlock()
	logger.Info("SSH", fmt.Sprintf("Persistent terminal session closed for key: %s", p.key))
}

func (p *persistentSSHSession) startOutputPipes() {
	// Stdout reader
	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := p.stdoutPipe.Read(buf)
			if n > 0 {
				_ = p.writeWs(domain.WsTerminalMessage{
					Type: "data",
					Data: string(buf[:n]),
				})
			}
			if err != nil {
				break
			}
		}
		_ = p.writeWs(domain.WsTerminalMessage{Type: "disconnected"})
		p.closeSession()
	}()

	// Stderr reader
	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := p.stderrPipe.Read(buf)
			if n > 0 {
				_ = p.writeWs(domain.WsTerminalMessage{
					Type: "data",
					Data: string(buf[:n]),
				})
			}
			if err != nil {
				break
			}
		}
	}()
}

func (p *persistentSSHSession) startKeepAlive() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-p.done:
				return
			case <-ticker.C:
				p.mu.Lock()
				isClosed := p.closed
				client := p.client
				p.mu.Unlock()

				if isClosed || client == nil {
					return
				}

				// 1. Send non-blocking OpenSSH keepalive to target VPS
				_, _, _ = client.SendRequest("keepalive@openssh.com", false, nil)

				// 2. Send native WebSocket Ping control frame (Opcode 0x9) for Cloudflare Tunnel / Edge keepalive
				p.wsMu.Lock()
				if p.activeWs != nil {
					_ = p.activeWs.WriteControl(websocket.PingMessage, []byte("hcp"), time.Now().Add(5*time.Second))
				}
				p.wsMu.Unlock()

				// 3. Send application ping to WebSocket client
				_ = p.writeWs(domain.WsTerminalMessage{Type: "ping"})
			}
		}
	}()
}

type WsTerminalService struct {
	sshService *SSHService
	sessions   map[string]*persistentSSHSession
	sessionsMu sync.RWMutex
}

func NewWsTerminalService(sshService *SSHService) *WsTerminalService {
	return &WsTerminalService{
		sshService: sshService,
		sessions:   make(map[string]*persistentSSHSession),
	}
}

func (s *WsTerminalService) HandleWebSocketSession(ws *websocket.Conn, cfg *domain.RemoteHostConfig, initialCols, initialRows int, userID int, sessionID string, forceNew bool) {
	if sessionID == "" {
		sessionID = cfg.ID
	}
	sessionKey := fmt.Sprintf("%d:%s:%s", userID, cfg.ID, sessionID)

	s.sessionsMu.Lock()
	pSess, exists := s.sessions[sessionKey]
	if exists && !pSess.closed {
		if forceNew {
			logger.Info("SSH", fmt.Sprintf("Force closing stale terminal session for clean reconnect: %s", sessionKey))
			pSess.closeSession()
			delete(s.sessions, sessionKey)
		} else {
			// Re-attach to existing live session
			pSess.mu.Lock()
			if pSess.detachTimer != nil {
				pSess.detachTimer.Stop()
				pSess.detachTimer = nil
			}
			// Close previous stale ws if any
			pSess.wsMu.Lock()
			if pSess.activeWs != nil && pSess.activeWs != ws {
				_ = pSess.activeWs.Close()
			}
			pSess.activeWs = ws
			pSess.wsMu.Unlock()

			if initialCols > 0 && initialRows > 0 && pSess.session != nil {
				_ = pSess.session.WindowChange(initialRows, initialCols)
			}
			pSess.mu.Unlock()
			s.sessionsMu.Unlock()

			logger.Info("SSH", fmt.Sprintf("Re-attached client to persistent terminal session: %s", sessionKey))
			_ = pSess.writeWs(domain.WsTerminalMessage{
				Type:    "connected",
				Message: fmt.Sprintf("Re-attached to active session: %s (%s)", cfg.Name, cfg.Host),
			})

			s.runWsReadLoop(pSess, ws)
			return
		}
	}
	s.sessionsMu.Unlock()

	// New SSH session required
	client, err := s.sshService.Dial(cfg)
	if err != nil {
		_ = ws.WriteJSON(domain.WsTerminalMessage{
			Type:    "error",
			Message: fmt.Sprintf("SSH connection failed: %v", err),
		})
		_ = ws.Close()
		return
	}

	session, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		_ = ws.WriteJSON(domain.WsTerminalMessage{
			Type:    "error",
			Message: fmt.Sprintf("SSH session allocation failed: %v", err),
		})
		_ = ws.Close()
		return
	}

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
		_ = session.Close()
		_ = client.Close()
		_ = ws.WriteJSON(domain.WsTerminalMessage{
			Type:    "error",
			Message: fmt.Sprintf("Request PTY failed: %v", err),
		})
		_ = ws.Close()
		return
	}

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		_ = ws.Close()
		return
	}

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		_ = stdinPipe.Close()
		_ = session.Close()
		_ = client.Close()
		_ = ws.Close()
		return
	}

	stderrPipe, err := session.StderrPipe()
	if err != nil {
		_ = stdinPipe.Close()
		_ = session.Close()
		_ = client.Close()
		_ = ws.Close()
		return
	}

	if err := session.Shell(); err != nil {
		_ = stdinPipe.Close()
		_ = session.Close()
		_ = client.Close()
		_ = ws.WriteJSON(domain.WsTerminalMessage{
			Type:    "error",
			Message: fmt.Sprintf("Failed to spawn shell: %v", err),
		})
		_ = ws.Close()
		return
	}

	pSess = &persistentSSHSession{
		key:        sessionKey,
		client:     client,
		session:    session,
		stdinPipe:  stdinPipe,
		stdoutPipe: stdoutPipe,
		stderrPipe: stderrPipe,
		cols:       cols,
		rows:       rows,
		activeWs:   ws,
		done:       make(chan struct{}),
		service:    s,
	}

	s.sessionsMu.Lock()
	s.sessions[sessionKey] = pSess
	s.sessionsMu.Unlock()

	pSess.startOutputPipes()
	pSess.startKeepAlive()

	_ = pSess.writeWs(domain.WsTerminalMessage{
		Type:    "connected",
		Message: fmt.Sprintf("Connected to %s (%s@%s:%d)", cfg.Name, cfg.Username, cfg.Host, cfg.Port),
	})

	s.runWsReadLoop(pSess, ws)
}

func (s *WsTerminalService) runWsReadLoop(p *persistentSSHSession, ws *websocket.Conn) {
	defer ws.Close()

	// Use generous 24-hour read deadline and 10MB read limit for large scripts/pastes
	const generousIdle = 24 * time.Hour
	ws.SetReadLimit(10 * 1024 * 1024)
	_ = ws.SetReadDeadline(time.Now().Add(generousIdle))

	ws.SetPongHandler(func(string) error {
		_ = ws.SetReadDeadline(time.Now().Add(generousIdle))
		return nil
	})
	ws.SetPingHandler(func(appData string) error {
		_ = ws.SetReadDeadline(time.Now().Add(generousIdle))
		p.wsMu.Lock()
		if p.activeWs == ws {
			_ = ws.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
		}
		p.wsMu.Unlock()
		return nil
	})

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			break
		}
		_ = ws.SetReadDeadline(time.Now().Add(generousIdle))

		var msg domain.WsTerminalMessage
		if err := json.Unmarshal(message, &msg); err == nil {
			switch msg.Type {
			case "input", "stdin":
				if p.stdinPipe != nil {
					_, _ = io.WriteString(p.stdinPipe, msg.Data)
				}
			case "resize":
				if msg.Cols > 0 && msg.Rows > 0 && p.session != nil {
					_ = p.session.WindowChange(msg.Rows, msg.Cols)
				}
			case "ping":
				_ = p.writeWs(domain.WsTerminalMessage{Type: "pong"})
			case "pong":
				// Heartbeat pong received from client
			case "disconnect":
				// User explicitly closed session (e.g. closed tab)
				p.closeSession()
				return
			case "auth", "reconnect":
				if msg.Cols > 0 && msg.Rows > 0 && p.session != nil {
					_ = p.session.WindowChange(msg.Rows, msg.Cols)
				}
				_ = p.writeWs(domain.WsTerminalMessage{Type: "connected", Message: "Terminal stream ready."})
			}
		} else {
			if p.stdinPipe != nil {
				_, _ = p.stdinPipe.Write(message)
			}
		}
	}

	// WebSocket disconnected (e.g. tab hidden, Cloudflare Tunnel timeout, or network blip)
	p.mu.Lock()
	if !p.closed {
		p.wsMu.Lock()
		if p.activeWs == ws {
			p.activeWs = nil
		}
		p.wsMu.Unlock()

		// Start a 15-minute grace period before terminating the remote SSH session
		if p.detachTimer != nil {
			p.detachTimer.Stop()
		}
		p.detachTimer = time.AfterFunc(15*time.Minute, func() {
			logger.Info("SSH", fmt.Sprintf("Detached terminal session grace period expired (15m): %s", p.key))
			p.closeSession()
		})
		logger.Info("SSH", fmt.Sprintf("Terminal session detached for %s (15m grace period started)", p.key))
	}
	p.mu.Unlock()
}
