package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"
	"go-hephaestus/internal/repository"
)

type DockerService struct {
	dockerRepo *repository.DockerRepository
	remoteRepo *repository.RemoteHostRepository
	sshService *SSHService
}

func NewDockerService(
	dockerRepo *repository.DockerRepository,
	remoteRepo *repository.RemoteHostRepository,
	sshService *SSHService,
) *DockerService {
	return &DockerService{
		dockerRepo: dockerRepo,
		remoteRepo: remoteRepo,
		sshService: sshService,
	}
}

// -------------------------------------------------------------
// Connection Resolver
// -------------------------------------------------------------

func (s *DockerService) resolveConnection(ctx context.Context, connectionID string) (*domain.DockerConnection, error) {
	if connectionID == "" || connectionID == "default" || connectionID == "active" {
		conn, err := s.dockerRepo.GetDefaultConnection(ctx)
		if err != nil {
			return nil, err
		}
		if conn == nil {
			return nil, errors.New("no active Docker connection found. Please configure a Docker host")
		}
		return conn, nil
	}

	conn, err := s.dockerRepo.GetConnectionByID(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, fmt.Errorf("docker connection '%s' not found", connectionID)
	}
	return conn, nil
}

func (s *DockerService) resolveRemoteHost(ctx context.Context, conn *domain.DockerConnection) (*domain.RemoteHostConfig, error) {
	if conn.RemoteHostID != nil && *conn.RemoteHostID != "" {
		return s.remoteRepo.GetRawByID(ctx, *conn.RemoteHostID)
	}

	if conn.SSHHost == nil || *conn.SSHHost == "" {
		return nil, errors.New("remote SSH host configuration is missing")
	}

	port := 22
	if conn.SSHPort != nil && *conn.SSHPort > 0 {
		port = *conn.SSHPort
	}
	user := "root"
	if conn.SSHUser != nil && *conn.SSHUser != "" {
		user = *conn.SSHUser
	}
	auth := "password"
	if conn.SSHAuth != nil && *conn.SSHAuth != "" {
		auth = *conn.SSHAuth
	}

	return &domain.RemoteHostConfig{
		ID:       conn.ID,
		Name:     conn.Name,
		Host:     *conn.SSHHost,
		Port:     port,
		Username: user,
		AuthType: auth,
		Password: conn.SSHPassword,
		SSHKey:   conn.SSHKey,
	}, nil
}

// -------------------------------------------------------------
// Container Operations
// -------------------------------------------------------------

// ListContainers retrieves container list from the target Docker host
func (s *DockerService) ListContainers(ctx context.Context, connectionID string, all bool) ([]domain.DockerContainer, error) {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		return s.listContainersSSH(ctx, conn, all)
	}

	return s.listContainersLocal(ctx, conn, all)
}

func (s *DockerService) listContainersLocal(ctx context.Context, conn *domain.DockerConnection, all bool) ([]domain.DockerContainer, error) {
	client, baseURL := s.getLocalHTTPClient(conn)
	url := fmt.Sprintf("%s/v1.43/containers/json?all=%t", baseURL, all)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to communicate with Docker daemon socket (%s): %w", conn.SocketPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("docker returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var raw []struct {
		ID      string            `json:"Id"`
		Names   []string          `json:"Names"`
		Image   string            `json:"Image"`
		ImageID string            `json:"ImageID"`
		Command string            `json:"Command"`
		Created int64             `json:"Created"`
		State   string            `json:"State"`
		Status  string            `json:"Status"`
		Ports   []struct {
			IP          string `json:"IP"`
			PrivatePort int    `json:"PrivatePort"`
			PublicPort  int    `json:"PublicPort"`
			Type        string `json:"Type"`
		} `json:"Ports"`
		Labels map[string]string `json:"Labels"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	containers := make([]domain.DockerContainer, 0, len(raw))
	for _, r := range raw {
		name := ""
		if len(r.Names) > 0 {
			name = strings.TrimPrefix(r.Names[0], "/")
		}

		ports := make([]domain.DockerContainerPort, 0, len(r.Ports))
		for _, p := range r.Ports {
			ports = append(ports, domain.DockerContainerPort{
				IP:          p.IP,
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Type:        p.Type,
			})
		}

		containers = append(containers, domain.DockerContainer{
			ID:      r.ID,
			Names:   r.Names,
			Name:    name,
			Image:   r.Image,
			ImageID: r.ImageID,
			Command: r.Command,
			Created: r.Created,
			State:   r.State,
			Status:  r.Status,
			Ports:   ports,
			Labels:  r.Labels,
		})
	}

	return containers, nil
}

func (s *DockerService) listContainersSSH(ctx context.Context, conn *domain.DockerConnection, all bool) ([]domain.DockerContainer, error) {
	remoteHost, err := s.resolveRemoteHost(ctx, conn)
	if err != nil {
		return nil, err
	}

	allFlag := ""
	if all {
		allFlag = "-a"
	}
	cmd := fmt.Sprintf(`docker ps %s --no-trunc --format '{{json .}}'`, allFlag)
	stdout, _, _, err := s.sshService.ExecuteElevatedCommand(remoteHost, cmd)
	if err != nil {
		return nil, fmt.Errorf("remote docker ps failed: %w", err)
	}

	var containers []domain.DockerContainer
	scanner := bufio.NewScanner(strings.NewReader(stdout))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var item struct {
			ID      string `json:"ID"`
			Names   string `json:"Names"`
			Image   string `json:"Image"`
			Command string `json:"Command"`
			Created string `json:"CreatedAt"`
			State   string `json:"State"`
			Status  string `json:"Status"`
			Ports   string `json:"Ports"`
		}

		if err := json.Unmarshal([]byte(line), &item); err == nil {
			state := strings.ToLower(item.State)
			if state == "" {
				if strings.HasPrefix(strings.ToLower(item.Status), "up") {
					state = "running"
				} else {
					state = "exited"
				}
			}

			containers = append(containers, domain.DockerContainer{
				ID:      item.ID,
				Names:   []string{"/" + item.Names},
				Name:    item.Names,
				Image:   item.Image,
				Command: item.Command,
				State:   state,
				Status:  item.Status,
				Ports:   parsePortsString(item.Ports),
			})
		}
	}

	return containers, nil
}

// -------------------------------------------------------------
// Container Lifecycle Actions (Start, Stop, Restart, Pause, Unpause, Delete)
// -------------------------------------------------------------

func (s *DockerService) StartContainer(ctx context.Context, connectionID, containerID string) error {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return err
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		remoteHost, err := s.resolveRemoteHost(ctx, conn)
		if err != nil {
			return err
		}
		_, _, _, err = s.sshService.ExecuteElevatedCommand(remoteHost, fmt.Sprintf("docker start %s", containerID))
		return err
	}

	return s.doLocalPost(ctx, conn, fmt.Sprintf("/v1.43/containers/%s/start", containerID), nil)
}

func (s *DockerService) StopContainer(ctx context.Context, connectionID, containerID string) error {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return err
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		remoteHost, err := s.resolveRemoteHost(ctx, conn)
		if err != nil {
			return err
		}
		_, _, _, err = s.sshService.ExecuteElevatedCommand(remoteHost, fmt.Sprintf("docker stop %s", containerID))
		return err
	}

	return s.doLocalPost(ctx, conn, fmt.Sprintf("/v1.43/containers/%s/stop?t=10", containerID), nil)
}

func (s *DockerService) RestartContainer(ctx context.Context, connectionID, containerID string) error {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return err
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		remoteHost, err := s.resolveRemoteHost(ctx, conn)
		if err != nil {
			return err
		}
		_, _, _, err = s.sshService.ExecuteElevatedCommand(remoteHost, fmt.Sprintf("docker restart %s", containerID))
		return err
	}

	return s.doLocalPost(ctx, conn, fmt.Sprintf("/v1.43/containers/%s/restart?t=10", containerID), nil)
}

func (s *DockerService) PauseContainer(ctx context.Context, connectionID, containerID string) error {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return err
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		remoteHost, err := s.resolveRemoteHost(ctx, conn)
		if err != nil {
			return err
		}
		_, _, _, err = s.sshService.ExecuteElevatedCommand(remoteHost, fmt.Sprintf("docker pause %s", containerID))
		return err
	}

	return s.doLocalPost(ctx, conn, fmt.Sprintf("/v1.43/containers/%s/pause", containerID), nil)
}

func (s *DockerService) UnpauseContainer(ctx context.Context, connectionID, containerID string) error {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return err
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		remoteHost, err := s.resolveRemoteHost(ctx, conn)
		if err != nil {
			return err
		}
		_, _, _, err = s.sshService.ExecuteElevatedCommand(remoteHost, fmt.Sprintf("docker unpause %s", containerID))
		return err
	}

	return s.doLocalPost(ctx, conn, fmt.Sprintf("/v1.43/containers/%s/unpause", containerID), nil)
}

func (s *DockerService) RemoveContainer(ctx context.Context, connectionID, containerID string, force bool) error {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return err
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		remoteHost, err := s.resolveRemoteHost(ctx, conn)
		if err != nil {
			return err
		}
		forceFlag := ""
		if force {
			forceFlag = "-f"
		}
		_, _, _, err = s.sshService.ExecuteElevatedCommand(remoteHost, fmt.Sprintf("docker rm %s %s", forceFlag, containerID))
		return err
	}

	client, baseURL := s.getLocalHTTPClient(conn)
	url := fmt.Sprintf("%s/v1.43/containers/%s?v=1&force=%t", baseURL, containerID, force)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to remove container (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// -------------------------------------------------------------
// Container Logs & Stats
// -------------------------------------------------------------

func (s *DockerService) GetContainerLogs(ctx context.Context, connectionID, containerID string, tail int) (string, error) {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return "", err
	}

	if tail <= 0 {
		tail = 200
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		remoteHost, err := s.resolveRemoteHost(ctx, conn)
		if err != nil {
			return "", err
		}
		cmd := fmt.Sprintf("docker logs --tail %d %s", tail, containerID)
		stdout, stderr, _, err := s.sshService.ExecuteElevatedCommand(remoteHost, cmd)
		if err != nil {
			return "", err
		}
		if stdout != "" {
			return stdout, nil
		}
		return stderr, nil
	}

	client, baseURL := s.getLocalHTTPClient(conn)
	url := fmt.Sprintf("%s/v1.43/containers/%s/logs?stdout=1&stderr=1&tail=%d", baseURL, containerID, tail)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Clean docker log multiplexed headers (8-byte header per frame)
	clean := cleanDockerLogStream(body)
	return clean, nil
}

func (s *DockerService) GetContainerStats(ctx context.Context, connectionID, containerID string) (*domain.DockerContainerStats, error) {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		remoteHost, err := s.resolveRemoteHost(ctx, conn)
		if err != nil {
			return nil, err
		}
		cmd := fmt.Sprintf(`docker stats --no-stream --format '{{json .}}' %s`, containerID)
		stdout, _, _, err := s.sshService.ExecuteElevatedCommand(remoteHost, cmd)
		if err != nil {
			return nil, err
		}

		var statsItem struct {
			CPUPerc string `json:"CPUPerc"`
			MemPerc string `json:"MemPerc"`
			MemUsage string `json:"MemUsage"`
			NetIO   string `json:"NetIO"`
		}
		_ = json.Unmarshal([]byte(strings.TrimSpace(stdout)), &statsItem)

		cpuVal, _ := strconv.ParseFloat(strings.TrimSuffix(statsItem.CPUPerc, "%"), 64)
		memPercVal, _ := strconv.ParseFloat(strings.TrimSuffix(statsItem.MemPerc, "%"), 64)

		return &domain.DockerContainerStats{
			ContainerID:   containerID,
			CPUPercent:    cpuVal,
			MemoryPercent: memPercVal,
		}, nil
	}

	client, baseURL := s.getLocalHTTPClient(conn)
	url := fmt.Sprintf("%s/v1.43/containers/%s/stats?stream=0", baseURL, containerID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
			OnlineCPUs     uint32 `json:"online_cpus"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemoryStats struct {
			Usage uint64 `json:"usage"`
			Limit uint64 `json:"limit"`
		} `json:"memory_stats"`
		Networks map[string]struct {
			RxBytes uint64 `json:"rx_bytes"`
			TxBytes uint64 `json:"tx_bytes"`
		} `json:"networks"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	// CPU % Calculation
	cpuDelta := float64(raw.CPUStats.CPUUsage.TotalUsage) - float64(raw.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(raw.CPUStats.SystemCPUUsage) - float64(raw.PreCPUStats.SystemCPUUsage)
	cpuPercent := 0.0
	onlineCPUs := float64(raw.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = 1
	}
	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}

	// Memory MB
	memUsageMB := float64(raw.MemoryStats.Usage) / (1024 * 1024)
	memLimitMB := float64(raw.MemoryStats.Limit) / (1024 * 1024)
	memPercent := 0.0
	if memLimitMB > 0 {
		memPercent = (memUsageMB / memLimitMB) * 100.0
	}

	// Network
	var rxBytes, txBytes uint64
	for _, n := range raw.Networks {
		rxBytes += n.RxBytes
		txBytes += n.TxBytes
	}

	return &domain.DockerContainerStats{
		ContainerID:   containerID,
		CPUPercent:    cpuPercent,
		MemoryUsageMB: memUsageMB,
		MemoryLimitMB: memLimitMB,
		MemoryPercent: memPercent,
		NetworkRxMB:   float64(rxBytes) / (1024 * 1024),
		NetworkTxMB:   float64(txBytes) / (1024 * 1024),
	}, nil
}

// -------------------------------------------------------------
// Images Management (List, Pull, Remove)
// -------------------------------------------------------------

func (s *DockerService) ListImages(ctx context.Context, connectionID string) ([]domain.DockerImage, error) {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		remoteHost, err := s.resolveRemoteHost(ctx, conn)
		if err != nil {
			return nil, err
		}
		cmd := `docker images --no-trunc --format '{{json .}}'`
		stdout, _, _, err := s.sshService.ExecuteElevatedCommand(remoteHost, cmd)
		if err != nil {
			return nil, err
		}

		var images []domain.DockerImage
		scanner := bufio.NewScanner(strings.NewReader(stdout))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var item struct {
				ID         string `json:"ID"`
				Repository string `json:"Repository"`
				Tag        string `json:"Tag"`
				Size       string `json:"Size"`
				CreatedAt  string `json:"CreatedAt"`
			}
			if err := json.Unmarshal([]byte(line), &item); err == nil {
				tag := fmt.Sprintf("%s:%s", item.Repository, item.Tag)
				images = append(images, domain.DockerImage{
					ID:       item.ID,
					RepoTags: []string{tag},
					SizeMB:   parseSizeMB(item.Size),
				})
			}
		}
		return images, nil
	}

	client, baseURL := s.getLocalHTTPClient(conn)
	url := fmt.Sprintf("%s/v1.43/images/json", baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw []struct {
		ID          string   `json:"Id"`
		RepoTags    []string `json:"RepoTags"`
		Size        int64    `json:"Size"`
		Created     int64    `json:"Created"`
		Containers  int      `json:"Containers"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	images := make([]domain.DockerImage, 0, len(raw))
	for _, r := range raw {
		images = append(images, domain.DockerImage{
			ID:         r.ID,
			RepoTags:   r.RepoTags,
			SizeMB:     float64(r.Size) / (1024 * 1024),
			Created:    r.Created,
			Containers: r.Containers,
		})
	}
	return images, nil
}

func (s *DockerService) PullImage(ctx context.Context, connectionID, imageName string) error {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return err
	}

	imageName = strings.TrimSpace(imageName)
	if imageName == "" {
		return errors.New("image name is required")
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		remoteHost, err := s.resolveRemoteHost(ctx, conn)
		if err != nil {
			return err
		}
		cmd := fmt.Sprintf("docker pull %s", imageName)
		_, _, _, err = s.sshService.ExecuteElevatedCommand(remoteHost, cmd)
		return err
	}

	client, baseURL := s.getLocalHTTPClient(conn)
	url := fmt.Sprintf("%s/v1.43/images/create?fromImage=%s", baseURL, imageName)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to pull image (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (s *DockerService) RemoveImage(ctx context.Context, connectionID, imageID string, force bool) error {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return err
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		remoteHost, err := s.resolveRemoteHost(ctx, conn)
		if err != nil {
			return err
		}
		forceFlag := ""
		if force {
			forceFlag = "-f"
		}
		_, _, _, err = s.sshService.ExecuteElevatedCommand(remoteHost, fmt.Sprintf("docker rmi %s %s", forceFlag, imageID))
		return err
	}

	client, baseURL := s.getLocalHTTPClient(conn)
	url := fmt.Sprintf("%s/v1.43/images/%s?force=%t", baseURL, imageID, force)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to remove image (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// -------------------------------------------------------------
// Deploy New Container
// -------------------------------------------------------------

func (s *DockerService) DeployContainer(ctx context.Context, connectionID string, req domain.DeployContainerRequest) (string, error) {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(req.Image) == "" {
		return "", errors.New("image is required")
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		remoteHost, err := s.resolveRemoteHost(ctx, conn)
		if err != nil {
			return "", err
		}

		args := []string{"run", "-d"}
		if req.Name != "" {
			args = append(args, fmt.Sprintf("--name %s", req.Name))
		}
		if req.RestartPolicy != "" && req.RestartPolicy != "no" {
			args = append(args, fmt.Sprintf("--restart %s", req.RestartPolicy))
		}
		for _, p := range req.PortBindings {
			p = strings.TrimSpace(p)
			if p != "" {
				args = append(args, fmt.Sprintf("-p %s", p))
			}
		}
		for _, v := range req.VolumeBindings {
			v = strings.TrimSpace(v)
			if v != "" {
				args = append(args, fmt.Sprintf("-v %s", v))
			}
		}
		for _, e := range req.EnvVars {
			e = strings.TrimSpace(e)
			if e != "" {
				args = append(args, fmt.Sprintf("-e %s", e))
			}
		}

		args = append(args, req.Image)
		if req.Command != "" {
			args = append(args, req.Command)
		}

		runCmd := fmt.Sprintf("docker %s", strings.Join(args, " "))
		stdout, stderr, _, err := s.sshService.ExecuteElevatedCommand(remoteHost, runCmd)
		if err != nil {
			return "", fmt.Errorf("deploy container failed: %w (stderr: %s)", err, stderr)
		}
		return strings.TrimSpace(stdout), nil
	}

	// Deploy on Local Docker Engine
	client, baseURL := s.getLocalHTTPClient(conn)
	createURL := fmt.Sprintf("%s/v1.43/containers/create", baseURL)
	if req.Name != "" {
		createURL += fmt.Sprintf("?name=%s", req.Name)
	}

	// Parse Port bindings
	exposedPorts := make(map[string]struct{})
	portMap := make(map[string][]map[string]string)
	for _, p := range req.PortBindings {
		parts := strings.Split(p, ":")
		if len(parts) == 2 {
			hostPort := parts[0]
			containerPort := parts[1]
			if !strings.Contains(containerPort, "/") {
				containerPort += "/tcp"
			}
			exposedPorts[containerPort] = struct{}{}
			portMap[containerPort] = []map[string]string{{"HostPort": hostPort}}
		}
	}

	restartPolicyName := req.RestartPolicy
	if restartPolicyName == "" {
		restartPolicyName = "unless-stopped"
	}

	payload := map[string]interface{}{
		"Image":        req.Image,
		"ExposedPorts": exposedPorts,
		"Env":          req.EnvVars,
		"HostConfig": map[string]interface{}{
			"Binds":        req.VolumeBindings,
			"PortBindings": portMap,
			"RestartPolicy": map[string]string{
				"Name": restartPolicyName,
			},
		},
	}
	if req.Command != "" {
		payload["Cmd"] = strings.Fields(req.Command)
	}

	bodyJSON, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", createURL, bytes.NewBuffer(bodyJSON))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.ID == "" {
		return "", errors.New("container created but no container ID returned")
	}

	// Automatically start created container
	startURL := fmt.Sprintf("%s/v1.43/containers/%s/start", baseURL, result.ID)
	startReq, _ := http.NewRequestWithContext(ctx, "POST", startURL, nil)
	_, _ = client.Do(startReq)

	return result.ID, nil
}

// -------------------------------------------------------------
// System Info & Connectivity Test
// -------------------------------------------------------------

func (s *DockerService) TestConnection(ctx context.Context, conn *domain.DockerConnection) (bool, string, *domain.DockerSystemInfo) {
	if conn == nil {
		return false, "Connection is nil", nil
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		remoteHost, err := s.resolveRemoteHost(ctx, conn)
		if err != nil {
			return false, err.Error(), nil
		}

		ok, msg := s.sshService.TestConnection(remoteHost)
		if !ok {
			return false, fmt.Sprintf("SSH connection failed: %s", msg), nil
		}

		stdout, _, _, err := s.sshService.ExecuteElevatedCommand(remoteHost, "docker info --format '{{json .}}'")
		if err != nil {
			return false, fmt.Sprintf("Docker is not accessible or not running on remote host: %v", err), nil
		}

		var info domain.DockerSystemInfo
		_ = json.Unmarshal([]byte(strings.TrimSpace(stdout)), &info)
		return true, "Successfully connected to remote Docker daemon via SSH!", &info
	}

	// Local Socket / TCP
	client, baseURL := s.getLocalHTTPClient(conn)
	url := fmt.Sprintf("%s/v1.43/info", baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err.Error(), nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Sprintf("Cannot connect to Docker daemon (%s): %v", conn.SocketPath, err), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("Docker returned HTTP %d", resp.StatusCode), nil
	}

	var rawInfo struct {
		ServerVersion string  `json:"ServerVersion"`
		Containers    int     `json:"Containers"`
		ContainersRunning int `json:"ContainersRunning"`
		ContainersPaused  int `json:"ContainersPaused"`
		ContainersStopped int `json:"ContainersStopped"`
		Images        int     `json:"Images"`
		OperatingSystem string `json:"OperatingSystem"`
		OSType        string  `json:"OSType"`
		Architecture  string  `json:"Architecture"`
		NCPU          int     `json:"NCPU"`
		MemTotal      uint64  `json:"MemTotal"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&rawInfo)

	info := &domain.DockerSystemInfo{
		ServerVersion:     rawInfo.ServerVersion,
		Containers:        rawInfo.Containers,
		ContainersRunning: rawInfo.ContainersRunning,
		ContainersPaused:  rawInfo.ContainersPaused,
		ContainersStopped: rawInfo.ContainersStopped,
		Images:            rawInfo.Images,
		OperatingSystem:   rawInfo.OperatingSystem,
		OSType:            rawInfo.OSType,
		Architecture:      rawInfo.Architecture,
		NCPU:              rawInfo.NCPU,
		MemTotalMB:        float64(rawInfo.MemTotal) / (1024 * 1024),
	}

	return true, "Successfully connected to local Docker daemon socket!", info
}

func (s *DockerService) GetSystemInfo(ctx context.Context, connectionID string) (*domain.DockerSystemInfo, error) {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	ok, msg, info := s.TestConnection(ctx, conn)
	if !ok {
		return nil, errors.New(msg)
	}
	return info, nil
}

// -------------------------------------------------------------
// Internal HTTP Helpers
// -------------------------------------------------------------

func (s *DockerService) getLocalHTTPClient(conn *domain.DockerConnection) (*http.Client, string) {
	socketPath := conn.SocketPath
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}

	if conn.HostType == "tcp" && conn.TcpURL != "" {
		return &http.Client{Timeout: 20 * time.Second}, strings.TrimRight(conn.TcpURL, "/")
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		},
		DisableCompression: true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}, "http://localhost"
}

func (s *DockerService) doLocalPost(ctx context.Context, conn *domain.DockerConnection, endpoint string, body io.Reader) error {
	client, baseURL := s.getLocalHTTPClient(conn)
	url := fmt.Sprintf("%s%s", baseURL, endpoint)

	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("action returned HTTP %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}

func cleanDockerLogStream(raw []byte) string {
	var buf strings.Builder
	r := bytes.NewReader(raw)
	header := make([]byte, 8)

	for {
		_, err := io.ReadFull(r, header)
		if err != nil {
			break
		}
		frameSize := int(header[4])<<24 | int(header[5])<<16 | int(header[6])<<8 | int(header[7])
		if frameSize <= 0 {
			continue
		}
		frame := make([]byte, frameSize)
		_, err = io.ReadFull(r, frame)
		if err != nil {
			buf.Write(frame)
			break
		}
		buf.Write(frame)
	}

	if buf.Len() == 0 && len(raw) > 0 {
		return string(raw)
	}
	return buf.String()
}

func parsePortsString(portsStr string) []domain.DockerContainerPort {
	var list []domain.DockerContainerPort
	if portsStr == "" {
		return list
	}

	parts := strings.Split(portsStr, ", ")
	for _, p := range parts {
		// Example: 0.0.0.0:80->80/tcp
		if strings.Contains(p, "->") {
			sub := strings.Split(p, "->")
			hostSide := sub[0]
			containerSide := sub[1]

			pubPort := 0
			ip := ""
			if strings.Contains(hostSide, ":") {
				hParts := strings.Split(hostSide, ":")
				ip = hParts[0]
				pubPort, _ = strconv.Atoi(hParts[1])
			}

			privPort := 0
			portType := "tcp"
			if strings.Contains(containerSide, "/") {
				cParts := strings.Split(containerSide, "/")
				privPort, _ = strconv.Atoi(cParts[0])
				portType = cParts[1]
			}

			list = append(list, domain.DockerContainerPort{
				IP:          ip,
				PublicPort:  pubPort,
				PrivatePort: privPort,
				Type:        portType,
			})
		}
	}
	return list
}

func parseSizeMB(sizeStr string) float64 {
	s := strings.TrimSpace(strings.ToUpper(sizeStr))
	if strings.HasSuffix(s, "MB") {
		val, _ := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, "MB")), 64)
		return val
	}
	if strings.HasSuffix(s, "GB") {
		val, _ := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, "GB")), 64)
		return val * 1024
	}
	if strings.HasSuffix(s, "KB") {
		val, _ := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, "KB")), 64)
		return val / 1024
	}
	return 0
}
