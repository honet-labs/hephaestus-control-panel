package handlers

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/repository"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  64 * 1024,
	WriteBufferSize: 64 * 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Origin checked in middleware or auth handshake
	},
}

type RemoteHostHandler struct {
	remoteRepo  *repository.RemoteHostRepository
	sshService  *services.SSHService
	wsService   *services.WsTerminalService
	authService *services.AuthService
}

func NewRemoteHostHandler(
	remoteRepo *repository.RemoteHostRepository,
	sshService *services.SSHService,
	wsService *services.WsTerminalService,
	authService *services.AuthService,
) *RemoteHostHandler {
	return &RemoteHostHandler{
		remoteRepo:  remoteRepo,
		sshService:  sshService,
		wsService:   wsService,
		authService: authService,
	}
}

func (h *RemoteHostHandler) List(c *gin.Context) {
	list, err := h.remoteRepo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

func (h *RemoteHostHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	cfg, err := h.remoteRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Host not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": cfg})
}

func (h *RemoteHostHandler) Save(c *gin.Context) {
	var req domain.RemoteHostConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if req.ID == "" {
		req.ID = fmt.Sprintf("rhc-%s", uuid.New().String()[:8])
	}
	if req.Port <= 0 {
		req.Port = 22
	}
	if req.GroupName == "" {
		req.GroupName = "Default"
	}

	if err := h.remoteRepo.Save(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Host saved successfully.", "data": req})
}

func (h *RemoteHostHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.remoteRepo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Host deleted successfully."})
}

func (h *RemoteHostHandler) TestConnection(c *gin.Context) {
	var req domain.RemoteHostConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}
	if req.Port <= 0 {
		req.Port = 22
	}

	success, msg := h.sshService.TestConnection(&req)
	c.JSON(http.StatusOK, gin.H{"success": success, "message": msg})
}

// WebSocket Terminal Stream Endpoint: /ws/remote-host
func (h *RemoteHostHandler) HandleWebSocketTerminal(c *gin.Context) {
	cols, _ := strconv.Atoi(c.DefaultQuery("cols", "80"))
	rows, _ := strconv.Atoi(c.DefaultQuery("rows", "24"))
	queryHostID := c.Query("hostId")
	queryToken := c.Query("token")

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("WS", "WebSocket upgrade failed", err)
		return
	}

	var userID int = 1
	var hostID string = queryHostID

	if queryToken != "" {
		user, err := h.authService.ValidateSession(c.Request.Context(), queryToken)
		if err == nil && user != nil {
			userID = user.ID
		}
	}

	if hostID == "" {
		// Wait for initial handshake message if host was not in query string
		_ = ws.SetReadDeadline(time.Now().Add(10 * time.Second))
		var authMsg domain.WsTerminalMessage
		if err := ws.ReadJSON(&authMsg); err != nil {
			_ = ws.WriteJSON(domain.WsTerminalMessage{Type: "error", Message: "Authentication required or timed out."})
			_ = ws.Close()
			return
		}
		_ = ws.SetReadDeadline(time.Time{}) // Clear deadline

		if authMsg.Token != "" {
			user, err := h.authService.ValidateSession(c.Request.Context(), authMsg.Token)
			if err != nil {
				_ = ws.WriteJSON(domain.WsTerminalMessage{Type: "error", Message: "Invalid or expired session token."})
				_ = ws.Close()
				return
			}
			userID = user.ID
		}

		if authMsg.HostConfigID != "" {
			hostID = authMsg.HostConfigID
		} else if authMsg.HostID != "" {
			hostID = authMsg.HostID
		}
		if authMsg.Cols > 0 {
			cols = authMsg.Cols
		}
		if authMsg.Rows > 0 {
			rows = authMsg.Rows
		}
	}

	if hostID == "" {
		_ = ws.WriteJSON(domain.WsTerminalMessage{Type: "error", Message: "Remote host config ID is required."})
		_ = ws.Close()
		return
	}

	cfg, err := h.remoteRepo.GetRawByID(c.Request.Context(), hostID)
	if err != nil {
		_ = ws.WriteJSON(domain.WsTerminalMessage{Type: "error", Message: fmt.Sprintf("Remote host '%s' not found.", hostID)})
		_ = ws.Close()
		return
	}

	h.wsService.HandleWebSocketSession(ws, cfg, cols, rows, userID)
}

// SFTP Endpoints
func (h *RemoteHostHandler) SftpList(c *gin.Context) {
	hostID := c.Param("id")
	remotePath := c.DefaultQuery("path", "/")

	files, err := h.sshService.SftpListDir(c.Request.Context(), hostID, remotePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": files})
}

func (h *RemoteHostHandler) SftpUpload(c *gin.Context) {
	hostID := c.Param("id")
	remotePath := c.Query("path")
	if remotePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Query param 'path' is required"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "No file uploaded"})
		return
	}
	defer file.Close()

	targetPath := remotePath
	if targetPath == "" || targetPath == "/" {
		targetPath = "/" + header.Filename
	}

	if err := h.sshService.SftpUpload(c.Request.Context(), hostID, targetPath, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("File '%s' uploaded successfully.", header.Filename)})
}

func (h *RemoteHostHandler) SftpDownload(c *gin.Context) {
	hostID := c.Param("id")
	remotePath := c.Query("path")
	if remotePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Path required"})
		return
	}

	reader, size, err := h.sshService.SftpDownload(c.Request.Context(), hostID, remotePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	defer reader.Close()

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(remotePath)))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", size))
	_, _ = io.Copy(c.Writer, reader)
}
