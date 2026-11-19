package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go-hephaestus/internal/core/domain"

	"github.com/gin-gonic/gin"
)

func TestVisibleDockerContainers(t *testing.T) {
	ownerID, otherID := 7, 9
	containers := []domain.DockerContainer{
		{ID: "owned", UserID: &ownerID, State: "running"},
		{ID: "private", UserID: &otherID, State: "running"},
		{ID: "shared-read", UserID: &otherID, State: "exited"},
		{ID: "123456789012abcdef", UserID: &otherID, State: "paused"},
		{ID: "legacy", OwnerUsername: "OPERATOR", State: "running"},
		{ID: "wrong-id", UserID: &otherID, OwnerUsername: "operator", State: "running"},
		{ID: "public", UserID: &otherID, Visibility: "public", State: "running"},
	}
	shares := map[string][]domain.DockerContainerShare{
		"shared-read":  {{UserID: ownerID, Permission: "read"}},
		"123456789012": {{UserID: ownerID, Permission: "manage"}},
		"private":      {{UserID: otherID, Permission: "manage"}},
	}
	tests := []struct {
		name string
		user *domain.User
		want int
	}{
		{"missing authentication", nil, 0},
		{"administrator", &domain.User{ID: 1, Role: "ADMIN"}, 7},
		{"owner and explicit shares", &domain.User{ID: ownerID, Username: "operator", Role: "OPERATOR"}, 5},
		{"connection share does not expose private containers", &domain.User{ID: 11, Role: "VIEWER"}, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			visible := visibleDockerContainers(test.user, containers, shares)
			if len(visible) != test.want {
				t.Fatalf("got %d containers, want %d", len(visible), test.want)
			}
		})
	}
	visible := visibleDockerContainers(&domain.User{ID: ownerID, Username: "operator", Role: "OPERATOR"}, containers, shares)
	if visible[1].UserPermission != "read" || visible[2].UserPermission != "manage" {
		t.Fatal("full and short container IDs must preserve read/manage sharing permissions")
	}
	if containers[0].UserPermission != "" {
		t.Fatal("filter must not mutate the source container list")
	}
}

func TestDockerAccessFailsClosedWithoutAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &DockerHandler{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/docker/system/info", nil)
	if _, allowed := handler.authorizeDockerConnection(ctx); allowed || recorder.Code != http.StatusUnauthorized {
		t.Fatal("unauthenticated connection access must return 401")
	}
	if handler.canAccessContainer(ctx, "engine", "container", "read") {
		t.Fatal("missing user context must never grant container access")
	}
	ctx.Set("user", "invalid user context")
	if handler.canAccessContainer(ctx, "engine", "container", "manage") {
		t.Fatal("invalid user context must never grant container access")
	}
}

func TestSelectDockerConnection(t *testing.T) {
	connections := []domain.DockerConnection{
		{ID: "inactive", IsActive: false},
		{ID: "shared-engine", IsActive: true},
		{ID: "own-engine", IsActive: true},
	}
	for _, test := range []struct {
		requested string
		want      string
		allowed   bool
	}{
		{"", "shared-engine", true},
		{"own-engine", "own-engine", true},
		{"other-users-default", "", false},
	} {
		got, allowed := selectDockerConnection(connections, test.requested)
		if got != test.want || allowed != test.allowed {
			t.Errorf("select(%q) = (%q, %t), want (%q, %t)", test.requested, got, allowed, test.want, test.allowed)
		}
	}
	if _, allowed := selectDockerConnection(nil, ""); allowed {
		t.Fatal("users with no accessible connections must not fall back to the global engine")
	}
}

func TestDockerSummaryUsesVisibleContainers(t *testing.T) {
	ownerID, otherID := 7, 9
	containers := []domain.DockerContainer{
		{ID: "owned", UserID: &ownerID, State: "running"},
		{ID: "hidden", UserID: &otherID, State: "running"},
		{ID: "shared", UserID: &otherID, State: "paused"},
		{ID: "stopped", UserID: &ownerID, State: "exited"},
	}
	user := &domain.User{ID: ownerID, Role: "OPERATOR"}
	shares := map[string][]domain.DockerContainerShare{"shared": {{UserID: ownerID, Permission: "read"}}}
	info := &domain.DockerSystemInfo{Containers: 100, ContainersRunning: 90, ContainersPaused: 5, ContainersStopped: 5, ServerVersion: "29.0"}
	setDockerContainerCounts(info, visibleDockerContainers(user, containers, shares))
	if info.Containers != 3 || info.ContainersRunning != 1 || info.ContainersPaused != 1 || info.ContainersStopped != 1 {
		t.Fatalf("host totals leaked into user summary: %+v", info)
	}
	if info.ServerVersion != "29.0" {
		t.Fatal("engine details should remain unchanged")
	}
	setDockerContainerCounts(info, nil)
	if info.Containers != 0 || info.ContainersRunning != 0 || info.ContainersPaused != 0 || info.ContainersStopped != 0 {
		t.Fatal("empty authorized list must reset all counts to zero")
	}
}
