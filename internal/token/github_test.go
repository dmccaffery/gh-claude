package token

import (
	"errors"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

func TestParseExpiration(t *testing.T) {
	want := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	tests := []string{
		"2026-07-07 12:00:00 UTC",
		"2026-07-07 12:00:00 +0000",
		"2026-07-07T12:00:00Z",
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			got := parseExpiration(raw)
			if !got.Equal(want) {
				t.Errorf("parseExpiration(%q) = %v, want %v", raw, got, want)
			}
		})
	}
}

func TestParseExpirationUnknownFormat(t *testing.T) {
	if got := parseExpiration("not a date"); !got.IsZero() {
		t.Errorf("parseExpiration of garbage = %v, want zero time", got)
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"401", &api.HTTPError{StatusCode: 401}, true},
		{"403", &api.HTTPError{StatusCode: 403}, true},
		{"404", &api.HTTPError{StatusCode: 404}, false},
		{"500", &api.HTTPError{StatusCode: 500}, false},
		{"network error", errors.New("dial tcp: timeout"), false},
		{"nil", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsAuthError(tc.err); got != tc.want {
				t.Errorf("IsAuthError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
