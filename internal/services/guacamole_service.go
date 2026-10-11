package services

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"

	"github.com/gorilla/websocket"
)

type GuacamoleService struct {
	guacdHost string
	guacdPort string
	upgrader  websocket.Upgrader
}

func NewGuacamoleService() *GuacamoleService {
	host := os.Getenv("GUACD_HOST")
	if host == "" {
		host = "guacd"
	}
	port := os.Getenv("GUACD_PORT")
	if port == "" {
		port = "4822"
	}

	return &GuacamoleService{
		guacdHost: host,
		guacdPort: port,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  64 * 1024,
			WriteBufferSize: 64 * 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
			Subprotocols: []string{"guacamole"},
		},
	}
}

func (s *GuacamoleService) Upgrader() *websocket.Upgrader {
	return &s.upgrader
}

// EncodeInstruction formats opcode and args into the strict Guacamole protocol wire format:
// e.g., "6.select,3.rdp;"
func EncodeInstruction(opcode string, args ...string) string {
	var sb strings.Builder
	sb.WriteString(strconv.Itoa(len(opcode)))
	sb.WriteByte('.')
	sb.WriteString(opcode)
	for _, arg := range args {
		sb.WriteByte(',')
		sb.WriteString(strconv.Itoa(len(arg)))
		sb.WriteByte('.')
		sb.WriteString(arg)
	}
	sb.WriteByte(';')
	return sb.String()
}

// ReadInstruction parses one instruction from a bufio.Reader
func ReadInstruction(r *bufio.Reader) (opcode string, args []string, err error) {
	var elements []string
	for {
		lenStr, err := r.ReadString('.')
		if err != nil {
			return "", nil, err
		}
		lenStr = strings.TrimSuffix(lenStr, ".")
		length, err := strconv.Atoi(lenStr)
		if err != nil {
			return "", nil, fmt.Errorf("invalid length token '%s': %w", lenStr, err)
		}

		buf := make([]byte, length)
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", nil, err
		}
		elements = append(elements, string(buf))

		delim, err := r.ReadByte()
		if err != nil {
			return "", nil, err
		}
		if delim == ';' {
			break
		} else if delim != ',' {
			return "", nil, fmt.Errorf("unexpected element delimiter: %q", delim)
		}
	}
	if len(elements) == 0 {
		return "", nil, fmt.Errorf("empty instruction")
	}
	return elements[0], elements[1:], nil
}

// HandleTunnel establishes connection to guacd, performs handshake, and proxies instruction streams
func (s *GuacamoleService) HandleTunnel(ws *websocket.Conn, cfg *domain.RemoteHostConfig, proto, security string, width, height, dpi int) {
	defer ws.Close()

	if proto == "" {
		proto = "rdp"
	}
	proto = strings.ToLower(strings.TrimSpace(proto))
	if proto != "rdp" && proto != "vnc" {
		proto = "rdp"
	}

	if width <= 0 {
		width = 1920
	}
	if height <= 0 {
		height = 1080
	}
	if dpi <= 0 {
		dpi = 96
	}

	// Align display geometry to multiples of 8 for FreeRDP tile & scanline boundary stability
	if width%8 != 0 {
		width = ((width + 7) / 8) * 8
	}
	if height%8 != 0 {
		height = ((height + 7) / 8) * 8
	}

	// 1. Connect to guacd TCP daemon
	guacdAddr := net.JoinHostPort(s.guacdHost, s.guacdPort)
	guacdConn, err := net.DialTimeout("tcp", guacdAddr, 8*time.Second)
	if err != nil {
		logger.Error("Guacamole", fmt.Sprintf("Failed to connect to guacd at %s", guacdAddr), err)
		errMsg := EncodeInstruction("error", fmt.Sprintf("Guacamole daemon unreachable at %s: %v", guacdAddr, err), "512")
		_ = ws.WriteMessage(websocket.TextMessage, []byte(errMsg))
		return
	}
	defer guacdConn.Close()

	reader := bufio.NewReader(guacdConn)

	// 2. Send "select,<proto>;"
	selectInst := EncodeInstruction("select", proto)
	if _, err := guacdConn.Write([]byte(selectInst)); err != nil {
		logger.Error("Guacamole", "Failed to send select instruction", err)
		return
	}

	// 3. Read "args" from guacd
	argsOpcode, expectedArgs, err := ReadInstruction(reader)
	if err != nil {
		logger.Error("Guacamole", "Failed reading args from guacd", err)
		return
	}
	if argsOpcode != "args" {
		logger.Error("Guacamole", fmt.Sprintf("Expected args instruction from guacd, got: %s", argsOpcode), fmt.Errorf("unexpected opcode: %s", argsOpcode))
		return
	}

	// 4. Send client display size
	sizeInst := EncodeInstruction("size", strconv.Itoa(width), strconv.Itoa(height), strconv.Itoa(dpi))
	if _, err := guacdConn.Write([]byte(sizeInst)); err != nil {
		return
	}

	// 5. Send supported media formats
	audioInst := EncodeInstruction("audio", "audio/L16", "rate=44100,channels=2")
	_, _ = guacdConn.Write([]byte(audioInst))
	imageInst := EncodeInstruction("image", "image/png", "image/jpeg", "image/webp")
	_, _ = guacdConn.Write([]byte(imageInst))

	// 6. Map connection parameters
	paramMap := make(map[string]string)
	paramMap["hostname"] = cfg.Host
	paramMap["width"] = strconv.Itoa(width)
	paramMap["height"] = strconv.Itoa(height)
	paramMap["dpi"] = strconv.Itoa(dpi)

	port := 3389
	if proto == "vnc" {
		port = 5900
	}
	if cfg.Port > 0 && cfg.Port != 22 {
		port = cfg.Port
	}
	paramMap["port"] = strconv.Itoa(port)

	// Handle domain in username if present (e.g., "DOMAIN\user")
	username := cfg.Username
	if strings.Contains(username, "\\") {
		parts := strings.SplitN(username, "\\", 2)
		paramMap["domain"] = parts[0]
		username = parts[1]
	}
	if username != "" {
		paramMap["username"] = username
	}
	if cfg.Password != nil {
		paramMap["password"] = *cfg.Password
	}

	if proto == "rdp" {
		paramMap["ignore-cert"] = "true"
		paramMap["cert-ignore"] = "true"
		paramMap["disable-auth"] = "false"
		sec := strings.ToLower(strings.TrimSpace(security))
		if sec == "" {
			sec = "any"
		}
		paramMap["security"] = sec
		paramMap["resize-method"] = "display-update"
		// Critical anti-artifact & smoothness parameters:
		// Disabling bitmap & offscreen caching eliminates black/white rectangle tearing when client cache desyncs
		paramMap["disable-bitmap-caching"] = "true"
		paramMap["disable-offscreen-caching"] = "true"
		paramMap["disable-glyph-caching"] = "true"
		// 32-bit native color matches Windows DWM without CPU planar color conversion overhead
		paramMap["color-depth"] = "32"
		paramMap["enable-desktop-composition"] = "true"
		paramMap["enable-font-smoothing"] = "true"
		paramMap["enable-full-window-drag"] = "true"
		paramMap["enable-theming"] = "true"
		paramMap["enable-wallpaper"] = "false"
		paramMap["client-name"] = "Hephaestus-RDP"
	}

	// Build connect parameters in exact sequence expected by guacd args
	connectArgs := make([]string, len(expectedArgs))
	for i, argName := range expectedArgs {
		if strings.HasPrefix(argName, "VERSION_") {
			// Echo back protocol version supported by guacd (e.g. VERSION_1_5_0)
			connectArgs[i] = argName
		} else if val, exists := paramMap[argName]; exists {
			connectArgs[i] = val
		} else {
			connectArgs[i] = ""
		}
	}

	logger.Info("Guacamole", fmt.Sprintf("Initiating %s handshake to %s:%d (hostId: %s)", strings.ToUpper(proto), cfg.Host, port, cfg.ID))

	connectInst := EncodeInstruction("connect", connectArgs...)
	if _, err := guacdConn.Write([]byte(connectInst)); err != nil {
		logger.Error("Guacamole", "Failed to send connect instruction", err)
		errMsg := EncodeInstruction("error", fmt.Sprintf("Failed to send connect instruction to guacd: %v", err), "512")
		_ = ws.WriteMessage(websocket.TextMessage, []byte(errMsg))
		time.Sleep(100 * time.Millisecond)
		return
	}

	// 7. Read "ready" from guacd
	readyOpcode, readyArgs, err := ReadInstruction(reader)
	if err != nil {
		logger.Error("Guacamole", fmt.Sprintf("Failed reading ready instruction from guacd for %s:%d", cfg.Host, port), err)
		errMsg := EncodeInstruction("error", fmt.Sprintf("Connection failed or rejected by host %s:%d (RDP/VNC target unreachable or credentials invalid)", cfg.Host, port), "516")
		_ = ws.WriteMessage(websocket.TextMessage, []byte(errMsg))
		time.Sleep(100 * time.Millisecond)
		return
	}
	if readyOpcode != "ready" {
		logger.Warn("Guacamole", fmt.Sprintf("Expected ready, got %s: %v", readyOpcode, readyArgs))
		var errMsg string
		if readyOpcode == "error" && len(readyArgs) > 0 {
			code := "512"
			if len(readyArgs) > 1 {
				code = readyArgs[1]
			}
			errMsg = EncodeInstruction("error", readyArgs[0], code)
		} else {
			errMsg = EncodeInstruction("error", fmt.Sprintf("Remote desktop host %s:%d rejected connection (%s)", cfg.Host, port, readyOpcode), "512")
		}
		_ = ws.WriteMessage(websocket.TextMessage, []byte(errMsg))
		time.Sleep(100 * time.Millisecond)
		return
	}

	logger.Info("Guacamole", fmt.Sprintf("Connected successfully to %s:%d (protocol: %s, connectionId: %v)", cfg.Host, port, proto, readyArgs))

	// Forward ready instruction to the browser client WebSocket
	readyMsg := EncodeInstruction("ready", readyArgs...)
	if err := ws.WriteMessage(websocket.TextMessage, []byte(readyMsg)); err != nil {
		return
	}

	// 8. Bidirectional streaming loop between Browser WebSocket and guacd TCP socket
	var writeMu sync.Mutex
	safeWriteWs := func(msg []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return ws.WriteMessage(websocket.TextMessage, msg)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine A: guacd -> WebSocket (Screen display tiles & updates)
	go func() {
		defer wg.Done()
		defer ws.Close()
		defer guacdConn.Close()

		buf := make([]byte, 65536)
		for {
			n, err := reader.Read(buf)
			if err != nil {
				break
			}
			if n > 0 {
				if err := safeWriteWs(buf[:n]); err != nil {
					break
				}
			}
		}
	}()

	// Goroutine B: WebSocket -> guacd (Keyboard, Mouse, Sync, Display resize)
	go func() {
		defer wg.Done()
		defer ws.Close()
		defer guacdConn.Close()

		for {
			msgType, data, err := ws.ReadMessage()
			if err != nil {
				break
			}
			if msgType == websocket.TextMessage || msgType == websocket.BinaryMessage {
				if _, err := guacdConn.Write(data); err != nil {
					break
				}
			}
		}
	}()

	wg.Wait()
}
