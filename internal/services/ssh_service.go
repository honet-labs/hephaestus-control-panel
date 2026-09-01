package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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
	return ssh.Dial("tcp", addr, sshConfig)
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
