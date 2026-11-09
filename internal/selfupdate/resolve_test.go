package selfupdate

import (
	"fmt"
	"regexp"
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
		"v+1.2.3",
		"v-0.2.3",
		"v01.2.3",
		"v1.02.3",
		"v1.2.03",
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

// The grammar is the oracle here, not the parser: a release tag is `v` and
// three dot-separated numbers, each written with ASCII digits and no leading
// zero. Every candidate built from the fragments below is accepted exactly
// when the grammar accepts it.
func TestTagParseAgreesWithReleaseGrammar(t *testing.T) {
	grammar := regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	prefixes := []string{"v", "", "V", "vv", " v"}
	numbers := []string{"0", "1", "7", "10", "42", "", "01", "007", "+1", "-0", "-1", "1a", " 1", "1 ", "0x1", "1_0", "١"}

	checked, accepted := 0, 0
	for _, prefix := range prefixes {
		for _, major := range numbers {
			for _, minor := range numbers {
				for _, patch := range numbers {
					tag := prefix + major + "." + minor + "." + patch
					want := grammar.MatchString(tag)
					parsed, err := parseReleaseTag(tag)
					checked++
					if got := err == nil; got != want {
						t.Errorf("parseReleaseTag(%q) accepted = %t, the grammar says %t", tag, got, want)
						continue
					}
					if !want {
						continue
					}
					accepted++
					if text := fmt.Sprintf("v%d.%d.%d", parsed.major, parsed.minor, parsed.patch); text != tag {
						t.Errorf("parseReleaseTag(%q) = %+v, which reads back as %q", tag, parsed, text)
					}
				}
			}
		}
	}
	if checked != len(prefixes)*len(numbers)*len(numbers)*len(numbers) || accepted != 125 {
		t.Fatalf("checked %d candidates and accepted %d, want %d and 125", checked, accepted, len(prefixes)*len(numbers)*len(numbers)*len(numbers))
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
