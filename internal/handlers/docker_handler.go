package handlers

import (
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

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"total":      len(containers),
		"data":       containers,
	})
}

// GetContainer retrieves details of a specific container
func (h *DockerHandler) GetContainer(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

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

	if err := h.dockerService.StartContainer(c.Request.Context(), connectionID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Container started successfully"})
}

func (h *DockerHandler) StopContainer(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

	if err := h.dockerService.StopContainer(c.Request.Context(), connectionID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Container stopped successfully"})
}

func (h *DockerHandler) RestartContainer(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

	if err := h.dockerService.RestartContainer(c.Request.Context(), connectionID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Container restarted successfully"})
}

func (h *DockerHandler) PauseContainer(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

	if err := h.dockerService.PauseContainer(c.Request.Context(), connectionID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Container paused successfully"})
}

func (h *DockerHandler) UnpauseContainer(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

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

	logs, err := h.dockerService.GetContainerLogs(c.Request.Context(), connectionID, id, tail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "logs": logs})
}

func (h *DockerHandler) GetContainerStats(c *gin.Context) {
	id := c.Param("id")
	connectionID := c.Query("connectionId")

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

