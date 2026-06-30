// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

// Package store persists a small secret blob in the most secure place available
// on the host: the macOS Keychain or the Linux Secret Service when present, and
// an encrypted, machine-bound file as a fallback (e.g. WSL2, where no Secret
// Service daemon is usually running).
//
// The store is deliberately "dumb": it reads and writes opaque bytes by key.
// Callers own the schema of what they store.
package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/99designs/keyring"
)

// ServiceName is the service/collection identifier used across backends.
const ServiceName = "gh-claude"

// Backend names surfaced to the caller for diagnostics.
const (
	BackendKeychain      = "keychain"
	BackendSecretService = "secret-service"
	BackendFile          = "encrypted-file"
	Backend1Password     = "1password"
)

// backend is the pluggable persistence implementation behind a Store. Each OS
// keychain, the encrypted file fallback, and the optional 1Password backend
// implement it. It mirrors the "opaque bytes by key" contract of Store itself.
type backend interface {
	get(key string) ([]byte, bool, error)
	set(key string, data []byte) error
	delete(key string) error
}

// Store wraps a single resolved persistence backend.
type Store struct {
	impl    backend
	backend string
	detail  string
}

// Backend reports which backend was selected (see the Backend* constants).
func (s *Store) Backend() string { return s.backend }

// Detail reports an optional human-readable qualifier for the backend (e.g. the
// 1Password vault), or "" when there is none.
func (s *Store) Detail() string { return s.detail }

// IsFileFallback reports whether the store fell back to the on-disk encrypted
// file because no OS keychain was usable. Callers may warn the user.
func (s *Store) IsFileFallback() bool { return s.backend == BackendFile }

// Options configures backend selection. The zero value preserves the default,
// env-driven behavior; CLI flags populate it to override the environment.
type Options struct {
	// UseOnePassword forces the 1Password backend on (e.g. the --op flag), in
	// addition to the GH_CLAUDE_STORE environment variable.
	UseOnePassword bool
	// Vault overrides the 1Password vault (e.g. the --vault flag). When empty it
	// falls back to GH_CLAUDE_OP_VAULT and then the default vault.
	Vault string
}

// New resolves and opens the best available backend for this host. When the user
// opts into 1Password (--op or GH_CLAUDE_STORE), that backend is used exclusively
// and an unreachable op is a hard error. Otherwise it prefers the native OS
// keychain and falls back to the encrypted file backend if the keychain cannot be
// reached (the common WSL2 case).
func New(opts Options) (*Store, error) {
	if opts.UseOnePassword || onePasswordRequested() {
		b, err := newOPBackend(opts)
		if err != nil {
			return nil, fmt.Errorf("1Password store requested but unavailable: %w", err)
		}
		return &Store{impl: b, backend: Backend1Password, detail: "vault: " + b.vault}, nil
	}

	var lastErr error
	for _, b := range preferredBackends() {
		ring, err := open(b)
		if err != nil {
			lastErr = err
			continue
		}
		// Opening is lazy; probe with a cheap list to confirm the backend is
		// actually reachable (e.g. Secret Service may be compiled in but have
		// no D-Bus daemon behind it on WSL2).
		if _, err := ring.Keys(); err != nil {
			lastErr = err
			continue
		}
		return &Store{impl: keyringBackend{ring: ring}, backend: backendName(b)}, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no usable secret storage backend found")
	}
	return nil, fmt.Errorf("opening secret store: %w", lastErr)
}

// Get returns the bytes stored under key. The boolean is false (with a nil
// error) when no item exists for the key.
func (s *Store) Get(key string) ([]byte, bool, error) { return s.impl.get(key) }

// Set stores data under key, replacing any existing value.
func (s *Store) Set(key string, data []byte) error { return s.impl.set(key, data) }

// Delete removes the item under key. Removing a missing key is not an error.
func (s *Store) Delete(key string) error { return s.impl.delete(key) }

// keyringBackend persists items in an OS keychain (macOS Keychain, Windows
// Credential Manager, Linux Secret Service) or the encrypted file fallback, via
// the 99designs/keyring library.
type keyringBackend struct {
	ring keyring.Keyring
}

func (k keyringBackend) get(key string) ([]byte, bool, error) {
	item, err := k.ring.Get(key)
	if errors.Is(err, keyring.ErrKeyNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return item.Data, true, nil
}

func (k keyringBackend) set(key string, data []byte) error {
	return k.ring.Set(keyring.Item{
		Key:         key,
		Data:        data,
		Label:       ServiceName,
		Description: "gh-claude scoped GitHub token",
	})
}

// delete removes the item under key. Removing a missing key is not an error.
// (The file backend reports a missing item as an OS not-exist error rather than
// keyring.ErrKeyNotFound, so both are tolerated.)
func (k keyringBackend) delete(key string) error {
	err := k.ring.Remove(key)
	if err == nil || errors.Is(err, keyring.ErrKeyNotFound) || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// preferredBackends returns the ordered list of backends to try for this OS,
// always ending with the file backend so there is a usable fallback.
func preferredBackends() []keyring.BackendType {
	switch runtime.GOOS {
	case "darwin":
		return []keyring.BackendType{keyring.KeychainBackend, keyring.FileBackend}
	case "windows":
		return []keyring.BackendType{keyring.WinCredBackend, keyring.FileBackend}
	default:
		return []keyring.BackendType{keyring.SecretServiceBackend, keyring.FileBackend}
	}
}

func backendName(b keyring.BackendType) string {
	switch b {
	case keyring.KeychainBackend:
		return BackendKeychain
	case keyring.SecretServiceBackend:
		return BackendSecretService
	default:
		return BackendFile
	}
}

func open(b keyring.BackendType) (keyring.Keyring, error) {
	return keyring.Open(keyring.Config{
		ServiceName:                    ServiceName,
		AllowedBackends:                []keyring.BackendType{b},
		KeychainTrustApplication:       true,
		KeychainAccessibleWhenUnlocked: true,
		LibSecretCollectionName:        ServiceName,
		FileDir:                        fileDir(),
		FilePasswordFunc:               filePassword,
	})
}

// fileDir is where the encrypted file backend keeps its items.
func fileDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, ServiceName)
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", ServiceName)
	}
	return filepath.Join(".", "."+ServiceName)
}

// filePassword derives the symmetric key for the encrypted file backend. It is
// bound to a stable machine identifier so the blob is not portable to other
// machines, with a generated per-install key file as a last resort. This is
// defense-in-depth (the machine id is not itself a secret); the file's 0600
// permissions are the primary protection. See README "Security model".
func filePassword(string) (string, error) {
	if id := machineID(); id != "" {
		sum := sha256.Sum256([]byte(ServiceName + ":" + id))
		return hex.EncodeToString(sum[:]), nil
	}
	return installKey()
}

func machineID() string {
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if b, err := os.ReadFile(p); err == nil {
			if s := strings.TrimSpace(string(b)); s != "" {
				return s
			}
		}
	}
	return ""
}

// installKey returns a random per-install key, generating and persisting it
// (0600) the first time. Used only when no machine id is available.
func installKey() (string, error) {
	path := filepath.Join(fileDir(), "install-key")
	if b, err := os.ReadFile(path); err == nil {
		if s := strings.TrimSpace(string(b)); s != "" {
			return s, nil
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	enc := hex.EncodeToString(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(enc), 0o600); err != nil {
		return "", err
	}
	return enc, nil
}
