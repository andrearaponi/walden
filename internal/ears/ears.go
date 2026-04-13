package ears

import (
	"fmt"
	"regexp"
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

// ParsedCriterion is the result of parsing one acceptance criterion.
type ParsedCriterion struct {
	ID       string
	Raw      string
	Form     string
	Valid    bool
	Errors   []string
	Warnings []string
}

var acLinePattern = regexp.MustCompile("(?m)^\\d+\\.\\s+`(R\\d+\\.AC\\d+)`\\s+(.*)")

// ParseAllCriteria extracts and classifies all acceptance criteria from a
// requirements.md body. It matches lines of the form:
//
//	1. `R1.AC1` WHEN [trigger], the system SHALL [response]
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

	upper := strings.ToUpper(text)

	if !containsKeyword(upper, "SHALL") {
		result.Errors = append(result.Errors, "missing required keyword SHALL")
		return result
	}

	if n := countKeyword(upper, "SHALL"); n > 1 {
		result.Errors = append(result.Errors, fmt.Sprintf("criterion contains %d occurrences of SHALL; split into separate criteria", n))
		return result
	}

	shallPos := keywordPosition(upper, "SHALL")

	// Classification by keyword presence before SHALL.
	hasWhen := hasKeywordBefore(upper, "WHEN", shallPos)
	hasWhile := hasKeywordBefore(upper, "WHILE", shallPos) || hasKeywordBefore(upper, "DURING", shallPos)
	hasWhere := hasKeywordBefore(upper, "WHERE", shallPos)
	hasIfThen := hasIfThenBefore(upper, shallPos)
	hasIf := hasKeywordBefore(upper, "IF", shallPos)

	switch {
	case hasWhile && hasWhen:
		result.Form = FormComplex
		result.Valid = true
	case hasWhen && !hasWhile && !hasWhere && !hasIf:
		result.Form = FormEventDriven
		result.Valid = true
	case hasWhile && !hasWhen && !hasWhere && !hasIf:
		result.Form = FormStateDriven
		result.Valid = true
	case hasWhere && !hasWhen && !hasWhile && !hasIf:
		result.Form = FormOptional
		result.Valid = true
	case hasIf:
		if !hasIfThen {
			result.Errors = append(result.Errors, "IF keyword requires matching THEN before SHALL")
			return result
		}
		result.Form = FormUnwanted
		result.Valid = true
	case !hasWhen && !hasWhile && !hasWhere && !hasIf:
		result.Form = FormUbiquitous
		result.Valid = true
	default:
		result.Errors = append(result.Errors, "ambiguous keyword combination; does not match a supported EARS form")
	}

	if result.Valid {
		if err := validateSlots(upper, result.Form, shallPos); err != "" {
			result.Valid = false
			result.Form = ""
			result.Errors = append(result.Errors, err)
		}
	}

	// Post-SHALL keyword warning: only for ubiquitous-classified criteria.
	if result.Valid && result.Form == FormUbiquitous {
		postShall := upper[shallPos+len("SHALL"):]
		for _, kw := range []string{"WHEN", "WHILE", "DURING", "WHERE", "IF"} {
			if containsKeyword(postShall, kw) {
				result.Warnings = append(result.Warnings, fmt.Sprintf(
					"keyword %s appears after SHALL; the criterion may be an inverted %s form",
					kw, formForKeyword(kw),
				))
			}
		}
	}

	return result
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

func validateSlots(upper, form string, shallPos int) string {
	// Response slot: text after SHALL must be non-empty for all forms.
	response := strings.TrimSpace(upper[shallPos+len("SHALL"):])
	if response == "" {
		return "empty response slot after SHALL"
	}

	prefix := upper[:shallPos]

	switch form {
	case FormEventDriven:
		whenPos := keywordPosition(prefix, "WHEN")
		slot := extractSlotAfterKeyword(prefix, "WHEN", whenPos)
		if slot == "" {
			return "empty trigger slot after WHEN"
		}
	case FormStateDriven:
		whilePos := keywordPosition(prefix, "WHILE")
		if whilePos < 0 {
			whilePos = keywordPosition(prefix, "DURING")
			if whilePos >= 0 {
				slot := extractSlotAfterKeyword(prefix, "DURING", whilePos)
				if slot == "" {
					return "empty precondition slot after DURING"
				}
			}
		} else {
			slot := extractSlotAfterKeyword(prefix, "WHILE", whilePos)
			if slot == "" {
				return "empty precondition slot after WHILE"
			}
		}
	case FormOptional:
		wherePos := keywordPosition(prefix, "WHERE")
		slot := extractSlotAfterKeyword(prefix, "WHERE", wherePos)
		if slot == "" {
			return "empty feature slot after WHERE"
		}
	case FormUnwanted:
		ifPos := keywordPosition(prefix, "IF")
		thenPos := keywordPosition(prefix[ifPos:], "THEN")
		if thenPos >= 0 {
			slot := strings.TrimSpace(prefix[ifPos+len("IF") : ifPos+thenPos])
			slot = strings.TrimRight(slot, ", ")
			if slot == "" {
				return "empty trigger slot between IF and THEN"
			}
		}
	case FormComplex:
		whenPos := keywordPosition(prefix, "WHEN")
		whilePos := keywordPosition(prefix, "WHILE")
		if whilePos < 0 {
			whilePos = keywordPosition(prefix, "DURING")
			if whilePos >= 0 && whenPos > whilePos+len("DURING") {
				preslot := strings.TrimSpace(prefix[whilePos+len("DURING") : whenPos])
				preslot = strings.TrimRight(preslot, ", ")
				if preslot == "" {
					return "empty precondition slot after DURING"
				}
			}
		} else if whenPos > whilePos+len("WHILE") {
			preslot := strings.TrimSpace(prefix[whilePos+len("WHILE") : whenPos])
			preslot = strings.TrimRight(preslot, ", ")
			if preslot == "" {
				return "empty precondition slot after WHILE"
			}
		}
		if whenPos >= 0 {
			trigslot := extractSlotAfterKeyword(prefix, "WHEN", whenPos)
			if trigslot == "" {
				return "empty trigger slot after WHEN"
			}
		}
	}

	return ""
}

func extractSlotAfterKeyword(text, keyword string, kwPos int) string {
	after := text[kwPos+len(keyword):]
	// The slot is the text between the keyword and the next comma or end of prefix.
	if commaIdx := strings.Index(after, ","); commaIdx >= 0 {
		after = after[:commaIdx]
	}
	return strings.TrimSpace(after)
}

func countKeyword(upper, keyword string) int {
	count := 0
	remaining := upper
	for {
		pos := keywordPosition(remaining, keyword)
		if pos < 0 {
			return count
		}
		count++
		remaining = remaining[pos+len(keyword):]
	}
}

func containsKeyword(upper, keyword string) bool {
	return keywordPosition(upper, keyword) >= 0
}

func keywordPosition(upper, keyword string) int {
	pos := 0
	remaining := upper
	for {
		idx := strings.Index(remaining, keyword)
		if idx < 0 {
			return -1
		}
		absPos := pos + idx
		before := absPos == 0 || !isLetter(remaining[idx-1])
		after := idx+len(keyword) >= len(remaining) || !isLetter(remaining[idx+len(keyword)])
		if before && after {
			return absPos
		}
		pos += idx + len(keyword)
		remaining = remaining[idx+len(keyword):]
	}
}

func hasKeywordBefore(upper, keyword string, shallPos int) bool {
	prefix := upper[:shallPos]
	return keywordPosition(prefix, keyword) >= 0
}

func hasIfThenBefore(upper string, shallPos int) bool {
	prefix := upper[:shallPos]
	ifPos := keywordPosition(prefix, "IF")
	if ifPos < 0 {
		return false
	}
	thenPos := keywordPosition(prefix[ifPos:], "THEN")
	return thenPos >= 0
}

func isLetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}
