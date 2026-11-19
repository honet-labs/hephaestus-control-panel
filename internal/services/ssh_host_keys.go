package services

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var knownHostsMu sync.Mutex

// TOFU is explicit first-use enrollment; a changed key is always rejected.
// Strict mode requires keys provisioned through a trusted, out-of-band channel.
func hostKeyCallback(filename string, strict bool) (ssh.HostKeyCallback, error) {
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if strings.ContainsAny(hostname, "\r\n") {
			return fmt.Errorf("invalid SSH host name")
		}
		knownHostsMu.Lock()
		defer knownHostsMu.Unlock()
		verify, err := knownhosts.New(filename)
		if err != nil {
			return err
		}
		err = verify(hostname, remote, key)
		if err == nil {
			return nil
		}
		var keyError *knownhosts.KeyError
		if strict || !errors.As(err, &keyError) || len(keyError.Want) != 0 {
			return fmt.Errorf("SSH host key rejected for %s (%s); verify the fingerprint out-of-band: %w", hostname, ssh.FingerprintSHA256(key), err)
		}
		file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, writeErr := fmt.Fprintln(file, knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key))
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}, nil
}
