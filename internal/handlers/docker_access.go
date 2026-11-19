package handlers

import (
	"net/http"
	"strings"

	"go-hephaestus/internal/core/domain"

	"github.com/gin-gonic/gin"
)

// Sharing an engine does not grant access to its private containers. Always
// resolve even the default engine against the authenticated user's connections.
func (h *DockerHandler) authorizeDockerConnection(c *gin.Context) (string, bool) {
	return h.requireDockerConnection(c, c.Query("connectionId"), "read")
}

func (h *DockerHandler) requireDockerConnection(c *gin.Context, requestedID, action string) (string, bool) {
	userValue, _ := c.Get("user")
	user, ok := userValue.(*domain.User)
	if !ok || user == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Authentication required"})
		return "", false
	}
	connections, err := h.dockerRepo.ListConnections(c.Request.Context(), user.ID, user.Role)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Unable to verify Docker connection access"})
		return "", false
	}
	if connectionID, allowed := selectDockerConnection(connections, requestedID); allowed {
		for _, connection := range connections {
			if connection.ID == connectionID && (action == "read" || connection.IsOwner || connection.UserPermission == "manage" || user.IsAdmin()) {
				return connectionID, true
			}
		}
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": "Docker connection access denied"})
	return "", false
}

func selectDockerConnection(connections []domain.DockerConnection, requestedID string) (string, bool) {
	for _, connection := range connections {
		if connection.ID == requestedID || (requestedID == "" && connection.IsActive) {
			return connection.ID, true
		}
	}
	return "", false
}

func ownsDockerContainer(user *domain.User, container domain.DockerContainer) bool {
	if user == nil {
		return false
	}
	if container.UserID != nil {
		return *container.UserID == user.ID
	}
	return container.OwnerUsername != "" && strings.EqualFold(container.OwnerUsername, user.Username)
}

// Both the list and summary use this policy, including explicitly published
// public containers. Connection-level sharing alone never satisfies it.
func visibleDockerContainers(user *domain.User, containers []domain.DockerContainer, sharesMap map[string][]domain.DockerContainerShare) []domain.DockerContainer {
	visible := make([]domain.DockerContainer, 0, len(containers))
	if user == nil {
		return visible
	}
	for _, container := range containers {
		container.IsOwner = ownsDockerContainer(user, container)
		shares := sharesMap[container.ID]
		if len(shares) == 0 && len(container.ID) >= 12 {
			shares = sharesMap[container.ID[:12]]
		}
		container.SharesCount = len(shares)
		container.UserPermission = ""
		if user.IsAdmin() || container.IsOwner {
			container.UserPermission = "owner"
		} else {
			for _, share := range shares {
				if share.UserID == user.ID && (share.Permission == "read" || share.Permission == "manage") {
					container.UserPermission = share.Permission
					break
				}
			}
			if container.UserPermission == "" && container.Visibility == "public" {
				container.UserPermission = "public"
			}
		}
		if container.UserPermission != "" {
			visible = append(visible, container)
		}
	}
	return visible
}

func setDockerContainerCounts(info *domain.DockerSystemInfo, containers []domain.DockerContainer) {
	info.Containers = len(containers)
	info.ContainersRunning = 0
	info.ContainersPaused = 0
	info.ContainersStopped = 0
	for _, container := range containers {
		switch container.State {
		case "running":
			info.ContainersRunning++
		case "paused":
			info.ContainersPaused++
		default:
			info.ContainersStopped++
		}
	}
}
