// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package token

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// expirationHeader is returned by the GitHub API on requests authenticated with
// a PAT that has an expiry, giving the exact expiration timestamp.
const expirationHeader = "Github-Authentication-Token-Expiration"

// Identity is the result of validating a token against the API.
type Identity struct {
	Login     string
	ExpiresAt time.Time // zero when the token has no expiry or the header is absent
}

// Validate confirms a token authenticates to GitHub and returns the associated
// login and the token's expiry (read from GitHub's expiration response header).
func Validate(host, token string) (*Identity, error) {
	client, err := api.NewRESTClient(api.ClientOptions{
		Host:         host,
		AuthToken:    token,
		LogIgnoreEnv: true,
	})
	if err != nil {
		return nil, err
	}

	// Request returns a non-nil response only on a 2xx; non-2xx becomes a typed
	// *api.HTTPError, so reaching past this point means the token authenticated.
	resp, err := client.Request(http.MethodGet, "user", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var user struct {
		Login string `json:"login"`
	}
	if body, err := io.ReadAll(resp.Body); err == nil {
		_ = json.Unmarshal(body, &user)
	}

	id := &Identity{Login: user.Login}
	if raw := resp.Header.Get(expirationHeader); raw != "" {
		id.ExpiresAt = parseExpiration(raw)
	}
	return id, nil
}

// IsAuthError reports whether err is a GitHub rejection of the credential
// itself (401/403), as opposed to a transient/network failure.
func IsAuthError(err error) bool {
	var httpErr *api.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == http.StatusUnauthorized ||
			httpErr.StatusCode == http.StatusForbidden
	}
	return false
}

// expirationLayouts are the timestamp formats GitHub has used for the
// expiration header, tried in order.
var expirationLayouts = []string{
	"2006-01-02 15:04:05 MST",
	"2006-01-02 15:04:05 -0700",
	"2006-01-02 15:04:05 UTC",
	time.RFC3339,
}

// parseExpiration parses the expiration header value, returning the zero time
// if no known layout matches (callers fall back to a computed expiry).
func parseExpiration(raw string) time.Time {
	for _, layout := range expirationLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// authErrorHint wraps an auth error with actionable guidance.
func authErrorHint(err error) error {
	return fmt.Errorf("%w\nThe token may be expired, revoked, or missing the required permissions", err)
}
