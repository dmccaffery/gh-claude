package token

import "time"

// Host is the GitHub host this extension targets.
const Host = "github.com"

// storeKey is the key under which the token record is stored.
const storeKey = "github.com"

const (
	// reuseBuffer is how long before expiry a token is treated as unusable, so
	// we never hand Claude a token that is about to lapse.
	reuseBuffer = 5 * time.Minute
	// warnThreshold is the remaining lifetime under which we warn the user that
	// the token will need recreating soon.
	warnThreshold = 24 * time.Hour
)

// Record is the persisted token and its metadata.
type Record struct {
	Token     string    `json:"token"`
	Login     string    `json:"login"`
	Host      string    `json:"host"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// needsRefresh reports whether the token is expired or within the reuse buffer
// of expiring, and so must be re-provisioned.
func (r *Record) needsRefresh(now time.Time) bool {
	return !now.Before(r.ExpiresAt.Add(-reuseBuffer))
}

// expiringSoon reports whether the token is still usable but close enough to
// expiry that the user should be warned.
func (r *Record) expiringSoon(now time.Time) bool {
	return r.ExpiresAt.Sub(now) < warnThreshold
}
