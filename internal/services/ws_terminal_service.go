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
	key          string
	client       *ssh.Client
	session      *ssh.Session
	stdinPipe    io.WriteCloser
	stdoutPipe   io.Reader
	stderrPipe   io.Reader
	cols         int
	rows         int
	activeWs     *websocket.Conn
	wsMu         sync.Mutex
	detachTimer  *time.Timer
	closed       bool
	done         chan struct{}
	mu           sync.Mutex
	service      *WsTerminalService
	detachEpoch  uint64                     // protected by mu; prevents stale detach callbacks closing an attached shell
	history      []domain.WsTerminalMessage // protected by wsMu, never persisted to disk
	historyBytes int
	sequence     uint64
}

const terminalHistoryLimit = 1 << 20 // bounded output per live shell (1 MiB)

func (p *persistentSSHSession) writeWs(msg domain.WsTerminalMessage) error {
	p.wsMu.Lock()
	defer p.wsMu.Unlock()
	if msg.Type == "data" {
		p.sequence++
		msg.Sequence = p.sequence
		retained := msg
		if len(retained.Data) > terminalHistoryLimit {
			retained.Data = retained.Data[len(retained.Data)-terminalHistoryLimit:]
		}
		p.history = append(p.history, retained)
		p.historyBytes += len(retained.Data)
		for p.historyBytes > terminalHistoryLimit && len(p.history) > 1 {
			p.historyBytes -= len(p.history[0].Data)
			p.history[0] = domain.WsTerminalMessage{}
			p.history = p.history[1:]
		}
	}
	if p.activeWs == nil {
		return nil
	}
	_ = p.activeWs.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return p.activeWs.WriteJSON(msg)
}

func (p *persistentSSHSession) closeSession() {
	p.closeSessionAtDetach(0)
}

func (p *persistentSSHSession) closeSessionAtDetach(epoch uint64) {
	p.mu.Lock()
	if p.closed || (epoch != 0 && epoch != p.detachEpoch) {
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

	p.wsMu.Lock()
	if p.activeWs != nil {
		_ = p.activeWs.Close()
	}
	p.activeWs = nil
	p.history = nil
	p.historyBytes = 0
	p.wsMu.Unlock()

	p.service.removeSession(p.key, p)
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
	// SSH writes can block on a broken network. They must never stop the
	// independent WebSocket heartbeat used by proxies and background tabs.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-p.done:
				return
			case <-ticker.C:
				if _, _, err := p.client.SendRequest("keepalive@openssh.com", false, nil); err != nil {
					p.closeSession()
					return
				}
			}
		}
	}()
	p.startWebSocketHeartbeat(10 * time.Second)
}

func (p *persistentSSHSession) startWebSocketHeartbeat(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-p.done:
				return
			case <-ticker.C:
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

func (s *WsTerminalService) removeSession(key string, expected *persistentSSHSession) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	if current, ok := s.sessions[key]; ok && current == expected {
		delete(s.sessions, key)
	}
}

func (s *WsTerminalService) HandleWebSocketSession(ws *websocket.Conn, cfg *domain.RemoteHostConfig, initialCols, initialRows int, userID int, sessionID string, forceNew bool, cursor ...uint64) {
	var afterSequence uint64
	if len(cursor) > 0 {
		afterSequence = cursor[0]
	}
	if sessionID == "" {
		sessionID = cfg.ID
	}
	sessionKey := fmt.Sprintf("%d:%s:%s", userID, cfg.ID, sessionID)

	s.sessionsMu.Lock()
	pSess, exists := s.sessions[sessionKey]
	if exists {
		pSess.mu.Lock()
		isHealthy := !pSess.closed
		pSess.mu.Unlock()

		if forceNew || !isHealthy {
			if !isHealthy {
				logger.Warn("SSH", fmt.Sprintf("Terminal session %s is unresponsive or dead, recycling for fresh connection", sessionKey))
			} else {
				logger.Info("SSH", fmt.Sprintf("Force closing stale terminal session for clean reconnect: %s", sessionKey))
			}
			delete(s.sessions, sessionKey)
			s.sessionsMu.Unlock()

			// Safely close old session asynchronously so no deadlock or delay occurs
			go pSess.closeSession()
		} else {
			// Re-attach to existing live session
			s.sessionsMu.Unlock()
			if !pSess.attach(ws, afterSequence) {
				pSess.detach(ws)
				_ = ws.Close()
				return
			}
			// Never inject Enter on reattach: it can execute a half-typed command.
			s.runWsReadLoop(pSess, ws)
			return
		}
	} else {
		s.sessionsMu.Unlock()
	}

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
	if cols < 20 {
		cols = 80
	}
	rows := initialRows
	if rows < 4 {
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

	_ = pSess.writeWs(domain.WsTerminalMessage{
		Type:          "connected",
		ResetSequence: true,
		Message:       fmt.Sprintf("Connected to %s (%s@%s:%d)", cfg.Name, cfg.Username, cfg.Host, cfg.Port),
	})
	pSess.startOutputPipes()
	pSess.startKeepAlive()

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
		p.wsMu.Lock()
		isCurrent := p.activeWs == ws
		p.wsMu.Unlock()
		if !isCurrent {
			return
		}

		var msg domain.WsTerminalMessage
		if err := json.Unmarshal(message, &msg); err == nil {
			switch msg.Type {
			case "input", "stdin":
				if p.stdinPipe != nil {
					if _, err := io.WriteString(p.stdinPipe, msg.Data); err != nil {
						logger.Warn("SSH", fmt.Sprintf("Failed to write to stdin pipe (%v), recycling dead session %s", err, p.key))
						p.closeSession()
						_ = p.writeWs(domain.WsTerminalMessage{Type: "disconnected", Message: "SSH session connection terminated."})
						return
					}
				}
			case "resize":
				if msg.Cols >= 20 && msg.Rows >= 4 && p.session != nil {
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
				if msg.Cols >= 20 && msg.Rows >= 4 && p.session != nil {
					_ = p.session.WindowChange(msg.Rows, msg.Cols)
				}
				_ = p.writeWs(domain.WsTerminalMessage{Type: "connected", Message: "Terminal stream ready."})
			}
		} else {
			if p.stdinPipe != nil {
				if _, err := p.stdinPipe.Write(message); err != nil {
					logger.Warn("SSH", fmt.Sprintf("Failed to write raw bytes to stdin pipe (%v), recycling dead session %s", err, p.key))
					p.closeSession()
					_ = p.writeWs(domain.WsTerminalMessage{Type: "disconnected", Message: "SSH session connection terminated."})
					return
				}
			}
		}
	}

	// WebSocket disconnected (e.g. tab hidden, Cloudflare Tunnel timeout, or network blip)
	p.detach(ws)
}

func (p *persistentSSHSession) attach(ws *websocket.Conn, after uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return false
	}
	p.detachEpoch++
	if p.detachTimer != nil {
		p.detachTimer.Stop()
		p.detachTimer = nil
	}
	p.wsMu.Lock()
	defer p.wsMu.Unlock()
	if p.activeWs != nil && p.activeWs != ws {
		_ = p.activeWs.Close()
	}
	p.activeWs = ws
	_ = ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := ws.WriteJSON(domain.WsTerminalMessage{Type: "connected", Message: "Resumed existing shell"}); err != nil {
		return false
	}
	if len(p.history) > 0 && p.history[0].Sequence > after+1 {
		_ = ws.WriteJSON(domain.WsTerminalMessage{Type: "historyGap", Message: "Older terminal output exceeded the 1 MiB replay limit."})
	}
	for _, msg := range p.history {
		if msg.Sequence > after {
			if err := ws.WriteJSON(msg); err != nil {
				return false
			}
		}
	}
	return true
}

func (p *persistentSSHSession) detach(ws *websocket.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.wsMu.Lock()
	defer p.wsMu.Unlock()
	// A stale read loop must not schedule expiry for a replacement connection.
	if p.closed || p.activeWs != ws {
		return
	}
	p.activeWs = nil
	p.detachEpoch++
	epoch := p.detachEpoch
	if p.detachTimer != nil {
		p.detachTimer.Stop()
	}
	p.detachTimer = time.AfterFunc(15*time.Minute, func() { p.closeSessionAtDetach(epoch) })
}
