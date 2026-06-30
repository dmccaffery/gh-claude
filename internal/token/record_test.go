package token

import (
	"testing"
	"time"
)

func TestRecordNeedsRefresh(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		expires time.Time
		want    bool
	}{
		{"fresh 7d token", now.Add(7 * 24 * time.Hour), false},
		{"comfortably valid", now.Add(2 * time.Hour), false},
		{"just inside buffer", now.Add(reuseBuffer - time.Minute), true},
		{"exactly at expiry", now, true},
		{"already expired", now.Add(-time.Hour), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &Record{ExpiresAt: tc.expires}
			if got := r.needsRefresh(now); got != tc.want {
				t.Errorf("needsRefresh = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRecordExpiringSoon(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	if r := (&Record{ExpiresAt: now.Add(7 * 24 * time.Hour)}); r.expiringSoon(now) {
		t.Error("7-day token should not be flagged as expiring soon")
	}
	if r := (&Record{ExpiresAt: now.Add(2 * time.Hour)}); !r.expiringSoon(now) {
		t.Error("2-hour token should be flagged as expiring soon")
	}
}
