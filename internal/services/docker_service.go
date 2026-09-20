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
// Remote SSH Docker Helper
// -------------------------------------------------------------

func (s *DockerService) runRemoteDockerCommand(ctx context.Context, conn *domain.DockerConnection, dockerArgs string) (string, string, error) {
	remoteHost, err := s.resolveRemoteHost(ctx, conn)
	if err != nil {
		return "", "", err
	}

	socketPath := "/var/run/docker.sock"
	if conn.SocketPath != "" {
		socketPath = conn.SocketPath
	}

	escapedPass := ""
	if remoteHost.Password != nil && *remoteHost.Password != "" {
		escapedPass = strings.ReplaceAll(*remoteHost.Password, "'", "'\\''")
	}

	// Smart Docker Remote Wrapper:
	// 1. Export PATH so docker CLI is found across all distributions (/usr/bin, /usr/local/bin, /snap/bin)
	// 2. Set DOCKER_HOST to socketPath
	// 3. Test if user has direct permission on docker socket
	// 4. If permission denied, auto-add user to 'docker' group in background
	// 5. Ensure socket permissions if accessible and execute with elevation fallback
	script := fmt.Sprintf(`export PATH=$PATH:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/snap/bin
_SOCK="%s"
export DOCKER_HOST="unix://$_SOCK"
_P='%s'

_exec_docker() {
    # 1. If running as root, execute directly
    if [ "$(id -u)" -eq 0 ]; then
        docker "$@"
        return $?
    fi

    # 2. If user already has permission on docker socket
    if docker ps >/dev/null 2>&1; then
        docker "$@"
        return $?
    fi

    # 3. User lacks docker permission.
    # Auto-add user to docker group in background for permanent access
    if [ -n "$_P" ]; then
        echo "$_P" | sudo -S -p '' usermod -aG docker $(whoami) 2>/dev/null || true
    elif sudo -n true 2>/dev/null; then
        sudo -n usermod -aG docker $(whoami) 2>/dev/null || true
    fi

    # 4. Also ensure socket permissions if it is a unix socket
    if [ -S "$_SOCK" ]; then
        if [ -n "$_P" ]; then
            echo "$_P" | sudo -S -p '' chmod 666 "$_SOCK" 2>/dev/null || true
        elif sudo -n true 2>/dev/null; then
            sudo -n chmod 666 "$_SOCK" 2>/dev/null || true
        fi
    fi

    # 5. Execute with elevation so current command succeeds immediately
    if [ -n "$_P" ]; then
        echo "$_P" | sudo -S -p '' docker "$@"
    elif sudo -n true 2>/dev/null; then
        sudo -n docker "$@"
    else
        docker "$@"
    fi
}

_exec_docker %s
`, socketPath, escapedPass, dockerArgs)

	stdout, stderr, exitCode, err := s.sshService.ExecuteCommand(remoteHost, script)
	if err != nil {
		return "", stderr, err
	}
	if exitCode != 0 {
		errMsg := strings.TrimSpace(stderr)
		if errMsg == "" {
			errMsg = strings.TrimSpace(stdout)
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("docker command exited with code %d", exitCode)
		}
		return stdout, stderr, errors.New(errMsg)
	}

	return stdout, stderr, nil
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
		Labels          map[string]string `json:"Labels"`
		NetworkSettings struct {
			Networks map[string]struct {
				IPAddress string `json:"IPAddress"`
			} `json:"Networks"`
		} `json:"NetworkSettings"`
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

		var netNames []string
		ipAddr := ""
		for netName, netData := range r.NetworkSettings.Networks {
			netNames = append(netNames, netName)
			if ipAddr == "" && netData.IPAddress != "" {
				ipAddr = netData.IPAddress
			}
		}

		containers = append(containers, domain.DockerContainer{
			ID:        r.ID,
			Names:     r.Names,
			Name:      name,
			Image:     r.Image,
			ImageID:   r.ImageID,
			Command:   r.Command,
			Created:   r.Created,
			State:     r.State,
			Status:    r.Status,
			Ports:     ports,
			Networks:  netNames,
			IPAddress: ipAddr,
			Labels:    r.Labels,
		})
	}

	return containers, nil
}

func (s *DockerService) listContainersSSH(ctx context.Context, conn *domain.DockerConnection, all bool) ([]domain.DockerContainer, error) {
	allFlag := ""
	if all {
		allFlag = "-a"
	}
	stdout, _, err := s.runRemoteDockerCommand(ctx, conn, fmt.Sprintf("ps %s --no-trunc --format '{{json .}}'", allFlag))
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
			ID       string `json:"ID"`
			Names    string `json:"Names"`
			Image    string `json:"Image"`
			Command  string `json:"Command"`
			Created  string `json:"CreatedAt"`
			State    string `json:"State"`
			Status   string `json:"Status"`
			Ports    string `json:"Ports"`
			Networks string `json:"Networks"`
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

			var netNames []string
			if item.Networks != "" {
				for _, n := range strings.Split(item.Networks, ",") {
					n = strings.TrimSpace(n)
					if n != "" {
						netNames = append(netNames, n)
					}
				}
			}

			createdUnix := parseDockerTime(item.Created)

			containers = append(containers, domain.DockerContainer{
				ID:       item.ID,
				Names:    []string{"/" + item.Names},
				Name:     item.Names,
				Image:    item.Image,
				Command:  item.Command,
				Created:  createdUnix,
				State:    state,
				Status:   item.Status,
				Ports:    parsePortsString(item.Ports),
				Networks: netNames,
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
		_, _, err := s.runRemoteDockerCommand(ctx, conn, fmt.Sprintf("start %s", containerID))
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
		_, _, err := s.runRemoteDockerCommand(ctx, conn, fmt.Sprintf("stop %s", containerID))
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
		_, _, err := s.runRemoteDockerCommand(ctx, conn, fmt.Sprintf("restart %s", containerID))
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
		_, _, err := s.runRemoteDockerCommand(ctx, conn, fmt.Sprintf("pause %s", containerID))
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
		_, _, err := s.runRemoteDockerCommand(ctx, conn, fmt.Sprintf("unpause %s", containerID))
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
		forceFlag := ""
		if force {
			forceFlag = "-f"
		}
		_, _, err := s.runRemoteDockerCommand(ctx, conn, fmt.Sprintf("rm %s %s", forceFlag, containerID))
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
		stdout, stderr, err := s.runRemoteDockerCommand(ctx, conn, fmt.Sprintf("logs --tail %d %s", tail, containerID))
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
		stdout, _, err := s.runRemoteDockerCommand(ctx, conn, fmt.Sprintf(`stats --no-stream --format '{{json .}}' %s`, containerID))
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
		stdout, _, err := s.runRemoteDockerCommand(ctx, conn, "images --no-trunc --format '{{json .}}'")
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
				ID           string `json:"ID"`
				Repository   string `json:"Repository"`
				Tag          string `json:"Tag"`
				Image        string `json:"Image"`
				Size         string `json:"Size"`
				VirtualSize  string `json:"VirtualSize"`
				DiskUsage    string `json:"DiskUsage"`
				ContentSize  string `json:"ContentSize"`
				CreatedAt    string `json:"CreatedAt"`
				CreatedSince string `json:"CreatedSince"`
			}
			if err := json.Unmarshal([]byte(line), &item); err == nil {
				tag := ""
				if item.Repository != "" && item.Tag != "" {
					tag = fmt.Sprintf("%s:%s", item.Repository, item.Tag)
				} else if item.Image != "" {
					tag = item.Image
				} else if item.Repository != "" {
					tag = item.Repository
				} else {
					tag = "<none>:<none>"
				}

				sizeStr := item.Size
				if sizeStr == "" || sizeStr == "0B" || sizeStr == "0 B" || sizeStr == "N/A" {
					if item.VirtualSize != "" && item.VirtualSize != "0B" && item.VirtualSize != "0 B" && item.VirtualSize != "N/A" {
						sizeStr = item.VirtualSize
					} else if item.DiskUsage != "" {
						sizeStr = item.DiskUsage
					} else if item.ContentSize != "" {
						sizeStr = item.ContentSize
					}
				}
				sizeBytes, sizeMB := parseImageSize(sizeStr)
				createdUnix := parseDockerTime(item.CreatedAt)

				images = append(images, domain.DockerImage{
					ID:          item.ID,
					RepoTags:    []string{tag},
					Size:        sizeBytes,
					SizeMB:      sizeMB,
					VirtualSize: sizeBytes,
					Created:     createdUnix,
					CreatedStr:  item.CreatedAt,
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
		VirtualSize int64    `json:"VirtualSize"`
		Created     int64    `json:"Created"`
		Containers  int      `json:"Containers"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	images := make([]domain.DockerImage, 0, len(raw))
	for _, r := range raw {
		vSize := r.VirtualSize
		if vSize == 0 {
			vSize = r.Size
		}
		images = append(images, domain.DockerImage{
			ID:          r.ID,
			RepoTags:    r.RepoTags,
			Size:        r.Size,
			SizeMB:      float64(r.Size) / (1024 * 1024),
			VirtualSize: vSize,
			Created:     r.Created,
			Containers:  r.Containers,
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
		_, _, err := s.runRemoteDockerCommand(ctx, conn, fmt.Sprintf("pull %s", imageName))
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
		forceFlag := ""
		if force {
			forceFlag = "-f"
		}
		_, _, err := s.runRemoteDockerCommand(ctx, conn, fmt.Sprintf("rmi %s %s", forceFlag, imageID))
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

		stdout, stderr, err := s.runRemoteDockerCommand(ctx, conn, fmt.Sprintf("run %s", strings.Join(args, " ")))
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

		stdout, _, err := s.runRemoteDockerCommand(ctx, conn, "info --format '{{json .}}'")
		if err != nil {
			return false, fmt.Sprintf("Docker is not accessible or not running on remote host: %v", err), nil
		}

		var rawInfo struct {
			ServerVersion     string `json:"ServerVersion"`
			Containers        int    `json:"Containers"`
			ContainersRunning int    `json:"ContainersRunning"`
			ContainersPaused  int    `json:"ContainersPaused"`
			ContainersStopped int    `json:"ContainersStopped"`
			Images            int    `json:"Images"`
			OperatingSystem   string `json:"OperatingSystem"`
			OSType            string `json:"OSType"`
			Architecture      string `json:"Architecture"`
			NCPU              int    `json:"NCPU"`
			MemTotal          uint64 `json:"MemTotal"`
		}
		_ = json.Unmarshal([]byte(strings.TrimSpace(stdout)), &rawInfo)

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

		return true, "Successfully connected to remote Docker daemon via SSH!", info
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
	_, mb := parseImageSize(sizeStr)
	return mb
}

func parseImageSize(sizeStr string) (int64, float64) {
	s := strings.TrimSpace(strings.ToUpper(sizeStr))
	if s == "" || s == "-" || s == "N/A" {
		return 0, 0
	}

	// Remove possible commas or quotes
	s = strings.ReplaceAll(s, ",", "")

	var multiplier float64 = 1
	var numStr string

	if strings.HasSuffix(s, "TIB") || strings.HasSuffix(s, "TB") {
		multiplier = 1024 * 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(strings.TrimSuffix(s, "TIB"), "TB")
	} else if strings.HasSuffix(s, "GIB") || strings.HasSuffix(s, "GB") {
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(strings.TrimSuffix(s, "GIB"), "GB")
	} else if strings.HasSuffix(s, "MIB") || strings.HasSuffix(s, "MB") {
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(strings.TrimSuffix(s, "MIB"), "MB")
	} else if strings.HasSuffix(s, "KIB") || strings.HasSuffix(s, "KB") {
		multiplier = 1024
		numStr = strings.TrimSuffix(strings.TrimSuffix(s, "KIB"), "KB")
	} else if strings.HasSuffix(s, "B") {
		multiplier = 1
		numStr = strings.TrimSuffix(s, "B")
	} else {
		numStr = s
	}

	numStr = strings.TrimSpace(numStr)
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, 0
	}

	totalBytes := int64(val * multiplier)
	totalMB := float64(totalBytes) / (1024 * 1024)
	return totalBytes, totalMB
}

func parseDockerTime(timeStr string) int64 {
	timeStr = strings.TrimSpace(timeStr)
	if timeStr == "" || timeStr == "-" || timeStr == "N/A" {
		return 0
	}
	layouts := []string{
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 -0700 -0700",
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05",
		time.RFC3339,
		time.RFC3339Nano,
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, timeStr); err == nil {
			return t.Unix()
		}
	}
	if parts := strings.Split(timeStr, " "); len(parts) >= 2 {
		datePart := parts[0] + " " + parts[1]
		if t, err := time.Parse("2006-01-02 15:04:05", datePart); err == nil {
			return t.Unix()
		}
	}
	return 0
}

// -------------------------------------------------------------
// Networks Management (List, Create, Remove)
// -------------------------------------------------------------

type rawDockerNetwork struct {
	ID         string `json:"Id"`
	Name       string `json:"Name"`
	Created    string `json:"Created"`
	Scope      string `json:"Scope"`
	Driver     string `json:"Driver"`
	EnableIPv6 bool   `json:"EnableIPv6"`
	IPAM       struct {
		Config []struct {
			Subnet  string `json:"Subnet"`
			Gateway string `json:"Gateway"`
		} `json:"Config"`
	} `json:"IPAM"`
	Internal   bool `json:"Internal"`
	Containers map[string]struct {
		Name        string `json:"Name"`
		IPv4Address string `json:"IPv4Address"`
	} `json:"Containers"`
}

func (s *DockerService) ListNetworks(ctx context.Context, connectionID string) ([]domain.DockerNetwork, error) {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		// First try inspect for rich subnet, gateway, containers info
		stdout, _, err := s.runRemoteDockerCommand(ctx, conn, `network inspect $(docker network ls -q 2>/dev/null) 2>/dev/null || true`)
		if err == nil && strings.TrimSpace(stdout) != "" && strings.HasPrefix(strings.TrimSpace(stdout), "[") {
			var rawList []rawDockerNetwork
			if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &rawList); err == nil && len(rawList) > 0 {
				networks := make([]domain.DockerNetwork, 0, len(rawList))
				for _, r := range rawList {
					subnet := ""
					gateway := ""
					if len(r.IPAM.Config) > 0 {
						subnet = r.IPAM.Config[0].Subnet
						gateway = r.IPAM.Config[0].Gateway
					}
					contMap := make(map[string]string)
					for cId, cInfo := range r.Containers {
						cName := cInfo.Name
						if cName == "" {
							cName = cId
							if len(cName) > 12 {
								cName = cName[:12]
							}
						}
						contMap[cName] = cInfo.IPv4Address
					}

					networks = append(networks, domain.DockerNetwork{
						ID:              r.ID,
						Name:            r.Name,
						Driver:          r.Driver,
						Scope:           r.Scope,
						Subnet:          subnet,
						Gateway:         gateway,
						Internal:        r.Internal,
						EnableIPv6:      r.EnableIPv6,
						ContainersCount: len(r.Containers),
						Containers:      contMap,
						Created:         r.Created,
					})
				}
				return networks, nil
			}
		}

		// Fallback to basic network ls --format '{{json .}}'
		stdoutLs, _, errLs := s.runRemoteDockerCommand(ctx, conn, "network ls --no-trunc --format '{{json .}}'")
		if errLs != nil {
			return nil, fmt.Errorf("failed to list networks via ssh: %w", errLs)
		}

		var networks []domain.DockerNetwork
		scanner := bufio.NewScanner(strings.NewReader(stdoutLs))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var item struct {
				ID        string `json:"ID"`
				Name      string `json:"Name"`
				Driver    string `json:"Driver"`
				Scope     string `json:"Scope"`
				Internal  string `json:"Internal"`
				IPv6      string `json:"IPv6"`
				CreatedAt string `json:"CreatedAt"`
			}
			if err := json.Unmarshal([]byte(line), &item); err == nil {
				isInternal := strings.EqualFold(item.Internal, "true")
				isIPv6 := strings.EqualFold(item.IPv6, "true")
				networks = append(networks, domain.DockerNetwork{
					ID:         item.ID,
					Name:       item.Name,
					Driver:     item.Driver,
					Scope:      item.Scope,
					Internal:   isInternal,
					EnableIPv6: isIPv6,
					Created:    item.CreatedAt,
				})
			}
		}
		return networks, nil
	}

	client, baseURL := s.getLocalHTTPClient(conn)
	url := fmt.Sprintf("%s/v1.43/networks", baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rawList []rawDockerNetwork
	if err := json.NewDecoder(resp.Body).Decode(&rawList); err != nil {
		return nil, err
	}

	networks := make([]domain.DockerNetwork, 0, len(rawList))
	for _, r := range rawList {
		subnet := ""
		gateway := ""
		if len(r.IPAM.Config) > 0 {
			subnet = r.IPAM.Config[0].Subnet
			gateway = r.IPAM.Config[0].Gateway
		}
		contMap := make(map[string]string)
		for cId, cInfo := range r.Containers {
			cName := cInfo.Name
			if cName == "" {
				cName = cId
				if len(cName) > 12 {
					cName = cName[:12]
				}
			}
			contMap[cName] = cInfo.IPv4Address
		}

		networks = append(networks, domain.DockerNetwork{
			ID:              r.ID,
			Name:            r.Name,
			Driver:          r.Driver,
			Scope:           r.Scope,
			Subnet:          subnet,
			Gateway:         gateway,
			Internal:        r.Internal,
			EnableIPv6:      r.EnableIPv6,
			ContainersCount: len(r.Containers),
			Containers:      contMap,
			Created:         r.Created,
		})
	}
	return networks, nil
}

func (s *DockerService) CreateNetwork(ctx context.Context, connectionID string, req domain.CreateNetworkRequest) error {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return err
	}

	if req.Name == "" {
		return errors.New("network name is required")
	}
	if req.Driver == "" {
		req.Driver = "bridge"
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		var args []string
		args = append(args, "network", "create", "--driver", req.Driver)
		if req.Subnet != "" {
			args = append(args, "--subnet", req.Subnet)
		}
		if req.Gateway != "" {
			args = append(args, "--gateway", req.Gateway)
		}
		if req.Internal {
			args = append(args, "--internal")
		}
		if req.EnableIPv6 {
			args = append(args, "--ipv6")
		}
		args = append(args, req.Name)

		_, _, err := s.runRemoteDockerCommand(ctx, conn, strings.Join(args, " "))
		return err
	}

	client, baseURL := s.getLocalHTTPClient(conn)
	url := fmt.Sprintf("%s/v1.43/networks/create", baseURL)

	payload := map[string]interface{}{
		"Name":       req.Name,
		"Driver":     req.Driver,
		"Internal":   req.Internal,
		"EnableIPv6": req.EnableIPv6,
	}

	if req.Subnet != "" || req.Gateway != "" {
		ipamCfg := map[string]string{}
		if req.Subnet != "" {
			ipamCfg["Subnet"] = req.Subnet
		}
		if req.Gateway != "" {
			ipamCfg["Gateway"] = req.Gateway
		}
		payload["IPAM"] = map[string]interface{}{
			"Config": []map[string]string{ipamCfg},
		}
	}

	bodyBytes, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create network (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (s *DockerService) RemoveNetwork(ctx context.Context, connectionID, networkID string) error {
	conn, err := s.resolveConnection(ctx, connectionID)
	if err != nil {
		return err
	}

	if networkID == "" {
		return errors.New("network ID or Name is required")
	}

	// Protect default networks
	if networkID == "bridge" || networkID == "host" || networkID == "none" {
		return errors.New("cannot remove default system network (" + networkID + ")")
	}

	if conn.HostType == "ssh" || conn.RemoteHostID != nil {
		_, _, err := s.runRemoteDockerCommand(ctx, conn, fmt.Sprintf("network rm %s", networkID))
		return err
	}

	client, baseURL := s.getLocalHTTPClient(conn)
	url := fmt.Sprintf("%s/v1.43/networks/%s", baseURL, networkID)
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
		return fmt.Errorf("failed to remove network (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}
