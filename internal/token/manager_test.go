// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package token

import (
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

type fakeStore struct {
	data map[string][]byte
}

func newFakeStore() *fakeStore { return &fakeStore{data: map[string][]byte{}} }

func (f *fakeStore) Get(key string) ([]byte, bool, error) {
	v, ok := f.data[key]
	return v, ok, nil
}
func (f *fakeStore) Set(key string, data []byte) error { f.data[key] = data; return nil }
func (f *fakeStore) Delete(key string) error           { delete(f.data, key); return nil }

func (f *fakeStore) seed(t *testing.T, rec *Record) {
	t.Helper()
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	f.data[storeKey] = b
}

func (f *fakeStore) stored(t *testing.T) *Record {
	t.Helper()
	b, ok := f.data[storeKey]
	if !ok {
		return nil
	}
	var r Record
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return &r
}

var fixedNow = time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)

// recordingProvisioner builds a Provisioner that records calls and returns the
// given token from ReadToken.
func recordingProvisioner(tok string) (Provisioner, *int, *int) {
	opens, reads := new(int), new(int)
	return Provisioner{
		Hostname: "test-host",
		OpenURL:  func(string) error { *opens++; return nil },
		ReadToken: func() (string, error) {
			*reads++
			return tok, nil
		},
	}, opens, reads
}

// sequenceProvisioner returns the given tokens in order on successive ReadToken
// calls, repeating the last one once the sequence is exhausted. Used to exercise
// the re-prompt loop.
func sequenceProvisioner(tokens ...string) (Provisioner, *int, *int) {
	opens, reads := new(int), new(int)
	return Provisioner{
		Hostname: "test-host",
		OpenURL:  func(string) error { *opens++; return nil },
		ReadToken: func() (string, error) {
			i := *reads
			*reads++
			if i >= len(tokens) {
				i = len(tokens) - 1
			}
			return tokens[i], nil
		},
	}, opens, reads
}

func TestEnsureReusesValidToken(t *testing.T) {
	fs := newFakeStore()
	fs.seed(t, &Record{Token: "stored-tok", Login: "octocat", Host: Host, ExpiresAt: fixedNow.Add(7 * 24 * time.Hour)})

	prov, opens, reads := recordingProvisioner("new-tok")
	m := &Manager{
		Store: fs,
		Out:   io.Discard,
		Now:   func() time.Time { return fixedNow },
		Validate: func(host, tok string) (*Identity, error) {
			if tok != "stored-tok" {
				t.Errorf("validated %q, want stored-tok", tok)
			}
			return &Identity{Login: "octocat"}, nil
		},
	}

	rec, err := m.Ensure(false, prov)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Token != "stored-tok" {
		t.Errorf("token = %q, want stored-tok", rec.Token)
	}
	if *opens != 0 || *reads != 0 {
		t.Errorf("provisioner was invoked (opens=%d reads=%d); expected reuse", *opens, *reads)
	}
}

func TestEnsureProvisionsWhenExpired(t *testing.T) {
	fs := newFakeStore()
	fs.seed(t, &Record{Token: "old-tok", Login: "octocat", Host: Host, ExpiresAt: fixedNow.Add(-time.Hour)})

	wantExpiry := fixedNow.Add(7 * 24 * time.Hour)
	prov, opens, reads := recordingProvisioner("github_pat_new")
	m := &Manager{
		Store: fs,
		Out:   io.Discard,
		Now:   func() time.Time { return fixedNow },
		Validate: func(host, tok string) (*Identity, error) {
			if tok != "github_pat_new" {
				t.Errorf("validated %q, want the freshly pasted token", tok)
			}
			return &Identity{Login: "octocat", HasExpiry: true, ExpiresAt: wantExpiry}, nil
		},
	}

	rec, err := m.Ensure(false, prov)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Token != "github_pat_new" {
		t.Errorf("token = %q, want github_pat_new", rec.Token)
	}
	if !rec.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("expiry = %v, want %v (from API header)", rec.ExpiresAt, wantExpiry)
	}
	if *opens != 1 || *reads != 1 {
		t.Errorf("provisioner calls opens=%d reads=%d, want 1/1", *opens, *reads)
	}
	if got := fs.stored(t); got == nil || got.Token != "github_pat_new" {
		t.Errorf("new token was not persisted: %+v", got)
	}
}

func TestEnsureProvisionsWhenStoredTokenRejected(t *testing.T) {
	fs := newFakeStore()
	fs.seed(t, &Record{Token: "revoked-tok", Login: "octocat", Host: Host, ExpiresAt: fixedNow.Add(48 * time.Hour)})

	prov, opens, reads := recordingProvisioner("github_pat_new")
	m := &Manager{
		Store: fs,
		Out:   io.Discard,
		Now:   func() time.Time { return fixedNow },
		Validate: func(host, tok string) (*Identity, error) {
			if tok == "revoked-tok" {
				return nil, &api.HTTPError{StatusCode: 401}
			}
			return &Identity{Login: "octocat", HasExpiry: true}, nil
		},
	}

	rec, err := m.Ensure(false, prov)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Token != "github_pat_new" {
		t.Errorf("token = %q, want github_pat_new after rejection", rec.Token)
	}
	if *opens != 1 || *reads != 1 {
		t.Errorf("provisioner calls opens=%d reads=%d, want 1/1", *opens, *reads)
	}
}

func TestEnsureReusesOnTransientError(t *testing.T) {
	fs := newFakeStore()
	fs.seed(t, &Record{Token: "stored-tok", Login: "octocat", Host: Host, ExpiresAt: fixedNow.Add(48 * time.Hour)})

	prov, opens, reads := recordingProvisioner("new-tok")
	m := &Manager{
		Store: fs,
		Out:   io.Discard,
		Now:   func() time.Time { return fixedNow },
		Validate: func(host, tok string) (*Identity, error) {
			return nil, errors.New("dial tcp: network is unreachable")
		},
	}

	rec, err := m.Ensure(false, prov)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Token != "stored-tok" {
		t.Errorf("token = %q, want stored-tok reused despite transient error", rec.Token)
	}
	if *opens != 0 || *reads != 0 {
		t.Errorf("provisioner invoked on transient error (opens=%d reads=%d)", *opens, *reads)
	}
}

func TestEnsureForceRefresh(t *testing.T) {
	fs := newFakeStore()
	fs.seed(t, &Record{Token: "stored-tok", Login: "octocat", Host: Host, ExpiresAt: fixedNow.Add(7 * 24 * time.Hour)})

	prov, opens, reads := recordingProvisioner("github_pat_forced")
	m := &Manager{
		Store: fs,
		Out:   io.Discard,
		Now:   func() time.Time { return fixedNow },
		Validate: func(host, tok string) (*Identity, error) {
			return &Identity{Login: "octocat", HasExpiry: true}, nil
		},
	}

	rec, err := m.Ensure(true, prov)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Token != "github_pat_forced" {
		t.Errorf("token = %q, want forced new token", rec.Token)
	}
	if *opens != 1 || *reads != 1 {
		t.Errorf("provisioner calls opens=%d reads=%d, want 1/1", *opens, *reads)
	}
}

func TestProvisionRejectsEmptyToken(t *testing.T) {
	fs := newFakeStore()
	prov, _, _ := recordingProvisioner("   ")
	m := &Manager{
		Store:    fs,
		Out:      io.Discard,
		Now:      func() time.Time { return fixedNow },
		Validate: func(host, tok string) (*Identity, error) { return &Identity{Login: "x"}, nil },
	}
	if _, err := m.Ensure(false, prov); err == nil {
		t.Fatal("expected an error for an empty pasted token")
	}
}

func TestProvisionRejectsTokenWithoutExpiry(t *testing.T) {
	fs := newFakeStore()
	prov, _, reads := recordingProvisioner("github_pat_new")
	m := &Manager{
		Store: fs,
		Out:   io.Discard,
		Now:   func() time.Time { return fixedNow },
		// Header absent (HasExpiry false) -> a "No expiration" token; must be rejected.
		Validate: func(host, tok string) (*Identity, error) {
			return &Identity{Login: "octocat"}, nil
		},
	}
	if _, err := m.Ensure(false, prov); err == nil {
		t.Fatal("expected rejection of a token with no expiry")
	}
	if *reads != maxProvisionAttempts {
		t.Errorf("reads = %d, want %d (one per attempt)", *reads, maxProvisionAttempts)
	}
	if got := fs.stored(t); got != nil {
		t.Errorf("a rejected token must not be persisted: %+v", got)
	}
}

func TestProvisionFallsBackWhenExpiryUnparseable(t *testing.T) {
	fs := newFakeStore()
	prov, _, _ := recordingProvisioner("github_pat_new")
	m := &Manager{
		Store: fs,
		Out:   io.Discard,
		Now:   func() time.Time { return fixedNow },
		// Header present but unparseable: HasExpiry true, ExpiresAt zero -> computed fallback.
		Validate: func(host, tok string) (*Identity, error) {
			return &Identity{Login: "octocat", HasExpiry: true}, nil
		},
	}
	rec, err := m.Ensure(false, prov)
	if err != nil {
		t.Fatal(err)
	}
	want := fixedNow.Add(ExpiresInDays * 24 * time.Hour)
	if !rec.ExpiresAt.Equal(want) {
		t.Errorf("fallback expiry = %v, want %v", rec.ExpiresAt, want)
	}
}

func TestProvisionRetriesAfterRejectedToken(t *testing.T) {
	fs := newFakeStore()
	// First paste is a classic token (rejected on type), second is a valid one.
	prov, opens, reads := sequenceProvisioner("ghp_classic", "github_pat_good")
	wantExpiry := fixedNow.Add(ExpiresInDays * 24 * time.Hour)
	m := &Manager{
		Store: fs,
		Out:   io.Discard,
		Now:   func() time.Time { return fixedNow },
		Validate: func(host, tok string) (*Identity, error) {
			return &Identity{Login: "octocat", HasExpiry: true, ExpiresAt: wantExpiry}, nil
		},
	}
	rec, err := m.Ensure(true, prov)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Token != "github_pat_good" {
		t.Errorf("token = %q, want github_pat_good after retry", rec.Token)
	}
	if *opens != 1 {
		t.Errorf("opens = %d, want 1 (browser opened once)", *opens)
	}
	if *reads != 2 {
		t.Errorf("reads = %d, want 2 (one rejected, one accepted)", *reads)
	}
	if got := fs.stored(t); got == nil || got.Token != "github_pat_good" {
		t.Errorf("accepted token was not persisted: %+v", got)
	}
}
