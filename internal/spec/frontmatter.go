package spec

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var frontmatterPattern = regexp.MustCompile(`(?s)\A---\n(.*?)\n---\n?`)

type frontmatter struct {
	Status                       string
	ApprovedAt                   string
	LastModified                 string
	SourceRequirementsApprovedAt string
	SourceDesignApprovedAt       string
	Fields                       map[string]string
}

func parseFrontmatter(text string) (frontmatter, string, error) {
	match := frontmatterPattern.FindStringSubmatch(text)
	if match == nil {
		return frontmatter{}, "", fmt.Errorf("missing YAML frontmatter")
	}

	values := frontmatter{
		Fields: map[string]string{},
	}
	for _, line := range strings.Split(match[1], "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return frontmatter{}, "", fmt.Errorf("invalid frontmatter line %q", line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		values.Fields[key] = value
		switch key {
		case "status":
			values.Status = value
		case "approved_at":
			values.ApprovedAt = value
		case "last_modified":
			values.LastModified = value
		case "source_requirements_approved_at":
			values.SourceRequirementsApprovedAt = value
		case "source_design_approved_at":
			values.SourceDesignApprovedAt = value
		}
	}

	return values, text[len(match[0]):], nil
}

// ParseWaldenTimestamp parses a timestamp string into a time.Time value.
// It accepts RFC3339 and RFC3339Nano formats and normalizes to UTC.
func ParseWaldenTimestamp(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid timestamp %q: expected RFC3339 format", s)
		}
	}
	return t.UTC(), nil
}

// TimestampsEqual returns true if two timestamp strings represent the same instant.
func TimestampsEqual(a, b string) (bool, error) {
	ta, err := ParseWaldenTimestamp(a)
	if err != nil {
		return false, err
	}
	tb, err := ParseWaldenTimestamp(b)
	if err != nil {
		return false, err
	}
	return ta.Equal(tb), nil
}
