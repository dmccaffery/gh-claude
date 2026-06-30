package token

import (
	"net/url"
	"strconv"
)

// creationBaseURL is GitHub's fine-grained PAT creation page, which accepts the
// query parameters used below to pre-fill the form.
// https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens#pre-filling-fine-grained-personal-access-token-details-using-url-parameters
const creationBaseURL = "https://github.com/settings/personal-access-tokens/new"

// ExpiresInDays is the requested token lifetime (GitHub allows 1–366).
const ExpiresInDays = 7

// tokenNamePrefix is the display name prefix; the machine hostname is appended
// to help the user tell tokens apart in their GitHub settings. The combined
// name is capped at GitHub's 40-character limit.
const tokenNamePrefix = "gh-claude"

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
// hostname. The user still selects "All repositories" in the form (resource
// access cannot be preselected via URL).
func CreationURL(hostname string) string {
	v := url.Values{}
	v.Set("name", tokenName(hostname))
	v.Set("description", "Temporary read-only-code credential for Claude Code (gh-claude). Choose \"All repositories\".")
	v.Set("expires_in", strconv.Itoa(ExpiresInDays))
	for perm, level := range Permissions {
		v.Set(perm, level)
	}
	// url.Values.Encode sorts keys, so the result is deterministic.
	return creationBaseURL + "?" + v.Encode()
}

// tokenName returns the token display name, capped at GitHub's length limit.
func tokenName(hostname string) string {
	name := tokenNamePrefix
	if hostname != "" {
		name = tokenNamePrefix + " (" + hostname + ")"
	}
	if len(name) > maxTokenNameLen {
		name = name[:maxTokenNameLen]
	}
	return name
}
