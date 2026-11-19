package services

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestSSHHostKeyEnrollmentAndRotation(t *testing.T) {
	makeKey := func() ssh.PublicKey {
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		key, err := ssh.NewPublicKey(pub)
		if err != nil {
			t.Fatal(err)
		}
		return key
	}
	file := filepath.Join(t.TempDir(), "known_hosts")
	strict, err := hostKeyCallback(file, true)
	if err != nil {
		t.Fatal(err)
	}
	addr := &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 22}
	key := makeKey()
	if err := strict("example.test:22", addr, key); err == nil {
		t.Fatal("strict mode enrolled unknown host")
	}
	tofu, err := hostKeyCallback(file, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := tofu("example.test:22", addr, key); err != nil {
		t.Fatal(err)
	}
	if err := strict("example.test:22", addr, key); err != nil {
		t.Fatal(err)
	}
	if err := tofu("example.test:22", addr, makeKey()); err == nil {
		t.Fatal("accepted changed host key")
	}
}
