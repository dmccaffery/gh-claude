// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package token

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

// creationBaseURL is GitHub's fine-grained PAT creation page. It accepts the query
// parameters used below to pre-fill the form — see GitHub's docs on "pre-filling
// fine-grained personal access token details using URL parameters".
const creationBaseURL = "https://github.com/settings/personal-access-tokens/new"

// ExpiresInDays is the requested token lifetime (GitHub allows 1–366).
const ExpiresInDays = 7

// tokenNamePrefix is the display name prefix; the machine's short hostname and
// the creation date are appended. Fine-grained PAT names are unique per user,
// and a renewal typically happens while the previous token is still live, so
// the date suffix is what keeps the pre-filled name from colliding with the
// token it replaces. The combined name is capped at GitHub's 40-character limit.
const tokenNamePrefix = "gh-claude"

// tokenNameDateLayout renders the creation date as DDMMYYYY (e.g. 02072026).
// Same-day renewals still collide, but that is a deliberate trade for a name
// that stays short and scannable in GitHub's token list.
const tokenNameDateLayout = "02012006"

const maxTokenNameLen = 40

// Permissions is the fine-grained permission set the token is created with.
// Keys are GitHub fine-grained permission slugs; values are read|write|admin.
// This is the single source of truth for the token's scope — extend it here.
// (metadata:read is mandatory and added by GitHub automatically.)
var Permissions = map[string]string{
	"contents":      "read",  // source code: read-only — no push
	"issues":        "write", // write implies read
	"pull_requests": "write",
}

// CreationURL builds the pre-filled token-creation URL for the given machine
// hostname and creation time. The user still selects "All repositories" in the
// form (resource access cannot be preselected via URL).
func CreationURL(hostname string, now time.Time) string {
	v := url.Values{}
	v.Set("name", tokenName(hostname, now))
	v.Set("description", "Temporary read-only-code credential for Claude Code (gh-claude). Choose \"All repositories\".")
	v.Set("expires_in", strconv.Itoa(ExpiresInDays))
	for perm, level := range Permissions {
		v.Set(perm, level)
	}
	// url.Values.Encode sorts keys, so the result is deterministic.
	return creationBaseURL + "?" + v.Encode()
}

// tokenName returns the token display name: the prefix plus the short hostname
// (up to the first period — "mac.lan" and "mac.local" are the same machine)
// and the creation date. When the total exceeds GitHub's length limit the
// hostname is trimmed, never the date: the date suffix is what keeps a renewal
// from colliding with the still-live previous token.
func tokenName(hostname string, now time.Time) string {
	date := now.Format(tokenNameDateLayout)
	host, _, _ := strings.Cut(hostname, ".")
	if host == "" {
		return tokenNamePrefix + " (" + date + ")"
	}
	if max := maxTokenNameLen - len(tokenNamePrefix+" ( "+date+")"); len(host) > max {
		host = host[:max]
	}
	return tokenNamePrefix + " (" + host + " " + date + ")"
}
