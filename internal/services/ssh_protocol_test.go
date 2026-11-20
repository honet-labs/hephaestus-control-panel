package services

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/pkg/sftp"
	"go-hephaestus/internal/core/domain"
	"golang.org/x/crypto/ssh"
)

func TestSSHAndSFTPOverLoopback(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SSH_KNOWN_HOSTS", filepath.Join(root, "known_hosts"))
	t.Setenv("SSH_HOST_KEY_POLICY", "tofu")
	newSigner := func() ssh.Signer {
		_, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		signer, err := ssh.NewSignerFromKey(private)
		if err != nil {
			t.Fatal(err)
		}
		return signer
	}
	var active atomic.Value
	active.Store(newSigner())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				cfg := &ssh.ServerConfig{PasswordCallback: func(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
					if metadata.User() != "audit" || string(password) != "test-password" {
						return nil, fmt.Errorf("authentication failed")
					}
					return nil, nil
				}}
				cfg.AddHostKey(active.Load().(ssh.Signer))
				server, channels, requests, err := ssh.NewServerConn(conn, cfg)
				if err != nil {
					return
				}
				defer server.Close()
				go ssh.DiscardRequests(requests)
				for incoming := range channels {
					if incoming.ChannelType() != "session" {
						_ = incoming.Reject(ssh.UnknownChannelType, "session only")
						continue
					}
					channel, requests, err := incoming.Accept()
					if err != nil {
						continue
					}
					go func() {
						defer channel.Close()
						for req := range requests {
							switch req.Type {
							case "exec":
								_ = req.Reply(true, nil)
								_, _ = channel.Write([]byte("audit-ok"))
								_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
								return
							case "subsystem":
								var payload struct{ Name string }
								if ssh.Unmarshal(req.Payload, &payload) != nil || payload.Name != "sftp" {
									_ = req.Reply(false, nil)
									continue
								}
								_ = req.Reply(true, nil)
								server, err := sftp.NewServer(channel)
								if err == nil {
									_ = server.Serve()
									_ = server.Close()
								}
								return
							default:
								_ = req.Reply(false, nil)
							}
						}
					}()
				}
			}()
		}
	}()
	host, portString, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portString)
	password := "test-password"
	cfg := &domain.RemoteHostConfig{Host: host, Port: port, Username: "audit", AuthType: "password", Password: &password}
	s := &SSHService{connPool: make(map[string]*pooledSSH)}
	t.Cleanup(func() {
		for _, conn := range s.connPool {
			_ = conn.sftpClient.Close()
			_ = conn.client.Close()
		}
	})
	stdout, _, code, err := s.ExecuteCommand(cfg, "audit-command")
	if err != nil || code != 0 || stdout != "audit-ok" {
		t.Fatalf("SSH command: %q %d %v", stdout, code, err)
	}
	file := filepath.Join(root, "download.txt")
	if err := os.WriteFile(file, []byte("sftp-audit"), 0600); err != nil {
		t.Fatal(err)
	}
	reader, _, err := s.SftpDownloadWithConfig(cfg, filepath.ToSlash(file))
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil || string(data) != "sftp-audit" {
		t.Fatalf("SFTP download: %q %v", data, err)
	}
	active.Store(newSigner())
	if client, err := s.Dial(cfg); err == nil {
		client.Close()
		t.Fatal("changed server key accepted over real SSH handshake")
	}
}
