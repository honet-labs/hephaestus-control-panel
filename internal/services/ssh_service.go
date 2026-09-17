package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/repository"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SSHService struct {
	remoteRepo *repository.RemoteHostRepository
	connPool   map[string]*pooledSSH
	poolMu     sync.RWMutex
}

type pooledSSH struct {
	client     *ssh.Client
	sftpClient *sftp.Client
	lastUsed   time.Time
}

func NewSSHService(remoteRepo *repository.RemoteHostRepository) *SSHService {
	s := &SSHService{
		remoteRepo: remoteRepo,
		connPool:   make(map[string]*pooledSSH),
	}
	go s.idleConnectionCleaner()
	return s
}

func (s *SSHService) GetSSHClientConfig(cfg *domain.RemoteHostConfig) (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	if cfg.AuthType == "key" && cfg.SSHKey != nil && *cfg.SSHKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(*cfg.SSHKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else if cfg.Password != nil && *cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(*cfg.Password))
	} else {
		return nil, errors.New("no authentication credentials provided")
	}

	return &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Configurable for internal host access
		Timeout:         15 * time.Second,
		Config: ssh.Config{
			Ciphers: []string{
				"aes128-ctr", "aes192-ctr", "aes256-ctr",
				"aes128-gcm@openssh.com", "chacha20-poly1305@openssh.com",
				"aes128-cbc", "aes256-cbc", "3des-cbc",
			},
			KeyExchanges: []string{
				"curve25519-sha256", "curve25519-sha256@libssh.org",
				"ecdh-sha2-nistp256", "ecdh-sha2-nistp384", "ecdh-sha2-nistp521",
				"diffie-hellman-group14-sha256", "diffie-hellman-group14-sha1",
				"diffie-hellman-group-exchange-sha256", "diffie-hellman-group1-sha1",
			},
		},
	}, nil
}

func (s *SSHService) Dial(cfg *domain.RemoteHostConfig) (*ssh.Client, error) {
	sshConfig, err := s.GetSSHClientConfig(cfg)
	if err != nil {
		return nil, err
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	netDialer := &net.Dialer{
		Timeout:   15 * time.Second,
		KeepAlive: 15 * time.Second,
	}
	conn, err := netDialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	ncc, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return ssh.NewClient(ncc, chans, reqs), nil
}

func (s *SSHService) ExecuteCommand(cfg *domain.RemoteHostConfig, command string) (string, string, int, error) {
	client, err := s.Dial(cfg)
	if err != nil {
		return "", "", -1, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", "", -1, err
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	err = session.Run(command)
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			exitCode = 1
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode, nil
}

// ExecuteElevatedCommand executes a command with the sudo wrapper prepended
func (s *SSHService) ExecuteElevatedCommand(cfg *domain.RemoteHostConfig, command string) (string, string, int, error) {
	cmd := fmt.Sprintf(`%s
%s`, buildSudoWrapper(cfg.Password), command)
	return s.ExecuteCommand(cfg, cmd)
}

func (s *SSHService) TestConnection(cfg *domain.RemoteHostConfig) (bool, string) {
	client, err := s.Dial(cfg)
	if err != nil {
		return false, fmt.Sprintf("Connection failed: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return false, fmt.Sprintf("Failed to open SSH session: %v", err)
	}
	defer session.Close()

	return true, fmt.Sprintf("Successfully connected as %s@%s:%d", cfg.Username, cfg.Host, cfg.Port)
}

// SFTP Methods
func (s *SSHService) getSftpClient(cfg *domain.RemoteHostConfig) (*sftp.Client, *ssh.Client, error) {
	s.poolMu.Lock()
	defer s.poolMu.Unlock()

	poolKey := cfg.ID
	if entry, ok := s.connPool[poolKey]; ok {
		// Ping test
		if _, err := entry.sftpClient.Getwd(); err == nil {
			entry.lastUsed = time.Now()
			return entry.sftpClient, entry.client, nil
		}
		// Stale
		_ = entry.sftpClient.Close()
		_ = entry.client.Close()
		delete(s.connPool, poolKey)
	}

	sshClient, err := s.Dial(cfg)
	if err != nil {
		return nil, nil, err
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, nil, err
	}

	s.connPool[poolKey] = &pooledSSH{
		client:     sshClient,
		sftpClient: sftpClient,
		lastUsed:   time.Now(),
	}

	return sftpClient, sshClient, nil
}

func (s *SSHService) SftpListDir(ctx context.Context, hostID, remotePath string) ([]domain.SftpFileEntry, error) {
	cfg, err := s.remoteRepo.GetRawByID(ctx, hostID)
	if err != nil {
		return nil, err
	}

	sftpClient, _, err := s.getSftpClient(cfg)
	if err != nil {
		return nil, err
	}

	cleanPath := sanitizeRemotePath(remotePath)
	files, err := sftpClient.ReadDir(cleanPath)
	if err != nil {
		return nil, err
	}

	var entries []domain.SftpFileEntry
	for _, f := range files {
		entries = append(entries, domain.SftpFileEntry{
			Name:    f.Name(),
			IsDir:   f.IsDir(),
			Size:    f.Size(),
			ModTime: f.ModTime(),
		})
	}
	return entries, nil
}

func (s *SSHService) SftpUpload(ctx context.Context, hostID, remotePath string, reader io.Reader) error {
	cfg, err := s.remoteRepo.GetRawByID(ctx, hostID)
	if err != nil {
		return err
	}

	sftpClient, _, err := s.getSftpClient(cfg)
	if err != nil {
		return err
	}

	cleanPath := sanitizeRemotePath(remotePath)
	dir := filepath.Dir(cleanPath)
	_ = sftpClient.MkdirAll(dir)

	dstFile, err := sftpClient.Create(cleanPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, reader)
	return err
}

func (s *SSHService) SftpDownload(ctx context.Context, hostID, remotePath string) (io.ReadCloser, int64, error) {
	cfg, err := s.remoteRepo.GetRawByID(ctx, hostID)
	if err != nil {
		return nil, 0, err
	}

	sftpClient, _, err := s.getSftpClient(cfg)
	if err != nil {
		return nil, 0, err
	}

	cleanPath := sanitizeRemotePath(remotePath)
	srcFile, err := sftpClient.Open(cleanPath)
	if err != nil {
		return nil, 0, err
	}

	stat, err := srcFile.Stat()
	if err != nil {
		_ = srcFile.Close()
		return nil, 0, err
	}

	return srcFile, stat.Size(), nil
}

func (s *SSHService) SftpDownloadWithConfig(cfg *domain.RemoteHostConfig, remotePath string) (io.ReadCloser, int64, error) {
	sftpClient, _, err := s.getSftpClient(cfg)
	if err != nil {
		return nil, 0, err
	}

	cleanPath := sanitizeRemotePath(remotePath)
	srcFile, err := sftpClient.Open(cleanPath)
	if err != nil {
		return nil, 0, err
	}

	stat, err := srcFile.Stat()
	if err != nil {
		_ = srcFile.Close()
		return nil, 0, err
	}

	return srcFile, stat.Size(), nil
}


func (s *SSHService) SftpTransferRemoteToRemote(ctx context.Context, srcHostID, srcPath, dstHostID, dstPath string) error {
	srcCfg, err := s.remoteRepo.GetRawByID(ctx, srcHostID)
	if err != nil {
		return fmt.Errorf("source host not found: %w", err)
	}
	dstCfg, err := s.remoteRepo.GetRawByID(ctx, dstHostID)
	if err != nil {
		return fmt.Errorf("destination host not found: %w", err)
	}

	srcSftp, _, err := s.getSftpClient(srcCfg)
	if err != nil {
		return fmt.Errorf("failed to connect to source SFTP: %w", err)
	}
	dstSftp, _, err := s.getSftpClient(dstCfg)
	if err != nil {
		return fmt.Errorf("failed to connect to destination SFTP: %w", err)
	}

	cleanSrc := sanitizeRemotePath(srcPath)
	cleanDst := sanitizeRemotePath(dstPath)

	srcFile, err := srcSftp.Open(cleanSrc)
	if err != nil {
		return fmt.Errorf("failed to open source file '%s': %w", cleanSrc, err)
	}
	defer srcFile.Close()

	dstDir := filepath.Dir(cleanDst)
	_ = dstSftp.MkdirAll(dstDir)

	dstFile, err := dstSftp.Create(cleanDst)
	if err != nil {
		return fmt.Errorf("failed to create destination file '%s': %w", cleanDst, err)
	}
	defer dstFile.Close()

	buf := make([]byte, 128*1024)
	_, err = io.CopyBuffer(dstFile, srcFile, buf)
	return err
}

func (s *SSHService) SftpUploadWithConfig(cfg *domain.RemoteHostConfig, remotePath string, reader io.Reader) error {
	client, err := s.Dial(cfg)
	if err != nil {
		return err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	cleanPath := sanitizeRemotePath(remotePath)
	dir := filepath.ToSlash(filepath.Dir(cleanPath))
	_ = sftpClient.MkdirAll(dir)

	dstFile, err := sftpClient.Create(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file '%s': %w", cleanPath, err)
	}
	defer dstFile.Close()

	buf := make([]byte, 128*1024)
	_, err = io.CopyBuffer(dstFile, reader, buf)
	return err
}

func buildSudoWrapper(password *string) string {
	var escapedPass string
	if password != nil && *password != "" {
		escapedPass = strings.ReplaceAll(*password, "'", "'\\''")
	}
	return fmt.Sprintf(`_P='%s'
_run_sudo() {
    if [ "$(id -u)" -eq 0 ]; then
        "$@"
    elif [ -n "$_P" ]; then
        echo "$_P" | sudo -S -p '' "$@"
    elif sudo -n true 2>/dev/null; then
        sudo -n "$@"
    else
        "$@"
    fi
}`, escapedPass)
}

func (s *SSHService) ReadFile(cfg *domain.RemoteHostConfig, remotePath string) (string, error) {
	client, err := s.Dial(cfg)
	if err != nil {
		return "", fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	cleanPath := sanitizeRemotePath(remotePath)
	sftpClient, err := sftp.NewClient(client)
	if err == nil {
		defer sftpClient.Close()
		f, err := sftpClient.Open(cleanPath)
		if err == nil {
			defer f.Close()
			data, err := io.ReadAll(f)
			if err == nil {
				return string(data), nil
			}
		}
	}

	// Fallback to command execution with sudo elevation
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	escapedCleanPath := strings.ReplaceAll(cleanPath, "'", "'\\''")
	sudoWrapper := buildSudoWrapper(cfg.Password)

	cmd := fmt.Sprintf(`%s
_run_sudo cat '%s'`, sudoWrapper, escapedCleanPath)

	if err := session.Run(cmd); err != nil {
		return "", fmt.Errorf("failed to read remote file '%s': %w (stderr: %s)", cleanPath, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func (s *SSHService) WriteFile(cfg *domain.RemoteHostConfig, remotePath string, content string) error {
	client, err := s.Dial(cfg)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	cleanPath := sanitizeRemotePath(remotePath)

	// 1. Direct write attempt via SFTP (succeeds if user is root or owns the target path)
	sftpClient, sftpErr := sftp.NewClient(client)
	if sftpErr == nil {
		dir := filepath.ToSlash(filepath.Dir(cleanPath))
		_ = sftpClient.MkdirAll(dir)

		f, err := sftpClient.Create(cleanPath)
		if err == nil {
			_, writeErr := f.Write([]byte(content))
			_ = f.Close()
			if writeErr == nil {
				_ = sftpClient.Close()
				return nil
			}
		}
	}

	// 2. Privileged write via temporary file in /tmp and sudo elevation
	// /tmp is world-writable (mode 1777), allowing any authenticated user to create a temporary file.
	tmpPath := fmt.Sprintf("/tmp/.hcp_write_%d.tmp", time.Now().UnixNano())
	var uploadedToTmp bool

	if sftpClient != nil {
		tmpFile, err := sftpClient.Create(tmpPath)
		if err == nil {
			if _, err := tmpFile.Write([]byte(content)); err == nil {
				uploadedToTmp = true
			}
			_ = tmpFile.Close()
		}
		_ = sftpClient.Close()
	}

	if !uploadedToTmp {
		session, err := client.NewSession()
		if err != nil {
			return fmt.Errorf("failed to create SSH session for temp file: %w", err)
		}
		session.Stdin = strings.NewReader(content)
		var stderr bytes.Buffer
		session.Stderr = &stderr
		if err := session.Run(fmt.Sprintf("cat > '%s'", tmpPath)); err != nil {
			_ = session.Close()
			return fmt.Errorf("failed to write temporary file '%s': %w (stderr: %s)", tmpPath, err, strings.TrimSpace(stderr.String()))
		}
		_ = session.Close()
	}

	// 3. Move/copy temp file to destination using sudo elevation
	session, err := client.NewSession()
	if err != nil {
		// Clean up tmpPath
		cleanupSess, _ := client.NewSession()
		if cleanupSess != nil {
			_ = cleanupSess.Run(fmt.Sprintf("rm -f '%s'", tmpPath))
			_ = cleanupSess.Close()
		}
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	var stderrBuf bytes.Buffer
	session.Stderr = &stderrBuf

	escapedCleanPath := strings.ReplaceAll(cleanPath, "'", "'\\''")
	sudoWrapper := buildSudoWrapper(cfg.Password)

	cmd := fmt.Sprintf(`%s
dir=$(dirname '%s')
_run_sudo sh -c 'mkdir -p "$1" && cp -f "$2" "$3" && chmod 644 "$3" && (chgrp --reference="$1" "$3" 2>/dev/null || true)' -- "$dir" '%s' '%s'
status=$?
rm -f '%s'
exit $status`, sudoWrapper, escapedCleanPath, tmpPath, escapedCleanPath, tmpPath)

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to write file '%s': %w (stderr: %s)", cleanPath, err, strings.TrimSpace(stderrBuf.String()))
	}
	return nil
}

// DeleteFile deletes a remote file via SFTP or with sudo elevation fallback
func (s *SSHService) DeleteFile(cfg *domain.RemoteHostConfig, remotePath string) error {
	client, err := s.Dial(cfg)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	cleanPath := sanitizeRemotePath(remotePath)

	// 1. Direct delete attempt via SFTP (succeeds if user is root or owns the file)
	sftpClient, sftpErr := sftp.NewClient(client)
	if sftpErr == nil {
		defer sftpClient.Close()
		if err := sftpClient.Remove(cleanPath); err == nil {
			return nil
		}
	}

	// 2. Privileged delete via shell session with sudo elevation
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	var stderrBuf bytes.Buffer
	session.Stderr = &stderrBuf

	escapedCleanPath := strings.ReplaceAll(cleanPath, "'", "'\\''")
	sudoWrapper := buildSudoWrapper(cfg.Password)

	cmd := fmt.Sprintf(`%s
_run_sudo rm -f '%s'`, sudoWrapper, escapedCleanPath)

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to delete file '%s': %w (stderr: %s)", cleanPath, err, strings.TrimSpace(stderrBuf.String()))
	}
	return nil
}

// CheckPath checks if a remote path exists, and whether it is a directory or regular file
func (s *SSHService) CheckPath(cfg *domain.RemoteHostConfig, remotePath string) (exists bool, isDir bool, err error) {
	client, err := s.Dial(cfg)
	if err != nil {
		return false, false, fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	cleanPath := sanitizeRemotePath(remotePath)
	sftpClient, err := sftp.NewClient(client)
	if err == nil {
		defer sftpClient.Close()
		stat, err := sftpClient.Stat(cleanPath)
		if err == nil {
			return true, stat.IsDir(), nil
		}
	}

	// Fallback via shell session with sudo elevation
	session, err := client.NewSession()
	if err != nil {
		return false, false, fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	escapedCleanPath := strings.ReplaceAll(cleanPath, "'", "'\\''")
	sudoWrapper := buildSudoWrapper(cfg.Password)

	cmd := fmt.Sprintf(`%s
if _run_sudo test -d '%s'; then
    echo 'DIR'
elif _run_sudo test -f '%s' || _run_sudo test -e '%s'; then
    echo 'FILE'
else
    exit 1
fi`, sudoWrapper, escapedCleanPath, escapedCleanPath, escapedCleanPath)

	out, err := session.Output(cmd)
	if err != nil {
		return false, false, fmt.Errorf("remote path '%s' does not exist or is not accessible", cleanPath)
	}

	res := strings.TrimSpace(string(out))
	if res == "DIR" {
		return true, true, nil
	}
	return true, false, nil
}

// SftpListDirWithConfig lists files in a remote directory directly using config credentials
func (s *SSHService) SftpListDirWithConfig(cfg *domain.RemoteHostConfig, remotePath string) ([]domain.SftpFileEntry, error) {
	client, err := s.Dial(cfg)
	if err != nil {
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	cleanPath := sanitizeRemotePath(remotePath)
	sftpClient, err := sftp.NewClient(client)
	if err == nil {
		defer sftpClient.Close()
		files, err := sftpClient.ReadDir(cleanPath)
		if err == nil {
			var entries []domain.SftpFileEntry
			for _, f := range files {
				entries = append(entries, domain.SftpFileEntry{
					Name:    f.Name(),
					IsDir:   f.IsDir(),
					Size:    f.Size(),
					ModTime: f.ModTime(),
				})
			}
			return entries, nil
		}
	}

	// Fallback to ls command with sudo elevation
	session, sessErr := client.NewSession()
	if sessErr != nil {
		return nil, fmt.Errorf("failed to create SSH session: %w", sessErr)
	}
	defer session.Close()

	escapedCleanPath := strings.ReplaceAll(cleanPath, "'", "'\\''")
	sudoWrapper := buildSudoWrapper(cfg.Password)

	cmd := fmt.Sprintf(`%s
_run_sudo ls -1Ap '%s'`, sudoWrapper, escapedCleanPath)

	out, runErr := session.Output(cmd)
	if runErr != nil {
		return nil, fmt.Errorf("failed to read remote directory '%s': %w", cleanPath, runErr)
	}

	var entries []domain.SftpFileEntry
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		isDir := strings.HasSuffix(line, "/")
		name := strings.TrimSuffix(line, "/")
		entries = append(entries, domain.SftpFileEntry{
			Name:    name,
			IsDir:   isDir,
			ModTime: time.Now(),
		})
	}
	return entries, nil
}

func (s *SSHService) idleConnectionCleaner() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		s.poolMu.Lock()
		now := time.Now()
		for k, entry := range s.connPool {
			if now.Sub(entry.lastUsed) > 5*time.Minute {
				_ = entry.sftpClient.Close()
				_ = entry.client.Close()
				delete(s.connPool, k)
			}
		}
		s.poolMu.Unlock()
	}
}

func sanitizeRemotePath(p string) string {
	if p == "" {
		return "/"
	}
	p = strings.ReplaceAll(p, "\x00", "")
	return filepath.ToSlash(filepath.Clean(p))
}
