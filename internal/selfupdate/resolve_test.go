package selfupdate

import (
	"strings"
	"testing"
)

func TestTagParseAcceptsReleaseFormat(t *testing.T) {
	cases := []struct {
		tag  string
		want releaseTag
	}{
		{"v0.5.0", releaseTag{0, 5, 0}},
		{"v1.2.3", releaseTag{1, 2, 3}},
		{"v10.20.30", releaseTag{10, 20, 30}},
	}

	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			got, err := parseReleaseTag(tc.tag)
			if err != nil {
				t.Fatalf("parseReleaseTag(%q) returned error: %v", tc.tag, err)
			}
			if got != tc.want {
				t.Fatalf("parseReleaseTag(%q) = %+v, want %+v", tc.tag, got, tc.want)
			}
		})
	}
}

func TestTagParseRejectsMalformedTags(t *testing.T) {
	cases := []string{
		"",
		"0.5.0",
		"v0.5",
		"v0.5.0.1",
		"v0.5.0-rc1",
		"va.b.c",
		"latest",
		"v0.6.1-0.20260709123456-abcdef123456",
	}

	for _, tag := range cases {
		t.Run(tag, func(t *testing.T) {
			if _, err := parseReleaseTag(tag); err == nil {
				t.Fatalf("parseReleaseTag(%q) accepted a malformed tag", tag)
			} else if !strings.Contains(err.Error(), "vMAJOR.MINOR.PATCH") {
				t.Fatalf("parseReleaseTag(%q) error %q does not name the expected format", tag, err)
			}
		})
	}
}

func TestTagCompareOrdersNumerically(t *testing.T) {
	skillGate := releaseTag{0, 5, 0}

	cases := []struct {
		tag       string
		belowGate bool
	}{
		{"v0.4.9", true},
		{"v0.5.0", false},
		{"v0.5.1", false},
		{"v0.10.0", false},
		{"v1.0.0", false},
		{"v0.0.9", true},
	}

	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			parsed, err := parseReleaseTag(tc.tag)
			if err != nil {
				t.Fatalf("parseReleaseTag(%q) returned error: %v", tc.tag, err)
			}
			if got := parsed.less(skillGate); got != tc.belowGate {
				t.Fatalf("%s below v0.5.0 = %t, want %t", tc.tag, got, tc.belowGate)
			}
		})
	}
}
