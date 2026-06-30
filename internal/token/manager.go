package token

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// secretStore is the subset of internal/store.Store the manager depends on,
// kept as an interface so the provisioning logic is unit-testable.
type secretStore interface {
	Get(key string) ([]byte, bool, error)
	Set(key string, data []byte) error
	Delete(key string) error
}

// Manager loads, reuses, and provisions the scoped token record.
type Manager struct {
	Store secretStore
	Out   io.Writer        // where status/warning messages are written
	Now   func() time.Time // injectable clock (defaults to time.Now)
	// Validate verifies a token against GitHub; defaults to the package
	// Validate. Injectable so the reuse/provision flow is testable offline.
	Validate func(host, token string) (*Identity, error)
}

// Provisioner supplies the interactive pieces of creating a new token: opening
// the browser and reading the pasted value. Kept injectable for testing.
type Provisioner struct {
	Hostname  string
	OpenURL   func(url string) error
	ReadToken func() (string, error)
}

func (m *Manager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func (m *Manager) validate(host, tok string) (*Identity, error) {
	if m.Validate != nil {
		return m.Validate(host, tok)
	}
	return Validate(host, tok)
}

// Current returns the stored record (or nil if none) without provisioning.
func (m *Manager) Current() (*Record, error) {
	return m.load()
}

// Clear deletes the stored record.
func (m *Manager) Clear() error {
	return m.Store.Delete(storeKey)
}

// Ensure returns a usable token record. It reuses the stored token when it is
// unexpired and still accepted by GitHub, and otherwise provisions a new one
// interactively. Set forceRefresh to always provision.
func (m *Manager) Ensure(forceRefresh bool, p Provisioner) (*Record, error) {
	if !forceRefresh {
		rec, err := m.load()
		if err != nil {
			return nil, err
		}
		if rec != nil && !rec.needsRefresh(m.now()) {
			if reused, err := m.verifyReusable(rec); reused {
				return rec, err
			}
		}
	}
	return m.provision(p)
}

// verifyReusable probes a non-expired stored token. It returns (true, nil) when
// the token is good to reuse, (false, nil) when GitHub rejected it (so we should
// provision), and (true, nil) with a warning on transient failures (we trust the
// stored token rather than forcing recreation when offline).
func (m *Manager) verifyReusable(rec *Record) (bool, error) {
	_, err := m.validate(rec.Host, rec.Token)
	switch {
	case err == nil:
		if rec.expiringSoon(m.now()) {
			fmt.Fprintf(m.Out, "Note: the stored token expires %s — run `gh claude login` to refresh it.\n",
				rec.ExpiresAt.Format(time.RFC1123))
		}
		return true, nil
	case IsAuthError(err):
		fmt.Fprintln(m.Out, "Stored token was rejected by GitHub; creating a new one.")
		return false, nil
	default:
		fmt.Fprintf(m.Out, "Warning: could not verify the stored token (%v); using it anyway.\n", err)
		return true, nil
	}
}

// provision runs the interactive create-and-store flow.
func (m *Manager) provision(p Provisioner) (*Record, error) {
	url := CreationURL(p.Hostname)

	fmt.Fprintln(m.Out, "A new GitHub token is needed.")
	fmt.Fprintln(m.Out, "In the page that opens:")
	fmt.Fprintln(m.Out, "  1. Under \"Repository access\" choose \"All repositories\".")
	fmt.Fprintln(m.Out, "  2. Leave the pre-filled permissions (Contents: read, Issues: read/write, Pull requests: read/write).")
	fmt.Fprintln(m.Out, "  3. Click \"Generate token\" and copy the value (starts with github_pat_).")
	fmt.Fprintln(m.Out)

	if err := p.OpenURL(url); err != nil {
		fmt.Fprintf(m.Out, "Open this URL in your browser:\n  %s\n\n", url)
	} else {
		fmt.Fprintf(m.Out, "Opened your browser. If nothing happened, use this URL:\n  %s\n\n", url)
	}

	raw, err := p.ReadToken()
	if err != nil {
		return nil, err
	}
	tok := strings.TrimSpace(raw)
	if tok == "" {
		return nil, errors.New("no token was entered")
	}

	id, err := m.validate(Host, tok)
	if err != nil {
		if IsAuthError(err) {
			return nil, fmt.Errorf("the pasted token was rejected by GitHub: %w", authErrorHint(err))
		}
		return nil, fmt.Errorf("could not verify the pasted token: %w", err)
	}

	now := m.now()
	rec := &Record{
		Token:     tok,
		Login:     id.Login,
		Host:      Host,
		CreatedAt: now,
		ExpiresAt: id.ExpiresAt,
	}
	if rec.ExpiresAt.IsZero() {
		rec.ExpiresAt = now.Add(ExpiresInDays * 24 * time.Hour)
	}

	if err := m.save(rec); err != nil {
		return nil, err
	}
	fmt.Fprintf(m.Out, "Saved token for @%s (expires %s).\n", rec.Login, rec.ExpiresAt.Format(time.RFC1123))
	return rec, nil
}

func (m *Manager) load() (*Record, error) {
	data, ok, err := m.Store.Get(storeKey)
	if err != nil || !ok {
		return nil, err
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		// Corrupt/unreadable record: treat as absent so we re-provision.
		return nil, nil
	}
	if rec.Host == "" {
		rec.Host = Host
	}
	return &rec, nil
}

func (m *Manager) save(rec *Record) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return m.Store.Set(storeKey, data)
}
