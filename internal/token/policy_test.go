// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package token

import (
	"testing"
	"time"
)

func TestCheckGrant(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	sevenDays := time.Duration(ExpiresInDays) * 24 * time.Hour

	tests := []struct {
		name    string
		id      *Identity
		token   string
		wantErr bool
	}{
		{
			name:    "valid fine-grained 7-day token",
			id:      &Identity{HasExpiry: true, ExpiresAt: now.Add(sevenDays)},
			token:   "github_pat_abc123",
			wantErr: false,
		},
		{
			name:    "shorter lifetime accepted",
			id:      &Identity{HasExpiry: true, ExpiresAt: now.Add(24 * time.Hour)},
			token:   "github_pat_abc123",
			wantErr: false,
		},
		{
			name:    "expiry just inside tolerance accepted",
			id:      &Identity{HasExpiry: true, ExpiresAt: now.Add(sevenDays + expiryTolerance - time.Minute)},
			token:   "github_pat_abc123",
			wantErr: false,
		},
		{
			name:    "header present but unparseable accepted (fallback later)",
			id:      &Identity{HasExpiry: true}, // ExpiresAt zero
			token:   "github_pat_abc123",
			wantErr: false,
		},
		{
			name:    "classic token rejected",
			id:      &Identity{HasExpiry: true, ExpiresAt: now.Add(sevenDays)},
			token:   "ghp_classicToken",
			wantErr: true,
		},
		{
			name:    "no expiry rejected",
			id:      &Identity{HasExpiry: false},
			token:   "github_pat_abc123",
			wantErr: true,
		},
		{
			name:    "over-window lifetime rejected",
			id:      &Identity{HasExpiry: true, ExpiresAt: now.Add(30 * 24 * time.Hour)},
			token:   "github_pat_abc123",
			wantErr: true,
		},
		{
			name:    "just past tolerance rejected",
			id:      &Identity{HasExpiry: true, ExpiresAt: now.Add(sevenDays + expiryTolerance + time.Minute)},
			token:   "github_pat_abc123",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkGrant(tc.id, tc.token, now)
			if (err != nil) != tc.wantErr {
				t.Errorf("checkGrant() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
