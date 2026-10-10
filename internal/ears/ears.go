package ears

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Supported EARS form names.
const (
	FormUbiquitous  = "ubiquitous"
	FormEventDriven = "event-driven"
	FormStateDriven = "state-driven"
	FormOptional    = "optional"
	FormUnwanted    = "unwanted"
	FormComplex     = "complex"
)

// Kinds of condition clause: WHERE opens an optional-feature clause, WHILE or
// DURING a state clause, WHEN an event clause, and IF an unwanted-behavior
// clause that THEN closes.
const (
	ClauseOptional = "optional"
	ClauseState    = "state"
	ClauseEvent    = "event"
	ClauseUnwanted = "unwanted"
)

// ParsedCriterion is the result of parsing one acceptance criterion.
type ParsedCriterion struct {
	ID       string
	Raw      string
	Form     string
	Valid    bool
	Errors   []string
	Warnings []string
	// Clauses lists the kinds of a valid criterion's condition clauses in
	// text order.
	Clauses []string
}

var acLinePattern = regexp.MustCompile("(?m)^\\d+\\.\\s+`(R\\d+\\.AC\\d+)`\\s+(.*)")

// clauseKeywords maps each condition keyword to the kind of clause it opens.
var clauseKeywords = []struct {
	keyword string
	kind    string
}{
	{"WHERE", ClauseOptional},
	{"WHILE", ClauseState},
	{"DURING", ClauseState},
	{"WHEN", ClauseEvent},
	{"IF", ClauseUnwanted},
}

// formOfKind names the form of a criterion whose clauses are all of one kind.
var formOfKind = map[string]string{
	ClauseOptional: FormOptional,
	ClauseState:    FormStateDriven,
	ClauseEvent:    FormEventDriven,
	ClauseUnwanted: FormUnwanted,
}

// clauseRank is the place of each clause kind in the EARS order: optional
// feature, state, then event or unwanted behavior.
var clauseRank = map[string]int{
	ClauseOptional: 1,
	ClauseState:    2,
	ClauseEvent:    3,
	ClauseUnwanted: 3,
}

const orderWarning = "clauses out of EARS order; EARS orders them WHERE → WHILE → WHEN/IF"

// pronouns are the subjects that name no component, in ASCII capitals.
var pronouns = []string{"IT", "THEY"}

// emptyClauseError names an empty clause by the keyword that opens it.
var emptyClauseError = map[string]string{
	"WHERE":  "empty feature slot after WHERE",
	"WHILE":  "empty precondition slot after WHILE",
	"DURING": "empty precondition slot after DURING",
	"WHEN":   "empty trigger slot after WHEN",
	"IF":     "empty trigger slot between IF and THEN",
}

// clause is one condition clause before SHALL: its keyword, its kind, and
// the offsets where the keyword starts and ends.
type clause struct {
	keyword string
	kind    string
	start   int
	end     int
}

// ParseAllCriteria extracts and classifies all acceptance criteria from a
// requirements.md body. It matches lines of the form:
//
//  1. `R1.AC1` WHEN [trigger], the system SHALL [response]
func ParseAllCriteria(body string) []ParsedCriterion {
	matches := acLinePattern.FindAllStringSubmatch(body, -1)
	results := make([]ParsedCriterion, 0, len(matches))
	for _, match := range matches {
		id := match[1]
		text := strings.TrimSpace(match[2])
		results = append(results, ParseCriterion(id, text))
	}
	return results
}

// ParseCriterion classifies a single acceptance criterion text into an EARS form.
// It validates keyword scaffolding only, not natural-language content.
func ParseCriterion(id, text string) ParsedCriterion {
	result := ParsedCriterion{
		ID:  id,
		Raw: text,
	}

	shalls := keywordOffsets(text, "SHALL")
	if len(shalls) == 0 {
		result.Errors = append(result.Errors, "missing required keyword SHALL")
		return result
	}
	if len(shalls) > 1 {
		result.Errors = append(result.Errors, fmt.Sprintf("criterion contains %d occurrences of SHALL; split into separate criteria", len(shalls)))
		return result
	}
	prefix := text[:shalls[0]]
	response := text[shalls[0]+len("SHALL"):]

	clauses := scanClauses(prefix)
	if problem := checkClauses(prefix, response, clauses); problem != "" {
		result.Errors = append(result.Errors, problem)
		return result
	}

	result.Valid = true
	result.Form = formOf(clauses)
	for _, c := range clauses {
		result.Clauses = append(result.Clauses, c.kind)
	}
	if result.Form == FormUbiquitous {
		result.Warnings = append(result.Warnings, keywordsAfterShall(response)...)
	}
	if outOfOrder(clauses) {
		result.Warnings = append(result.Warnings, orderWarning)
	}
	if subject := subjectOf(prefix); isPronoun(subject) {
		result.Warnings = append(result.Warnings, fmt.Sprintf("subject %q is a pronoun; name the component that must respond", subject))
	}
	return result
}

// subjectOf returns the text between a criterion's last clause and SHALL:
// the last comma-separated part of the prefix, after its last THEN.
func subjectOf(prefix string) string {
	subject := prefix
	if comma := strings.LastIndex(subject, ","); comma >= 0 {
		subject = subject[comma+1:]
	}
	if then := keywordOffsets(subject, "THEN"); len(then) > 0 {
		subject = subject[then[len(then)-1]+len("THEN"):]
	}
	return strings.TrimSpace(subject)
}

func isPronoun(subject string) bool {
	for _, pronoun := range pronouns {
		if len(subject) == len(pronoun) && equalFoldASCII(subject, pronoun) {
			return true
		}
	}
	return false
}

// outOfOrder reports whether a clause comes after one that the EARS order
// places behind it.
func outOfOrder(clauses []clause) bool {
	highest := 0
	for _, c := range clauses {
		rank := clauseRank[c.kind]
		if rank < highest {
			return true
		}
		highest = max(highest, rank)
	}
	return false
}

// scanClauses finds the condition keywords that open a clause: at the start
// of the prefix or after a comma in any letter case, or written in capitals
// at any position. Any other occurrence is text of the clause around it.
func scanClauses(prefix string) []clause {
	var clauses []clause
	for _, candidate := range clauseKeywords {
		for _, start := range keywordOffsets(prefix, candidate.keyword) {
			end := start + len(candidate.keyword)
			before := strings.TrimRight(prefix[:start], " \t")
			opensClause := before == "" || strings.HasSuffix(before, ",")
			if opensClause || prefix[start:end] == candidate.keyword {
				clauses = append(clauses, clause{keyword: candidate.keyword, kind: candidate.kind, start: start, end: end})
			}
		}
	}
	sort.Slice(clauses, func(i, j int) bool { return clauses[i].start < clauses[j].start })
	return clauses
}

// checkClauses returns the first structural error in the order the
// classifier has always reported them, or "" when there is none: an
// unwanted-behavior clause without THEN, an empty response, an empty clause.
func checkClauses(prefix, response string, clauses []clause) string {
	for _, c := range clauses {
		if c.kind != ClauseUnwanted {
			continue
		}
		if len(keywordOffsets(prefix[c.end:], "THEN")) == 0 {
			return "IF keyword requires matching THEN before SHALL"
		}
		break
	}

	if strings.TrimSpace(response) == "" {
		return "empty response slot after SHALL"
	}

	for i, c := range clauses {
		limit := len(prefix)
		if i+1 < len(clauses) {
			limit = clauses[i+1].start
		}
		text := prefix[c.end:limit]
		if c.kind == ClauseUnwanted {
			if then := keywordOffsets(text, "THEN"); len(then) > 0 {
				text = text[:then[0]]
			}
		} else if comma := strings.Index(text, ","); comma >= 0 {
			text = text[:comma]
		}
		if strings.Trim(text, " \t,") == "" {
			return emptyClauseError[c.keyword]
		}
	}
	return ""
}

// formOf names the form given by the kinds of a criterion's clauses: none is
// ubiquitous, one kind gives that kind's form, two or more kinds are complex.
func formOf(clauses []clause) string {
	kinds := map[string]bool{}
	for _, c := range clauses {
		kinds[c.kind] = true
	}
	switch len(kinds) {
	case 0:
		return FormUbiquitous
	case 1:
		return formOfKind[clauses[0].kind]
	default:
		return FormComplex
	}
}

// keywordsAfterShall warns about condition keywords in the response of a
// ubiquitous criterion, which may be an inverted conditional form.
func keywordsAfterShall(response string) []string {
	var warnings []string
	for _, kw := range []string{"WHEN", "WHILE", "DURING", "WHERE", "IF"} {
		if len(keywordOffsets(response, kw)) > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"keyword %s appears after SHALL; the criterion may be an inverted %s form",
				kw, formForKeyword(kw),
			))
		}
	}
	return warnings
}

func formForKeyword(kw string) string {
	switch kw {
	case "WHEN":
		return "event-driven"
	case "WHILE", "DURING":
		return "state-driven"
	case "WHERE":
		return "optional"
	case "IF":
		return "unwanted"
	default:
		return "unknown"
	}
}

// keywordOffsets returns the offsets of keyword in text as a whole word,
// matching ASCII letters in any case. Keywords are ASCII capitals, so the
// offsets index text itself.
func keywordOffsets(text, keyword string) []int {
	var offsets []int
	for i := 0; i+len(keyword) <= len(text); i++ {
		if !equalFoldASCII(text[i:i+len(keyword)], keyword) {
			continue
		}
		end := i + len(keyword)
		if (i > 0 && isLetter(text[i-1])) || (end < len(text) && isLetter(text[end])) {
			continue
		}
		offsets = append(offsets, i)
		i = end - 1
	}
	return offsets
}

// equalFoldASCII reports whether s spells keyword, a word in ASCII capitals,
// in any letter case.
func equalFoldASCII(s, keyword string) bool {
	for i := 0; i < len(keyword); i++ {
		c := s[i]
		if 'a' <= c && c <= 'z' {
			c -= 'a' - 'A'
		}
		if c != keyword[i] {
			return false
		}
	}
	return true
}

func isLetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}
