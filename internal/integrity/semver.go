// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package integrity

import (
	"strconv"
	"strings"
)

// This is a deliberately small semver comparator scoped to the shape of our
// release tags: MAJOR.MINOR.PATCH with an optional -prerelease and an optional
// build-metadata suffix (the "plus" form, ignored per SemVer §10). It implements
// enough precedence for the minimum-version floor — numeric core comparison plus
// "a pre-release ranks below its released core" (SemVer §11) — without pulling in
// golang.org/x/mod.
// Exotic pre-release identifier ordering beyond a lexical/numeric compare is not
// something our tags exercise; swap in x/mod/semver if that ever changes.

// normalize strips a leading "v" and any +build metadata, returning the bare
// version core (possibly with a -prerelease suffix).
func normalize(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	return v
}

// sameVersion reports whether two version strings denote the same release, after
// normalization. Used for exact revocation matches.
func sameVersion(a, b string) bool {
	return normalize(a) == normalize(b)
}

// parsed is a version split into its numeric core and pre-release suffix.
type parsed struct {
	core [3]int
	pre  string // "" when this is a released (non-pre-release) version
	ok   bool   // false when the core did not parse as MAJOR.MINOR.PATCH
}

// parseVersion parses an already-normalized version string.
func parseVersion(v string) parsed {
	var p parsed
	core := v
	if before, after, ok := strings.Cut(v, "-"); ok {
		core, p.pre = before, after
	}
	fields := strings.Split(core, ".")
	if len(fields) != 3 {
		return p
	}
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return p
		}
		p.core[i] = n
	}
	p.ok = true
	return p
}

// isRelease reports whether v parses as a concrete release version (as opposed to
// "dev", a snapshot, or anything else the min-version floor cannot reason about).
// The floor is only enforced for versions that isRelease accepts.
func isRelease(v string) bool {
	return parseVersion(normalize(v)).ok
}

// less reports whether normalized version a orders strictly before b. Unparseable
// inputs are treated as not-less (callers gate the floor with isRelease, so this
// only matters defensively).
func less(a, b string) bool {
	pa, pb := parseVersion(a), parseVersion(b)
	if !pa.ok || !pb.ok {
		return false
	}
	for i := range 3 {
		if pa.core[i] != pb.core[i] {
			return pa.core[i] < pb.core[i]
		}
	}
	// Cores are equal: a pre-release ranks below the released version (SemVer §11).
	if pa.pre == pb.pre {
		return false
	}
	if pa.pre == "" { // a is released, b is a pre-release of the same core
		return false
	}
	if pb.pre == "" { // a is a pre-release, b is the release
		return true
	}
	return lessPrerelease(pa.pre, pb.pre)
}

// lessPrerelease compares two non-empty pre-release strings by SemVer §11
// identifier rules: numeric identifiers compare numerically, alphanumeric ones
// lexically, numeric ranks below alphanumeric, and a shorter run of otherwise
// equal identifiers ranks lower.
func lessPrerelease(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if as[i] == bs[i] {
			continue
		}
		an, aErr := strconv.Atoi(as[i])
		bn, bErr := strconv.Atoi(bs[i])
		switch {
		case aErr == nil && bErr == nil:
			return an < bn
		case aErr == nil: // numeric < alphanumeric
			return true
		case bErr == nil:
			return false
		default:
			return as[i] < bs[i]
		}
	}
	return len(as) < len(bs)
}
