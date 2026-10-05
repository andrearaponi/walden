package consolidate

import (
	"regexp"
	"strings"
)

// Statement marks of a comparison.
const (
	MarkAdded      = "added"
	MarkChanged    = "changed"
	MarkUnchanged  = "unchanged"
	MarkUnverified = "unverified"
	MarkRemoved    = "removed"
)

// ComparedStatement is one statement of a pending feature (with its mark) or
// of a linked feature (without one).
type ComparedStatement struct {
	ID   string
	Text string
	Mark string
	// Previous is the recorded text of a changed statement.
	Previous string
}

// LinkedStatements are the current statements of a feature linked to a
// pending one. Features linked only through widely cited files carry none.
type LinkedStatements struct {
	Feature    string
	Links      []ScopeLink
	HubOnly    bool
	Statements []ComparedStatement
}

// Comparison puts a pending feature's statements next to the statements of
// every feature linked to it, so the material to compare is in one place.
type Comparison struct {
	Feature    string
	Kind       string
	Statements []ComparedStatement
	Linked     []LinkedStatements
}

// StatementTexts returns, for each definition line, the text that follows the
// identifier: the statement the identifier names. Fenced blocks are ignored.
func StatementTexts(body string) map[string]string {
	texts := map[string]string{}
	forEachLine(body, func(line string) {
		for _, pattern := range []*regexp.Regexp{criterionDefinition, nfrDefinition, constraintDefinition} {
			if location := pattern.FindStringSubmatchIndex(line); location != nil {
				id := line[location[2]:location[3]]
				if _, seen := texts[id]; !seen {
					texts[id] = strings.TrimSpace(line[location[1]:])
				}
				return
			}
		}
	})
	return texts
}

// ActiveStatements returns the texts of the feature's active statements.
func (f FeatureSnapshot) ActiveStatements() map[string]string {
	statements := map[string]string{}
	for _, id := range f.Definitions.Active() {
		statements[id] = f.Statements[id]
	}
	return statements
}

// Comparisons builds the comparison material of every pending change.
func Comparisons(snapshot Snapshot, record *Record, state State, scope []ScopeEntry) []Comparison {
	hubs := map[string]bool{}
	for _, hub := range hubFiles(snapshot) {
		hubs[hub.Path] = true
	}
	comparisons := []Comparison{}
	for _, change := range state.Pending {
		comparison := Comparison{Feature: change.Feature, Kind: change.Kind, Statements: []ComparedStatement{}}
		recorded, hasRecord := RecordFeature{}, false
		if record != nil {
			recorded, hasRecord = record.Features[change.Feature]
		}
		feature, exists := snapshot.Feature(change.Feature)
		if exists {
			for _, id := range feature.Definitions.Active() {
				mark, was := MarkAdded, ""
				if hasRecord && containsString(recorded.ActiveIDs, id) {
					previous, known := recorded.Statements[id]
					switch {
					case !known:
						mark = MarkUnverified
					case previous == feature.Statements[id]:
						mark = MarkUnchanged
					default:
						mark, was = MarkChanged, previous
					}
				}
				comparison.Statements = append(comparison.Statements, ComparedStatement{ID: id, Text: feature.Statements[id], Mark: mark, Previous: was})
			}
		}
		if hasRecord {
			for _, id := range recorded.ActiveIDs {
				if !exists || !feature.Definitions.Contains(id) {
					comparison.Statements = append(comparison.Statements, ComparedStatement{ID: id, Text: recorded.Statements[id], Mark: MarkRemoved})
				}
			}
		}
		for _, entry := range scope {
			if entry.Feature == change.Feature {
				continue
			}
			links := []ScopeLink{}
			hubOnly := true
			for _, link := range entry.Links {
				if link.Feature != change.Feature {
					continue
				}
				links = append(links, link)
				if link.Kind != LinkSharedFile || !hubs[link.Path] {
					hubOnly = false
				}
			}
			if len(links) == 0 {
				continue
			}
			linked := LinkedStatements{Feature: entry.Feature, Links: links, HubOnly: hubOnly}
			if other, ok := snapshot.Feature(entry.Feature); ok && !hubOnly {
				for _, id := range other.Definitions.Active() {
					linked.Statements = append(linked.Statements, ComparedStatement{ID: id, Text: other.Statements[id]})
				}
			}
			comparison.Linked = append(comparison.Linked, linked)
		}
		comparisons = append(comparisons, comparison)
	}
	return comparisons
}
