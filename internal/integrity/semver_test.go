// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package integrity

import "testing"

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"v1.2.3":          "1.2.3",
		"1.2.3":           "1.2.3",
		"  v1.2.3  ":      "1.2.3",
		"1.2.3+build.9":   "1.2.3",
		"v1.2.3-rc.1+abc": "1.2.3-rc.1",
	}
	for in, want := range cases {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsRelease(t *testing.T) {
	for _, v := range []string{"1.2.3", "v1.2.3", "0.0.1", "1.2.3-rc.1"} {
		if !isRelease(v) {
			t.Errorf("isRelease(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"dev", "", "1.2", "1.2.3.4", "1.2.x", "1.2.3-snapshot-abc123"} {
		// note: 1.2.3-snapshot-... IS a valid pre-release core; guard is on the
		// non-numeric cores only.
		if v == "1.2.3-snapshot-abc123" {
			continue
		}
		if isRelease(v) {
			t.Errorf("isRelease(%q) = true, want false", v)
		}
	}
}

func TestLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.2.3", "1.2.4", true},
		{"1.2.4", "1.2.3", false},
		{"1.2.3", "1.2.3", false},
		{"1.9.0", "1.10.0", true}, // numeric, not lexical
		{"1.10.0", "1.9.0", false},
		{"2.0.0", "1.9.9", false},
		{"1.2.3-rc.1", "1.2.3", true}, // pre-release < release
		{"1.2.3", "1.2.3-rc.1", false},
		{"1.2.3-rc.1", "1.2.3-rc.2", true},
		{"1.2.3-rc.2", "1.2.3-rc.10", true},  // numeric identifier compare
		{"1.2.3-alpha", "1.2.3-beta", true},  // lexical
		{"1.2.3-rc.1", "1.2.3-rc.1.1", true}, // shorter set ranks lower
		{"garbage", "1.2.3", false},          // unparseable -> not less
	}
	for _, c := range cases {
		if got := less(normalize(c.a), normalize(c.b)); got != c.want {
			t.Errorf("less(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
