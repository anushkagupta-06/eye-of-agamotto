// Package auth maps API keys to tenants.
//
// Only SHA-256 digests of keys are stored on disk. A presented key is hashed and looked up in a
// map keyed by the digest, so a lookup reveals nothing exploitable about the key itself and
// no plaintext key ever sits in a config file.
//
// File format, one entry per line (blank lines and '#' comments are ignored):
//
//	<tenant_id> <sha256-hex-of-key>
package auth

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
)

// ErrUnauthenticated is returned for a missing or unknown key.
var ErrUnauthenticated = errors.New("auth: invalid or missing API key")

var tenantRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// KeyStore is safe for concurrent use and supports atomic reloads.
type KeyStore struct {
	path    string
	entries atomic.Pointer[map[[sha256.Size]byte]string]
}

// Load reads the key file. An empty or malformed file is an error (fail closed).
func Load(path string) (*KeyStore, error) {
	k := &KeyStore{path: path}
	if err := k.Reload(); err != nil {
		return nil, err
	}
	return k, nil
}

// Reload re-reads the file. On error the previous keys stay active.
func (k *KeyStore) Reload() error {
	f, err := os.Open(k.path)
	if err != nil {
		return fmt.Errorf("auth: open key file: %w", err)
	}
	defer f.Close()
	m, err := parse(f)
	if err != nil {
		return err
	}
	k.entries.Store(&m)
	return nil
}

// Authenticate returns the tenant that owns key.
func (k *KeyStore) Authenticate(key string) (string, error) {
	if key == "" {
		return "", ErrUnauthenticated
	}
	tenant, ok := (*k.entries.Load())[sha256.Sum256([]byte(key))]
	if !ok {
		return "", ErrUnauthenticated
	}
	return tenant, nil
}

// Len reports how many keys are loaded.
func (k *KeyStore) Len() int { return len(*k.entries.Load()) }

func parse(r io.Reader) (map[[sha256.Size]byte]string, error) {
	m := make(map[[sha256.Size]byte]string)
	sc := bufio.NewScanner(r)
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Fields(text)
		if len(fields) != 2 {
			return nil, fmt.Errorf("auth: line %d: want '<tenant_id> <sha256-hex>'", line)
		}
		if !tenantRE.MatchString(fields[0]) {
			return nil, fmt.Errorf("auth: line %d: invalid tenant id %q", line, fields[0])
		}
		raw, err := hex.DecodeString(fields[1])
		if err != nil || len(raw) != sha256.Size {
			return nil, fmt.Errorf("auth: line %d: key digest must be %d hex chars", line, sha256.Size*2)
		}
		var digest [sha256.Size]byte
		copy(digest[:], raw)
		if _, dup := m[digest]; dup {
			return nil, fmt.Errorf("auth: line %d: duplicate key digest", line)
		}
		m[digest] = fields[0]
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("auth: read key file: %w", err)
	}
	if len(m) == 0 {
		return nil, errors.New("auth: key file contains no keys")
	}
	return m, nil
}
