package token

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestCreationURL(t *testing.T) {
	raw := CreationURL("my-laptop")
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
	if got := q.Get("name"); got != "gh-claude (my-laptop)" {
		t.Errorf("name = %q, want %q", got, "gh-claude (my-laptop)")
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

func TestCreationURLNameCappedAt40(t *testing.T) {
	long := strings.Repeat("x", 100)
	q := mustQuery(t, CreationURL(long))
	if name := q.Get("name"); len(name) > maxTokenNameLen {
		t.Errorf("name length = %d (%q), want <= %d", len(name), name, maxTokenNameLen)
	}
}

func TestCreationURLNoHostname(t *testing.T) {
	q := mustQuery(t, CreationURL(""))
	if got := q.Get("name"); got != tokenNamePrefix {
		t.Errorf("name = %q, want %q", got, tokenNamePrefix)
	}
}

func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("unparseable URL %q: %v", raw, err)
	}
	return u.Query()
}
