package consolidate

import (
	"fmt"
	"regexp"
	"strings"
)

// FindingCoherenceReview reports an incomplete coherence review in the view.
const FindingCoherenceReview = "coherence-review"

// CoherenceHeading starts the view section where the agent records, for each
// pending feature, the outcome of comparing its change with the linked
// statements.
const CoherenceHeading = "## Coherence Review"

// CoherenceEntry is one `### <feature>` entry of the coherence review.
type CoherenceEntry struct {
	Feature    string
	Text       string
	References []Reference
}

// CoherenceReview is the parsed coherence section of the view body.
type CoherenceReview struct {
	Present bool
	Entries []CoherenceEntry
}

var htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)

// ParseCoherenceReview parses the coherence section of a view body: every
// `### <feature>` entry up to the next heading of level one or two. Comments,
// including multi-line ones, do not count as content or headings; citations follow ParseCitations, so only
// `feature#ID` in inline code outside fenced blocks counts.
func ParseCoherenceReview(body string) CoherenceReview {
	review := CoherenceReview{Entries: []CoherenceEntry{}}
	inSection, inFence, inComment := false, false, false
	lines := [][]string{}
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		// Commented examples, such as the one in the created view, never count.
		if !inFence && (inComment || strings.HasPrefix(trimmed, "<!--")) {
			inComment = !strings.Contains(trimmed, "-->")
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
		}
		heading := !inFence && strings.HasPrefix(trimmed, "#")
		switch {
		case heading && (strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ")):
			inSection = trimmed == CoherenceHeading
			review.Present = review.Present || inSection
			continue
		case heading && inSection && strings.HasPrefix(trimmed, "### "):
			name := strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "### ")), "`")
			review.Entries = append(review.Entries, CoherenceEntry{Feature: name})
			lines = append(lines, []string{})
			continue
		}
		if inSection && len(review.Entries) > 0 {
			lines[len(lines)-1] = append(lines[len(lines)-1], line)
		}
	}
	for i := range review.Entries {
		text := htmlComment.ReplaceAllString(strings.Join(lines[i], "\n"), "")
		review.Entries[i].Text = strings.TrimSpace(text)
		review.Entries[i].References = ParseCitations(text).References
	}
	return review
}

// CheckCoherence reports what the coherence review still lacks while contract
// changes are pending: an entry per pending feature, no empty, duplicate or
// extra entry, and a cited linked statement wherever the comparison shows
// one. It checks presence, coverage and citations, never correctness.
func CheckCoherence(review CoherenceReview, state State, comparisons []Comparison) []Finding {
	if state.Tracking != TrackingActive || len(state.Pending) == 0 {
		return nil
	}
	where := "the coherence review (" + CoherenceHeading + " in " + ViewPath + ")"
	pending, order := map[string]bool{}, []string{}
	for _, change := range state.Pending {
		if !pending[change.Feature] {
			pending[change.Feature] = true
			order = append(order, change.Feature)
		}
	}
	linked := map[string]map[string]bool{}
	for _, comparison := range comparisons {
		statements := map[string]bool{}
		for _, entry := range comparison.Linked {
			if entry.HubOnly {
				continue
			}
			for _, statement := range entry.Statements {
				statements[entry.Feature+"#"+statement.ID] = true
			}
		}
		linked[comparison.Feature] = statements
	}
	entries := map[string][]CoherenceEntry{}
	for _, entry := range review.Entries {
		entries[entry.Feature] = append(entries[entry.Feature], entry)
	}
	findings := []Finding{}
	add := func(feature, message string, args ...any) {
		findings = append(findings, Finding{Kind: FindingCoherenceReview, Feature: feature, Message: fmt.Sprintf(message, args...)})
	}
	for _, feature := range order {
		list := entries[feature]
		switch {
		case len(list) == 0:
			add(feature, "%s has no entry for %s: record the outcome of comparing its change with its linked statements", where, feature)
			continue
		case len(list) > 1:
			add(feature, "%s has more than one entry for %s", where, feature)
			continue
		case list[0].Text == "":
			add(feature, "the coherence entry for %s is empty: record the inconsistencies found, or none", feature)
			continue
		}
		if statements := linked[feature]; len(statements) > 0 {
			cited := false
			for _, reference := range list[0].References {
				cited = cited || statements[reference.Feature+"#"+reference.ID]
			}
			if !cited {
				add(feature, "the coherence entry for %s cites none of the linked statements in its comparison: cite those you compared as `feature#ID` in inline code", feature)
			}
		}
	}
	reported := map[string]bool{}
	for _, entry := range review.Entries {
		if !pending[entry.Feature] && !reported[entry.Feature] {
			reported[entry.Feature] = true
			add(entry.Feature, "%s has an entry for %s, which has no pending change", where, entry.Feature)
		}
	}
	return findings
}
