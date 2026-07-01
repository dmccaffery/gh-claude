// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package token

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// tokenPrefix is GitHub's fixed prefix for fine-grained personal access tokens.
// Classic PATs (ghp_/gho_/…) do not carry it.
const tokenPrefix = "github_pat_"

// expiryTolerance absorbs clock skew and GitHub's rounding when comparing a
// token's expiry against the requested ExpiresInDays window. It sits comfortably
// below the next selectable preset (30 days), so a 7-day token passes and a
// 30-day one is still rejected.
const expiryTolerance = 12 * time.Hour

// checkGrant rejects a pasted token whose type or lifetime doesn't match what the
// creation form pre-populated. Per-permission scopes cannot be read back for
// fine-grained PATs (GitHub exposes no X-OAuth-Scopes header or introspection
// endpoint for them), so we enforce the token type instead and let GitHub enforce
// the actual permissions at use-time.
func checkGrant(id *Identity, token string, now time.Time) error {
	if !strings.HasPrefix(token, tokenPrefix) {
		return fmt.Errorf("that looks like a classic token; create a fine-grained token (starts with %q)", tokenPrefix)
	}
	if !id.HasExpiry {
		return errors.New("the token has no expiration; recreate it with a 7-day expiry")
	}
	// Skip the window check only when the header was present but unparseable
	// (ExpiresAt zero) — provision() then falls back to a computed 7-day expiry.
	if !id.ExpiresAt.IsZero() {
		maxLifetime := time.Duration(ExpiresInDays)*24*time.Hour + expiryTolerance
		if id.ExpiresAt.Sub(now) > maxLifetime {
			return fmt.Errorf("the token lasts longer than the required %d days; recreate it with a 7-day expiry", ExpiresInDays)
		}
	}
	return nil
}
