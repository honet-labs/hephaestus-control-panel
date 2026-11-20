package services

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go-hephaestus/internal/core/domain"
	"golang.org/x/crypto/ssh"
)

func terminalSocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		accepted <- ws
		<-release
	}))
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		close(release)
		server.Close()
		t.Fatal(err)
	}
	var backend *websocket.Conn
	select {
	case backend = <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("WebSocket upgrade timed out")
	}
	t.Cleanup(func() { _ = client.Close(); _ = backend.Close(); close(release); server.Close() })
	return backend, client
}

func readTerminalMessage(t *testing.T, client *websocket.Conn) domain.WsTerminalMessage {
	t.Helper()
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg domain.WsTerminalMessage
	if err := client.ReadJSON(&msg); err != nil {
		t.Fatal(err)
	}
	return msg
}

func testPersistentTerminal(t *testing.T) *persistentSSHSession {
	t.Helper()
	service := NewWsTerminalService(nil)
	p := &persistentSSHSession{key: "test-user:host:tab", done: make(chan struct{}), service: service}
	service.sessions[p.key] = p
	t.Cleanup(p.closeSession)
	return p
}

func TestTerminalReconnectReplaysOnlyMissedOutput(t *testing.T) {
	p := testPersistentTerminal(t)
	_ = p.writeWs(domain.WsTerminalMessage{Type: "data", Data: "first\r\n"})
	ws1, client1 := terminalSocketPair(t)
	if !p.attach(ws1, 0) {
		t.Fatal("initial attach failed")
	}
	if msg := readTerminalMessage(t, client1); msg.Type != "connected" {
		t.Fatal(msg)
	}
	if msg := readTerminalMessage(t, client1); msg.Sequence != 1 || msg.Data != "first\r\n" {
		t.Fatal(msg)
	}
	p.detach(ws1)
	_ = p.writeWs(domain.WsTerminalMessage{Type: "data", Data: "while detached\r\n"})
	ws2, client2 := terminalSocketPair(t)
	if !p.attach(ws2, 1) {
		t.Fatal("reattach failed")
	}
	if msg := readTerminalMessage(t, client2); msg.Type != "connected" || msg.ResetSequence {
		t.Fatal(msg)
	}
	if msg := readTerminalMessage(t, client2); msg.Sequence != 2 || msg.Data != "while detached\r\n" {
		t.Fatal(msg)
	}
	_ = p.writeWs(domain.WsTerminalMessage{Type: "data", Data: "live\r\n"})
	if msg := readTerminalMessage(t, client2); msg.Sequence != 3 || msg.Data != "live\r\n" {
		t.Fatal("duplicate or missing replay", msg)
	}
}

func TestTerminalStaleDetachCannotExpireReplacement(t *testing.T) {
	p := testPersistentTerminal(t)
	ws1, _ := terminalSocketPair(t)
	ws2, _ := terminalSocketPair(t)
	if !p.attach(ws1, 0) {
		t.Fatal("attach failed")
	}
	p.detach(ws1)
	p.mu.Lock()
	oldEpoch := p.detachEpoch
	p.mu.Unlock()
	if !p.attach(ws2, 0) {
		t.Fatal("reattach failed")
	}
	p.detach(ws1)                    // the old read loop exits after the replacement has attached
	p.closeSessionAtDetach(oldEpoch) // simulate an already-fired stale timer
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.detachTimer != nil {
		t.Fatal("stale connection expired the live shell")
	}
	p.wsMu.Lock()
	defer p.wsMu.Unlock()
	if p.activeWs != ws2 {
		t.Fatal("replacement socket lost")
	}
}

func TestTerminalReplayIsBoundedAndReportsGap(t *testing.T) {
	p := testPersistentTerminal(t)
	chunk := strings.Repeat("x", 8192)
	for i := 0; i < 200; i++ {
		_ = p.writeWs(domain.WsTerminalMessage{Type: "data", Data: chunk})
	}
	if p.historyBytes > terminalHistoryLimit || p.history[0].Sequence <= 1 {
		t.Fatal("history not bounded")
	}
	ws, client := terminalSocketPair(t)
	// Drain concurrently: replay can exceed the socket send buffer.
	result := make(chan bool, 1)
	go func() { result <- p.attach(ws, 0) }()
	if msg := readTerminalMessage(t, client); msg.Type != "connected" {
		t.Fatal(msg)
	}
	if msg := readTerminalMessage(t, client); msg.Type != "historyGap" {
		t.Fatal("missing truncation notice", msg)
	}
	for i := 0; i < len(p.history); i++ {
		if msg := readTerminalMessage(t, client); msg.Type != "data" {
			t.Fatal(msg)
		}
	}
	if !<-result {
		t.Fatal("replay failed")
	}
}

func TestTerminalServerHeartbeatWorksWithoutBrowserTimers(t *testing.T) {
	p := testPersistentTerminal(t)
	ws, client := terminalSocketPair(t)
	if !p.attach(ws, 0) {
		t.Fatal("attach failed")
	}
	_ = readTerminalMessage(t, client)
	p.startWebSocketHeartbeat(10 * time.Millisecond)
	for i := 0; i < 3; i++ {
		if msg := readTerminalMessage(t, client); msg.Type != "ping" {
			t.Fatal(msg)
		}
	}
}

type terminalInputRecorder struct{ bytes.Buffer }

func (*terminalInputRecorder) Close() error { return nil }

func TestTerminalReattachDoesNotSubmitPartialCommand(t *testing.T) {
	p := testPersistentTerminal(t)
	recorder := &terminalInputRecorder{}
	p.stdinPipe = recorder
	ws, _ := terminalSocketPair(t)
	if !p.attach(ws, 0) {
		t.Fatal("attach failed")
	}
	if recorder.Len() != 0 {
		t.Fatal("reattach wrote unsolicited shell input")
	}
}

func TestTerminalRealSSHChannelSurvivesWebSocketReconnect(t *testing.T) {
	t.Setenv("SSH_KNOWN_HOSTS", filepath.Join(t.TempDir(), "known_hosts"))
	t.Setenv("SSH_HOST_KEY_POLICY", "tofu")
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	var shells atomic.Int32
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				config := &ssh.ServerConfig{PasswordCallback: func(_ ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
					if string(password) != "fixture-only" {
						return nil, fmt.Errorf("denied")
					}
					return nil, nil
				}}
				config.AddHostKey(signer)
				server, channels, requests, err := ssh.NewServerConn(conn, config)
				if err != nil {
					return
				}
				defer server.Close()
				go ssh.DiscardRequests(requests)
				for incoming := range channels {
					if incoming.ChannelType() != "session" {
						_ = incoming.Reject(ssh.UnknownChannelType, "session only")
						continue
					}
					channel, requests, err := incoming.Accept()
					if err != nil {
						continue
					}
					go func() {
						defer channel.Close()
						for request := range requests {
							_ = request.Reply(true, nil)
							if request.Type == "shell" {
								id := shells.Add(1)
								go func() {
									scanner := bufio.NewScanner(channel)
									for scanner.Scan() {
										_, _ = fmt.Fprintf(channel, "shell-%d:%s\r\n", id, scanner.Text())
									}
								}()
							}
						}
					}()
				}
			}()
		}
	}()
	host, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	password := "fixture-only"
	cfg := &domain.RemoteHostConfig{ID: "fixture", Host: host, Port: port, Username: "audit", AuthType: "password", Password: &password}
	service := NewWsTerminalService(&SSHService{})
	t.Cleanup(func() {
		service.sessionsMu.RLock()
		sessions := make([]*persistentSSHSession, 0, len(service.sessions))
		for _, p := range service.sessions {
			sessions = append(sessions, p)
		}
		service.sessionsMu.RUnlock()
		for _, p := range sessions {
			p.closeSession()
		}
	})
	exited := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		after, _ := strconv.ParseUint(r.URL.Query().Get("after"), 10, 64)
		service.HandleWebSocketSession(ws, cfg, 80, 24, 7, "same-tab", false, after)
		exited <- struct{}{}
	}))
	defer server.Close()
	url := "ws" + strings.TrimPrefix(server.URL, "http")
	first, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if msg := readTerminalMessage(t, first); msg.Type != "connected" || !msg.ResetSequence {
		t.Fatal(msg)
	}
	if err := first.WriteJSON(domain.WsTerminalMessage{Type: "input", Data: "before\n"}); err != nil {
		t.Fatal(err)
	}
	var last uint64
	var output strings.Builder
	for !strings.Contains(output.String(), "shell-1:before") {
		msg := readTerminalMessage(t, first)
		output.WriteString(msg.Data)
		last = msg.Sequence
	}
	_ = first.Close()
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("detach timed out")
	}
	second, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("%s?after=%d", url, last), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if msg := readTerminalMessage(t, second); msg.Type != "connected" || msg.ResetSequence {
		t.Fatal(msg)
	}
	if err := second.WriteJSON(domain.WsTerminalMessage{Type: "input", Data: "after\n"}); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	for !strings.Contains(output.String(), "shell-1:after") {
		msg := readTerminalMessage(t, second)
		output.WriteString(msg.Data)
	}
	if shells.Load() != 1 {
		t.Fatal("reconnect spawned a fresh remote shell")
	}
}
