package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/repository"
	"go-hephaestus/internal/services"

	"github.com/gin-gonic/gin"
)

type DockerHandler struct {
	dockerService *services.DockerService
	dockerRepo    *repository.DockerRepository
}

func NewDockerHandler(
	dockerService *services.DockerService,
	dockerRepo *repository.DockerRepository,
) *DockerHandler {
	return &DockerHandler{
		dockerService: dockerService,
		dockerRepo:    dockerRepo,
	}
}

// -------------------------------------------------------------
// Connection Handlers
// -------------------------------------------------------------

func (h *DockerHandler) ListConnections(c *gin.Context) {
	connections, err := h.dockerRepo.ListConnections(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to list Docker connections",
			"details": err.Error(),
		})
		return
	}

	// Strip SSH passwords before returning and populate Driver alias
	for i := range connections {
		connections[i].SSHPassword = nil
		if connections[i].HostType == "local" {
			connections[i].Driver = "socket"
		} else {
			connections[i].Driver = connections[i].HostType
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    connections,
	})
}

func (h *DockerHandler) SaveConnection(c *gin.Context) {
	var input struct {
		ID           string  `json:"id"`
		Name         string  `json:"name" binding:"required"`
		HostType     string  `json:"hostType"` // local, ssh, tcp
		Driver       string  `json:"driver"`   // alias: socket, ssh, tcp
		SocketPath   string  `json:"socketPath"`
		TcpURL       string  `json:"tcpUrl"`
		TcpHost      string  `json:"tcpHost"`
		TcpPort      *int    `json:"tcpPort"`
		RemoteHostID *string `json:"remoteHostId"`
		SSHHost      *string `json:"sshHost"`
		SSHPort      *int    `json:"sshPort"`
		SSHUser      *string `json:"sshUser"`
		SSHAuth      *string `json:"sshAuth"`
		SSHPassword  *string `json:"sshPassword"`
		SSHKey       *string `json:"sshKey"`
		IsActive     *bool   `json:"isActive"`
		IsDefault    *bool   `json:"isDefault"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Name and Driver/HostType are required",
		})
		return
	}

	// Reconcile HostType vs Driver
	hostType := input.HostType
	if hostType == "" {
		if input.Driver == "socket" {
			hostType = "local"
		} else if input.Driver != "" {
			hostType = input.Driver
		} else {
			hostType = "local"
		}
	}

	// Reconcile TcpURL
	tcpURL := input.TcpURL
	if tcpURL == "" && input.TcpHost != "" {
		port := 2375
		if input.TcpPort != nil && *input.TcpPort > 0 {
			port = *input.TcpPort
		}
		tcpURL = "http://" + input.TcpHost + ":" + strconv.Itoa(port)
	}

	socketPath := input.SocketPath
	if socketPath == "" && hostType == "local" {
		socketPath = "/var/run/docker.sock"
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}
	isDefault := false
	if input.IsDefault != nil {
		isDefault = *input.IsDefault
	}

	if input.RemoteHostID != nil && strings.TrimSpace(*input.RemoteHostID) == "" {
		input.RemoteHostID = nil
	}
	if input.SSHHost != nil && strings.TrimSpace(*input.SSHHost) == "" {
		input.SSHHost = nil
	}
	if input.SSHUser != nil && strings.TrimSpace(*input.SSHUser) == "" {
		input.SSHUser = nil
	}
	if input.SSHPassword != nil && strings.TrimSpace(*input.SSHPassword) == "" {
		input.SSHPassword = nil
	}
	if input.SSHKey != nil && strings.TrimSpace(*input.SSHKey) == "" {
		input.SSHKey = nil
	}

	driver := input.Driver
	if driver == "" {
		if hostType == "local" {
			driver = "socket"
		} else {
			driver = hostType
		}
	}

	conn := domain.DockerConnection{
		ID:           input.ID,
		Name:         input.Name,
		HostType:     hostType,
		Driver:       driver,
		SocketPath:   socketPath,
		TcpURL:       tcpURL,
		RemoteHostID: input.RemoteHostID,
		SSHHost:      input.SSHHost,
		SSHPort:      input.SSHPort,
		SSHUser:      input.SSHUser,
		SSHAuth:      input.SSHAuth,
		SSHPassword:  input.SSHPassword,
		SSHKey:       input.SSHKey,
		IsActive:     isActive,
		IsDefault:    isDefault,
	}

	saved, err := h.dockerRepo.SaveConnection(c.Request.Context(), conn)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to save Docker connection",
			"details": err.Error(),
		})
		return
	}

	saved.SSHPassword = nil
	if saved.HostType == "local" {
		saved.Driver = "socket"
	} else {
		saved.Driver = saved.HostType
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Docker connection saved successfully",
		"data":    saved,
	})
}

func (h *DockerHandler) TestConnection(c *gin.Context) {
	var input struct {
		ID           string  `json:"id"`
		Name         string  `json:"name"`
		HostType     string  `json:"hostType"`
		Driver       string  `json:"driver"`
		SocketPath   string  `json:"socketPath"`
		TcpURL       string  `json:"tcpUrl"`
		TcpHost      string  `json:"tcpHost"`
		TcpPort      *int    `json:"tcpPort"`
		RemoteHostID *string `json:"remoteHostId"`
		SSHHost      *string `json:"sshHost"`
		SSHPort      *int    `json:"sshPort"`
		SSHUser      *string `json:"sshUser"`
		SSHAuth      *string `json:"sshAuth"`
		SSHPassword  *string `json:"sshPassword"`
		SSHKey       *string `json:"sshKey"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid test connection payload",
		})
		return
	}

	hostType := input.HostType
	if hostType == "" {
		if input.Driver == "socket" {
			hostType = "local"
		} else if input.Driver != "" {
			hostType = input.Driver
		} else {
			hostType = "local"
		}
	}

	tcpURL := input.TcpURL
	if tcpURL == "" && input.TcpHost != "" {
		port := 2375
		if input.TcpPort != nil && *input.TcpPort > 0 {
			port = *input.TcpPort
		}
		tcpURL = "http://" + input.TcpHost + ":" + strconv.Itoa(port)
	}

	socketPath := input.SocketPath
	if socketPath == "" && hostType == "local" {
		socketPath = "/var/run/docker.sock"
	}

	conn := domain.DockerConnection{
		ID:           input.ID,
		Name:         input.Name,
		HostType:     hostType,
		Driver:       input.Driver,
		SocketPath:   socketPath,
		TcpURL:       tcpURL,
		RemoteHostID: input.RemoteHostID,
		SSHHost:      input.SSHHost,
		SSHPort:      input.SSHPort,
		SSHUser:      input.SSHUser,
		SSHAuth:      input.SSHAuth,
		SSHPassword:  input.SSHPassword,
		SSHKey:       input.SSHKey,
	}

	// If testing existing connection without re-sending password
	if conn.ID != "" && conn.SSHPassword == nil {
		existing, _ := h.dockerRepo.GetConnectionByID(c.Request.Context(), conn.ID)
		if existing != nil {
			conn.SSHPassword = existing.SSHPassword
			conn.SSHKey = existing.SSHKey
		}
	}

	ok, msg, info := h.dockerService.TestConnection(c.Request.Context(), &conn)
	if !ok {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   msg,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": msg,
		"data":    info,
	})
}

func (h *DockerHandler) DeleteConnection(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Connection ID is required"})
		return
	}

	if err := h.dockerRepo.DeleteConnection(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete Docker connection",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Docker connection deleted successfully",
	})
}

// -------------------------------------------------------------
// Container Handlers
// -------------------------------------------------------------

// isOwnerOrAdmin checks if the requesting user is the owner of the container or an administrator
func (h *DockerHandler) isOwnerOrAdmin(c *gin.Context, connectionID, containerID string) (bool, *domain.User, *domain.DockerContainer) {
	userVal, exists := c.Get("user")
	var u *domain.User
	if exists {
		u, _ = userVal.(*domain.User)
	}

	containers, err := h.dockerService.ListContainers(c.Request.Context(), connectionID, true)
	if err != nil {
		return false, u, nil
	}

	var target *domain.DockerContainer
	for i := range containers {
		if containers[i].ID == containerID || (len(containerID) >= 8 && strings.HasPrefix(containers[i].ID, containerID)) {
			target = &containers[i]
			break
		}
	}

	if target == nil {
		return false, u, nil
	}

	if u != nil && u.IsAdmin() {
		return true, u, target
	}

	if u != nil {
		isOwner := (target.UserID != nil && *target.UserID == u.ID) ||
			(target.OwnerUsername != "" && strings.EqualFold(target.OwnerUsername, u.Username))
		if isOwner {
			return true, u, target
		}
	}

	return false, u, target
}

// canAccessContainer checks if the current user has access to a container
func (h *DockerHandler) canAccessContainer(c *gin.Context, connectionID, containerID, requiredPermission string) bool {
	userVal, exists := c.Get("user")
	if !exists {
		return true // Fallback if no user context
	}
	u, ok := userVal.(*domain.User)
	if !ok || u == nil {
		return true
	}
	if u.IsAdmin() {
		return true
	}

	containers, err := h.dockerService.ListContainers(c.Request.Context(), connectionID, true)
	if err != nil {
		return true
	}
	for _, ct := range containers {
		if ct.ID == containerID || (len(containerID) >= 8 && strings.HasPrefix(ct.ID, containerID)) {
			if ct.Visibility == "private" {
				// Creator has full control
				if ct.UserID != nil && *ct.UserID == u.ID {
					return true
				}
				if ct.OwnerUsername != "" && strings.EqualFold(ct.OwnerUsername, u.Username) {
					return true
				}

				// Check if shared with current user
				shares, err := h.dockerRepo.ListShares(c.Request.Context(), connectionID, ct.ID)
				if err == nil {
					for _, s := range shares {
						if s.UserID == u.ID {
							if requiredPermission == "read" {
								return true
							}
							return s.Permission == "manage"
						}
					}
				}
				return false
			}
			return true
		}
	}
	return true
}

func (h *DockerHandler) ListContainers(c *gin.Context) {
	connectionID := c.Query("connectionId")
	all := c.Query("all") == "true" || c.Query("all") == "1"

	containers, err := h.dockerService.ListContainers(c.Request.Context(), connectionID, all)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   err.Error(),
			"data":    []domain.DockerContainer{},
		})
		return
	}

	// Fetch all shares for this connection in one batch
	sharesMap, _ := h.dockerRepo.ListContainerSharesMap(c.Request.Context(), connectionID)

	// Filter and decorate based on current authenticated user
	userVal, hasUser := c.Get("user")
	var currentUser *domain.User
	if hasUser {
		currentUser, _ = userVal.(*domain.User)
	}

	filtered := make([]domain.DockerContainer, 0, len(containers))
	for _, ct := range containers {
		isOwner := false
		if currentUser != nil {
			if ct.UserID != nil && *ct.UserID == currentUser.ID {
				isOwner = true
			} else if ct.OwnerUsername != "" && strings.EqualFold(ct.OwnerUsername, currentUser.Username) {
				isOwner = true
			}
		}
		ct.IsOwner = isOwner

		// Attach shares info
		shares := sharesMap[ct.ID]
		if len(shares) == 0 && len(ct.ID) >= 12 {
			shares = sharesMap[ct.ID[:12]]
		}
		ct.SharesCount = len(shares)

		// Determine user permission on this container:
		// "owner", "manage", "read", "public"
		userPerm := ""
		if currentUser != nil && (currentUser.IsAdmin() || isOwner) {
			userPerm = "owner"
		} else {
			if currentUser != nil {
				for _, s := range shares {
					if s.UserID == currentUser.ID {
						userPerm = s.Permission // "manage" or "read"
						break
					}
				}
			}
			if userPerm == "" && (ct.Visibility == "public" || ct.Visibility == "") {
				userPerm = "public"
			}
		}
		ct.UserPermission = userPerm

		// Visibility logic:
		// 1. Superadmin / Admin sees all containers
		// 2. Container creator always sees their own container
		// 3. Containers explicitly shared with user (read or manage)
		// 4. Public containers
		if currentUser != nil && currentUser.IsAdmin() {
			filtered = append(filtered, ct)
		} else if isOwner || userPerm == "read" || userPerm == "manage" {
			filtered = append(filtered, ct)
		} else if ct.Visibility == "public" || ct.Visibility == "" {
			filtered = append(filtered, ct)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"total":   len(filtered),
		"data":    filtered,
	})
}

// GetContainer retrieves details of a specific container
func (h *DockerHandler) GetContainer(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

	if !h.canAccessContainer(c, connectionID, id, "read") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied. This is a private container."})
		return
	}

	containers, err := h.dockerService.ListContainers(c.Request.Context(), connectionID, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	for _, ct := range containers {
		if ct.ID == id || (len(id) >= 8 && strings.HasPrefix(ct.ID, id)) {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": ct})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Container not found"})
}

func (h *DockerHandler) StartContainer(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

	if !h.canAccessContainer(c, connectionID, id, "manage") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: you need manage permission for this container"})
		return
	}

	if err := h.dockerService.StartContainer(c.Request.Context(), connectionID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Container started successfully"})
}

func (h *DockerHandler) StopContainer(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

	if !h.canAccessContainer(c, connectionID, id, "manage") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: you need manage permission for this container"})
		return
	}

	if err := h.dockerService.StopContainer(c.Request.Context(), connectionID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Container stopped successfully"})
}

func (h *DockerHandler) RestartContainer(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

	if !h.canAccessContainer(c, connectionID, id, "manage") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: you need manage permission for this container"})
		return
	}

	if err := h.dockerService.RestartContainer(c.Request.Context(), connectionID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Container restarted successfully"})
}

func (h *DockerHandler) PauseContainer(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

	if !h.canAccessContainer(c, connectionID, id, "manage") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: you need manage permission for this container"})
		return
	}

	if err := h.dockerService.PauseContainer(c.Request.Context(), connectionID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Container paused successfully"})
}

func (h *DockerHandler) UnpauseContainer(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

	if !h.canAccessContainer(c, connectionID, id, "manage") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: you need manage permission for this container"})
		return
	}

	if err := h.dockerService.UnpauseContainer(c.Request.Context(), connectionID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Container unpaused successfully"})
}

func (h *DockerHandler) RemoveContainer(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")
	force := c.Query("force") == "true" || c.Query("force") == "1"

	isOwnerOrAdmin, _, target := h.isOwnerOrAdmin(c, connectionID, id)
	if target != nil && target.Visibility == "private" && !isOwnerOrAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: only the container creator or an Administrator can remove this private container"})
		return
	}
	if !h.canAccessContainer(c, connectionID, id, "manage") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: you need manage permission for this container"})
		return
	}

	if err := h.dockerService.RemoveContainer(c.Request.Context(), connectionID, id, force); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Container removed successfully"})
}

func (h *DockerHandler) GetContainerLogs(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")
	tail, _ := strconv.Atoi(c.DefaultQuery("tail", "200"))

	if !h.canAccessContainer(c, connectionID, id, "read") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied to private container logs"})
		return
	}

	logs, err := h.dockerService.GetContainerLogs(c.Request.Context(), connectionID, id, tail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"logs":    logs,
		"data": gin.H{
			"logs": logs,
		},
	})
}

func (h *DockerHandler) GetContainerStats(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

	if !h.canAccessContainer(c, connectionID, id, "read") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied to private container stats"})
		return
	}

	stats, err := h.dockerService.GetContainerStats(c.Request.Context(), connectionID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

func (h *DockerHandler) DeployContainer(c *gin.Context) {
	connectionID := c.Query("connectionId")
	var req domain.DeployContainerRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid container deployment specification"})
		return
	}

	// Inject creator user identity
	if userVal, exists := c.Get("user"); exists {
		if u, ok := userVal.(*domain.User); ok {
			req.UserID = &u.ID
			req.OwnerUsername = u.Username
		}
	}

	id, err := h.dockerService.DeployContainer(c.Request.Context(), connectionID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "Container deployed successfully",
		"containerId": id,
	})
}

func (h *DockerHandler) InspectContainer(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

	if !h.canAccessContainer(c, connectionID, id, "read") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied to private container"})
		return
	}

	details, err := h.dockerService.InspectContainer(c.Request.Context(), connectionID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    details,
	})
}

func (h *DockerHandler) EditContainer(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")
	var req domain.DeployContainerRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid container configuration: " + err.Error()})
		return
	}

	if !h.canAccessContainer(c, connectionID, id, "manage") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: you need manage permission to edit this container"})
		return
	}

	// Preserve owner info if not specified
	if userVal, exists := c.Get("user"); exists {
		if u, ok := userVal.(*domain.User); ok {
			if req.UserID == nil {
				req.UserID = &u.ID
				req.OwnerUsername = u.Username
			}
		}
	}

	newID, err := h.dockerService.EditContainer(c.Request.Context(), connectionID, id, req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if strings.Contains(strings.ToLower(err.Error()), "running") {
			statusCode = http.StatusBadRequest
		}
		c.JSON(statusCode, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "Container updated and recreated successfully",
		"containerId": newID,
	})
}

// UpdateVisibility allows changing the visibility of a container
func (h *DockerHandler) UpdateVisibility(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

	var input struct {
		Visibility string `json:"visibility" binding:"required"`
		Name       string `json:"name"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Visibility ('public' or 'private') is required"})
		return
	}

	isOwnerOrAdmin, u, target := h.isOwnerOrAdmin(c, connectionID, id)
	if target == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Container not found"})
		return
	}

	if !isOwnerOrAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Only the container creator or an Administrator can change container visibility"})
		return
	}

	cleanName := target.Name
	if input.Name != "" {
		cleanName = input.Name
	}

	var userID *int
	username := target.OwnerUsername
	if target.UserID != nil {
		userID = target.UserID
	} else if u != nil {
		userID = &u.ID
		username = u.Username
	}

	err := h.dockerService.UpdateContainerVisibility(c.Request.Context(), connectionID, target.ID, input.Visibility, cleanName, userID, username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    fmt.Sprintf("Container visibility changed to %s", input.Visibility),
		"visibility": input.Visibility,
	})
}

// -------------------------------------------------------------
// Container Sharing Handlers (Portainer-style granular access)
// -------------------------------------------------------------

// ListShares returns all access grants for a container
func (h *DockerHandler) ListShares(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

	isOwnerOrAdmin, _, target := h.isOwnerOrAdmin(c, connectionID, id)
	if target == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Container not found"})
		return
	}
	if !isOwnerOrAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: only container creator or administrator can view shares"})
		return
	}

	shares, err := h.dockerService.ListShares(c.Request.Context(), connectionID, target.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if shares == nil {
		shares = []domain.DockerContainerShare{}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    shares,
	})
}

// AddShare grants or updates access for a user to a container
func (h *DockerHandler) AddShare(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

	isOwnerOrAdmin, u, target := h.isOwnerOrAdmin(c, connectionID, id)
	if target == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Container not found"})
		return
	}
	if !isOwnerOrAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: only container creator or administrator can grant access"})
		return
	}

	var req struct {
		UserID     int    `json:"userId" binding:"required"`
		Permission string `json:"permission"` // "read" or "manage"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid share request: " + err.Error()})
		return
	}

	if u != nil && req.UserID == u.ID {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "You cannot share a container with yourself"})
		return
	}

	currentUserID := 0
	if u != nil {
		currentUserID = u.ID
	}

	if err := h.dockerService.AddShare(c.Request.Context(), connectionID, target.ID, req.UserID, req.Permission, currentUserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Access granted successfully",
	})
}

// DeleteShare revokes access for a user to a container
func (h *DockerHandler) DeleteShare(c *gin.Context) {
	id := c.Param("id")
	targetUserIDStr := c.Param("userId")
	connectionID := c.Query("connectionId")

	targetUserID, err := strconv.Atoi(targetUserIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid target user ID"})
		return
	}

	isOwnerOrAdmin, _, target := h.isOwnerOrAdmin(c, connectionID, id)
	if target == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Container not found"})
		return
	}
	if !isOwnerOrAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Access denied: only container creator or administrator can revoke access"})
		return
	}

	if err := h.dockerService.DeleteShare(c.Request.Context(), connectionID, target.ID, targetUserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Access revoked successfully",
	})
}

// ListUsers returns list of available users in the system for sharing
func (h *DockerHandler) ListUsers(c *gin.Context) {
	users, err := h.dockerRepo.ListAvailableUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if users == nil {
		users = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    users,
	})
}

// -------------------------------------------------------------
// Images Handlers
// -------------------------------------------------------------

func (h *DockerHandler) ListImages(c *gin.Context) {
	connectionID := c.Query("connectionId")

	images, err := h.dockerService.ListImages(c.Request.Context(), connectionID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   err.Error(),
			"data":    []domain.DockerImage{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"total":   len(images),
		"data":    images,
	})
}

func (h *DockerHandler) PullImage(c *gin.Context) {
	connectionID := c.Query("connectionId")
	var input struct {
		Image string `json:"image" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Image name is required"})
		return
	}

	if err := h.dockerService.PullImage(c.Request.Context(), connectionID, input.Image); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Image pulled successfully",
	})
}

func (h *DockerHandler) RemoveImage(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")
	force := c.Query("force") == "true"

	if err := h.dockerService.RemoveImage(c.Request.Context(), connectionID, id, force); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Image removed successfully",
	})
}

func (h *DockerHandler) GetSystemInfo(c *gin.Context) {
	connectionID := c.Query("connectionId")

	info, err := h.dockerService.GetSystemInfo(c.Request.Context(), connectionID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    info,
	})
}

// -------------------------------------------------------------
// Network Handlers
// -------------------------------------------------------------

func (h *DockerHandler) ListNetworks(c *gin.Context) {
	connectionID := c.Query("connectionId")

	networks, err := h.dockerService.ListNetworks(c.Request.Context(), connectionID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   err.Error(),
			"data":    []domain.DockerNetwork{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"total":   len(networks),
		"data":    networks,
	})
}

func (h *DockerHandler) CreateNetwork(c *gin.Context) {
	connectionID := c.Query("connectionId")
	var req domain.CreateNetworkRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid network specification"})
		return
	}

	if err := h.dockerService.CreateNetwork(c.Request.Context(), connectionID, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Network created successfully",
	})
}

func (h *DockerHandler) RemoveNetwork(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

	if err := h.dockerService.RemoveNetwork(c.Request.Context(), connectionID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Network removed successfully",
	})
}

// Aliases for route flexibility
func (h *DockerHandler) DeleteContainer(c *gin.Context) {
	h.RemoveContainer(c)
}

func (h *DockerHandler) GetLogs(c *gin.Context) {
	h.GetContainerLogs(c)
}

func (h *DockerHandler) GetStats(c *gin.Context) {
	h.GetContainerStats(c)
}

func (h *DockerHandler) DeleteImage(c *gin.Context) {
	h.RemoveImage(c)
}

func (h *DockerHandler) DeleteNetwork(c *gin.Context) {
	h.RemoveNetwork(c)
}

