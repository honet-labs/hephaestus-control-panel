package handlers

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
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
	remoteRepo      *repository.RemoteHostRepository
	sshService      *services.SSHService
	wsService       *services.WsTerminalService
	authService     *services.AuthService
	vpsService      *services.VpsService
	firewallService *services.FirewallService
}

func NewRemoteHostHandler(
	remoteRepo *repository.RemoteHostRepository,
	sshService *services.SSHService,
	wsService *services.WsTerminalService,
	authService *services.AuthService,
	vpsService *services.VpsService,
	firewallService *services.FirewallService,
) *RemoteHostHandler {
	return &RemoteHostHandler{
		remoteRepo:      remoteRepo,
		sshService:      sshService,
		wsService:       wsService,
		authService:     authService,
		vpsService:      vpsService,
		firewallService: firewallService,
	}
}

func getUserContext(c *gin.Context) (int, string) {
	var userID int
	var userRole string
	if val, exists := c.Get("userId"); exists {
		if id, ok := val.(int); ok {
			userID = id
		}
	}
	if val, exists := c.Get("userRole"); exists {
		if r, ok := val.(string); ok {
			userRole = r
		}
	}
	return userID, userRole
}

func (h *RemoteHostHandler) ensureAccess(c *gin.Context, hostID string, requiredPermission string) bool {
	userID, userRole := getUserContext(c)
	hasAccess, isOwner, perm, err := h.remoteRepo.CheckAccess(c.Request.Context(), hostID, userID, userRole)
	if err != nil || !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Access denied: you do not have permission to access this remote server",
		})
		return false
	}

	if requiredPermission == "manage" && !isOwner && !strings.EqualFold(userRole, "ADMIN") && perm != "manage" {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Access denied: full management permission is required for this action",
		})
		return false
	}
	return true
}

func (h *RemoteHostHandler) List(c *gin.Context) {
	userID, userRole := getUserContext(c)
	list, err := h.remoteRepo.List(c.Request.Context(), userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

func (h *RemoteHostHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	userID, userRole := getUserContext(c)
	cfg, err := h.remoteRepo.GetByID(c.Request.Context(), id, userID, userRole)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Host not found or access denied"})
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

	userID, userRole := getUserContext(c)

	if req.ID == "" {
		req.ID = fmt.Sprintf("rhc-%s", uuid.New().String()[:8])
	}
	if req.Port <= 0 {
		req.Port = 22
	}
	if req.GroupName == "" {
		req.GroupName = "Default"
	}

	if err := h.remoteRepo.Save(c.Request.Context(), req, userID, userRole); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Host saved successfully.", "data": req})
}

func (h *RemoteHostHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	userID, userRole := getUserContext(c)
	if err := h.remoteRepo.Delete(c.Request.Context(), id, userID, userRole); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": err.Error()})
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
	defer ws.Close()

	var userID int
	var userRole string
	var hostID string = queryHostID
	var isAuthenticated bool

	// 1. Try authentication via query token
	if queryToken != "" {
		user, err := h.authService.ValidateSession(c.Request.Context(), queryToken)
		if err == nil && user != nil {
			userID = user.ID
			userRole = user.Role
			isAuthenticated = true
		} else {
			_ = ws.WriteJSON(domain.WsTerminalMessage{Type: "error", Message: "Invalid or expired session token."})
			_ = ws.Close()
			return
		}
	}

	// 2. If not authenticated or hostID missing, wait for initial handshake message
	if !isAuthenticated || hostID == "" {
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
			if err != nil || user == nil {
				_ = ws.WriteJSON(domain.WsTerminalMessage{Type: "error", Message: "Invalid or expired session token."})
				_ = ws.Close()
				return
			}
			userID = user.ID
			userRole = user.Role
			isAuthenticated = true
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

	// Enforce strict authentication requirement
	if !isAuthenticated {
		_ = ws.WriteJSON(domain.WsTerminalMessage{Type: "error", Message: "Access denied. Valid authentication token required."})
		_ = ws.Close()
		return
	}

	if hostID == "" {
		_ = ws.WriteJSON(domain.WsTerminalMessage{Type: "error", Message: "Remote host config ID is required."})
		_ = ws.Close()
		return
	}

	// Multi-tenant Access Check for Terminal Session
	hasAccess, _, _, err := h.remoteRepo.CheckAccess(c.Request.Context(), hostID, userID, userRole)
	if err != nil || !hasAccess {
		_ = ws.WriteJSON(domain.WsTerminalMessage{Type: "error", Message: "Access denied. You do not have permission to access this remote server."})
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
	if !h.ensureAccess(c, hostID, "read") {
		return
	}

	remotePath := c.DefaultQuery("path", "/")
	files, err := h.sshService.SftpListDir(c.Request.Context(), hostID, remotePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": files, "currentPath": remotePath})
}

func (h *RemoteHostHandler) SftpUpload(c *gin.Context) {
	hostID := c.Param("id")
	if !h.ensureAccess(c, hostID, "manage") {
		return
	}

	targetDir := c.DefaultPostForm("path", "/")
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "file is required"})
		return
	}
	defer file.Close()

	remotePath := filepath.ToSlash(filepath.Join(targetDir, header.Filename))
	if err := h.sshService.SftpUpload(c.Request.Context(), hostID, remotePath, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "File uploaded successfully."})
}

func (h *RemoteHostHandler) SftpDownload(c *gin.Context) {
	hostID := c.Param("id")
	if !h.ensureAccess(c, hostID, "read") {
		return
	}

	remotePath := c.Query("path")
	if remotePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "path is required"})
		return
	}

	rc, size, err := h.sshService.SftpDownload(c.Request.Context(), hostID, remotePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	defer rc.Close()

	filename := filepath.Base(remotePath)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Content-Type", "application/octet-stream")
	if size > 0 {
		c.Header("Content-Length", strconv.FormatInt(size, 10))
	}

	_, _ = io.Copy(c.Writer, rc)
}

type SftpRemoteTransferReq struct {
	SrcHostID string `json:"srcHostId" binding:"required"`
	SrcPath   string `json:"srcPath" binding:"required"`
	DstHostID string `json:"dstHostId" binding:"required"`
	DstPath   string `json:"dstPath" binding:"required"`
}

func (h *RemoteHostHandler) SftpTransferRemote(c *gin.Context) {
	var req SftpRemoteTransferReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if !h.ensureAccess(c, req.SrcHostID, "read") {
		return
	}
	if !h.ensureAccess(c, req.DstHostID, "manage") {
		return
	}

	if err := h.sshService.SftpTransferRemoteToRemote(c.Request.Context(), req.SrcHostID, req.SrcPath, req.DstHostID, req.DstPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Server-to-Server file transfer completed successfully."})
}

// Telemetry & Systems Endpoints
func (h *RemoteHostHandler) GetMetrics(c *gin.Context) {
	hostID := c.Param("id")
	if !h.ensureAccess(c, hostID, "read") {
		return
	}

	metrics, err := h.vpsService.GetMetrics(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": metrics})
}

func (h *RemoteHostHandler) GetProcesses(c *gin.Context) {
	hostID := c.Param("id")
	if !h.ensureAccess(c, hostID, "read") {
		return
	}

	procs, err := h.vpsService.GetProcesses(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": procs})
}

func (h *RemoteHostHandler) KillProcess(c *gin.Context) {
	hostID := c.Param("id")
	if !h.ensureAccess(c, hostID, "manage") {
		return
	}

	pidStr := c.Param("pid")
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid PID"})
		return
	}

	if err := h.vpsService.KillProcess(c.Request.Context(), hostID, pid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Process terminated successfully."})
}

func (h *RemoteHostHandler) GetServices(c *gin.Context) {
	hostID := c.Param("id")
	if !h.ensureAccess(c, hostID, "read") {
		return
	}

	svcs, err := h.vpsService.GetServices(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": svcs})
}

func (h *RemoteHostHandler) ControlService(c *gin.Context) {
	hostID := c.Param("id")
	if !h.ensureAccess(c, hostID, "manage") {
		return
	}

	var req struct {
		ServiceName string `json:"serviceName" binding:"required"`
		Action      string `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "serviceName and action required"})
		return
	}

	out, err := h.vpsService.ControlService(c.Request.Context(), hostID, req.ServiceName, req.Action)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error(), "output": out})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "output": out})
}

func (h *RemoteHostHandler) GetNetworkInfo(c *gin.Context) {
	hostID := c.Param("id")
	if !h.ensureAccess(c, hostID, "read") {
		return
	}

	netInfo, err := h.vpsService.GetNetworkInfo(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": netInfo})
}

func (h *RemoteHostHandler) GetFirewallStatus(c *gin.Context) {
	hostID := c.Param("id")
	if !h.ensureAccess(c, hostID, "read") {
		return
	}

	res, err := h.firewallService.GetFirewallStatus(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": res})
}

func (h *RemoteHostHandler) AddFirewallRule(c *gin.Context) {
	hostID := c.Param("id")
	if !h.ensureAccess(c, hostID, "manage") {
		return
	}

	var rule domain.RemoteHostFirewallRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid firewall rule payload"})
		return
	}

	created, err := h.firewallService.AddFirewallRule(c.Request.Context(), hostID, rule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Firewall rule created and applied successfully.", "data": created})
}

func (h *RemoteHostHandler) DeleteFirewallRule(c *gin.Context) {
	hostID := c.Param("id")
	if !h.ensureAccess(c, hostID, "manage") {
		return
	}

	ruleID := c.Param("ruleId")
	if ruleID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "ruleId is required"})
		return
	}

	if err := h.firewallService.DeleteFirewallRule(c.Request.Context(), hostID, ruleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Firewall rule deleted successfully."})
}

func (h *RemoteHostHandler) ToggleFirewall(c *gin.Context) {
	hostID := c.Param("id")
	if !h.ensureAccess(c, hostID, "manage") {
		return
	}

	var req struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid toggle payload"})
		return
	}

	out, err := h.firewallService.ToggleFirewall(c.Request.Context(), hostID, req.Enable)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error(), "output": out})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Firewall status toggled successfully.", "output": out})
}

// ==================== SHARE ACCESS HANDLERS ====================

func (h *RemoteHostHandler) ListShares(c *gin.Context) {
	hostID := c.Param("id")
	userID, userRole := getUserContext(c)

	hasAccess, isOwner, _, err := h.remoteRepo.CheckAccess(c.Request.Context(), hostID, userID, userRole)
	if err != nil || !hasAccess || (!isOwner && !strings.EqualFold(userRole, "ADMIN")) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: only host owner or administrator can view shared access"})
		return
	}

	shares, err := h.remoteRepo.ListShares(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": shares})
}

func (h *RemoteHostHandler) AddShare(c *gin.Context) {
	hostID := c.Param("id")
	userID, userRole := getUserContext(c)

	hasAccess, isOwner, _, err := h.remoteRepo.CheckAccess(c.Request.Context(), hostID, userID, userRole)
	if err != nil || !hasAccess || (!isOwner && !strings.EqualFold(userRole, "ADMIN")) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: only host owner or administrator can share access"})
		return
	}

	var req struct {
		UserID     int    `json:"userId" binding:"required"`
		Permission string `json:"permission"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	if req.UserID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "You cannot share a server with yourself"})
		return
	}

	if err := h.remoteRepo.AddShare(c.Request.Context(), hostID, req.UserID, req.Permission, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Access granted successfully."})
}

func (h *RemoteHostHandler) DeleteShare(c *gin.Context) {
	hostID := c.Param("id")
	targetUserIDStr := c.Param("userId")
	targetUserID, err := strconv.Atoi(targetUserIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid user ID"})
		return
	}

	userID, userRole := getUserContext(c)
	hasAccess, isOwner, _, err := h.remoteRepo.CheckAccess(c.Request.Context(), hostID, userID, userRole)
	if err != nil || !hasAccess || (!isOwner && !strings.EqualFold(userRole, "ADMIN")) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: only host owner or administrator can revoke access"})
		return
	}

	if err := h.remoteRepo.DeleteShare(c.Request.Context(), hostID, targetUserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Access revoked successfully."})
}

func (h *RemoteHostHandler) ListAvailableUsers(c *gin.Context) {
	users, err := h.remoteRepo.ListAvailableUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": users})
}
