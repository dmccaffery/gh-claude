// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package token

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// prefillNow is the fixed clock for name assertions: 2 July 2026 -> "02072026".
var prefillNow = time.Date(2026, time.July, 2, 15, 4, 5, 0, time.UTC)

func TestCreationURL(t *testing.T) {
	raw := CreationURL("my-laptop", prefillNow)
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("CreationURL produced an unparseable URL: %v", err)
	}
	if got := u.Scheme + "://" + u.Host + u.Path; got != creationBaseURL {
		t.Errorf("base URL = %q, want %q", got, creationBaseURL)
	}

	q := u.Query()
	if got := q.Get("expires_in"); got != strconv.Itoa(ExpiresInDays) {
		t.Errorf("expires_in = %q, want %q", got, strconv.Itoa(ExpiresInDays))
	}
	if got := q.Get("name"); got != "gh-claude (my-laptop 02072026)" {
		t.Errorf("name = %q, want %q", got, "gh-claude (my-laptop 02072026)")
	}
	for perm, level := range Permissions {
		if got := q.Get(perm); got != level {
			t.Errorf("permission %q = %q, want %q", perm, got, level)
		}
	}
	// Source code must be read-only — guard against an accidental write scope.
	if q.Get("contents") != "read" {
		t.Errorf("contents scope = %q, want read (no push)", q.Get("contents"))
	}
}

func TestTokenNameUsesShortHostname(t *testing.T) {
	// Everything from the first period on is noise ("mac.lan" and "mac.local"
	// are the same machine) and would waste the 40-char budget.
	if got, want := tokenName("mac.local", prefillNow), "gh-claude (mac 02072026)"; got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
}

func TestTokenNameCappedAt40KeepsDate(t *testing.T) {
	long := strings.Repeat("x", 100)
	name := tokenName(long, prefillNow)
	if len(name) > maxTokenNameLen {
		t.Errorf("name length = %d (%q), want <= %d", len(name), name, maxTokenNameLen)
	}
	// The cap must trim the hostname, never the date suffix — the date is what
	// keeps a renewal from colliding with the still-live previous token.
	if !strings.HasSuffix(name, " 02072026)") {
		t.Errorf("name = %q, want the date suffix to survive truncation", name)
	}
}

func TestTokenNameNoHostname(t *testing.T) {
	if got, want := tokenName("", prefillNow), "gh-claude (02072026)"; got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
}

func TestTokenNameDiffersAcrossDays(t *testing.T) {
	// A renewal happens while the previous token is still live; the day in the
	// name is what makes the pre-filled names distinct.
	if a, b := tokenName("mac", prefillNow), tokenName("mac", prefillNow.AddDate(0, 0, 7)); a == b {
		t.Errorf("names for renewals a week apart are identical: %q", a)
	}
}
