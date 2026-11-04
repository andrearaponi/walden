package spec

import (
	"path/filepath"
	"strings"
)

// decisionMarker is the skill protocol's open-checkpoint text. The gates
// treat it as a deterministic textual lint, never as an interpretation of
// the protocol itself.
const decisionMarker = "[decision:"

const (
	commentOpener     = "<!--"
	commentTerminator = "-->"
)

// UnresolvedKind names a kind of unresolved content.
type UnresolvedKind int

const (
	// OpenDecisionMarker is a decision marker outside quoted text.
	OpenDecisionMarker UnresolvedKind = iota + 1
	// UnterminatedComment is an HTML comment opener with no terminator after it.
	UnterminatedComment
)

// Unresolved is one finding of ScanUnresolved.
type Unresolved struct {
	Kind UnresolvedKind
	// Line is the body line: 1 is the first line after the frontmatter, as
	// proof errors count it.
	Line int
}

// Label is the noun phrase the gates print for the kind.
func (k UnresolvedKind) Label() string {
	switch k {
	case OpenDecisionMarker:
		return "unresolved " + decisionMarker + "] marker"
	case UnterminatedComment:
		return "unterminated HTML comment"
	default:
		return "unresolved content"
	}
}

// Remedy says how to clear a finding of the kind.
func (k UnresolvedKind) Remedy() string {
	switch k {
	case OpenDecisionMarker:
		return "resolve it, or quote it in a code span"
	case UnterminatedComment:
		return "close it with " + commentTerminator + ", or quote it in a code span"
	default:
		return "resolve it"
	}
}

// ScanUnresolved reports the decision markers and the comment openers with
// no terminator after them, in document order. It reads the body in order,
// so the construct that opens first governs the text until it closes:
// code spans, fenced code blocks and terminated comments are quoted text,
// and so are the proof steps and attributes of tasks.md, recognized by base
// name. A construct that never closes hides nothing. The result depends only
// on the two arguments: nothing is read from disk.
func ScanUnresolved(documentPath, body string) []Unresolved {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	exemptProofLines := filepath.Base(documentPath) == "tasks.md"

	var found []Unresolved
	for index := 0; index < len(lines); {
		if exemptProofLines && isProofStepOrAttribute(lines[index]) {
			index++
			continue
		}
		if closer, ok := fenceCloser(lines, index); ok {
			index = closer + 1
			continue
		}
		index = scanInline(lines, index, &found)
	}
	return found
}

func isProofStepOrAttribute(line string) bool {
	kind := classifyProofLine(line).kind
	return kind == proofLineStep || kind == proofLineAttribute
}

// fenceCloser returns the line that closes a fence opened at index: a run
// of three or more backticks or tildes after any indentation, closed by a
// line holding only a run of the same character at least as long. A
// backtick fence takes no backtick in its info string.
func fenceCloser(lines []string, index int) (int, bool) {
	opener := strings.TrimLeft(lines[index], " \t")
	if opener == "" || (opener[0] != '`' && opener[0] != '~') {
		return 0, false
	}
	char := opener[0]
	length := runLength(opener, 0, char)
	if length < 3 || (char == '`' && strings.IndexByte(opener[length:], '`') >= 0) {
		return 0, false
	}
	for closer := index + 1; closer < len(lines); closer++ {
		candidate := strings.TrimSpace(lines[closer])
		if len(candidate) >= length && runLength(candidate, 0, char) == len(candidate) {
			return closer, true
		}
	}
	return 0, false
}

type inlineToken int

const (
	tokenNone inlineToken = iota
	tokenMarker
	tokenOpener
	tokenBackticks
)

// scanInline walks the line at index with a cursor and returns the index of
// the next line to read, past the end of a comment that spans lines.
func scanInline(lines []string, index int, found *[]Unresolved) int {
	line, cursor := lines[index], 0
	for {
		position, token := nextToken(line, cursor)
		switch token {
		case tokenNone:
			return index + 1
		case tokenMarker:
			*found = append(*found, Unresolved{Kind: OpenDecisionMarker, Line: index + 1})
			cursor = position + len(decisionMarker)
		case tokenBackticks:
			length := runLength(line, position, '`')
			cursor = position + length
			if end, ok := closingBackticks(line, cursor, length); ok {
				cursor = end
			}
		case tokenOpener:
			closeLine, closeEnd, ok := commentEnd(lines, index, position)
			if !ok {
				*found = append(*found, Unresolved{Kind: UnterminatedComment, Line: index + 1})
				cursor = position + len(commentOpener)
				continue
			}
			index, line, cursor = closeLine, lines[closeLine], closeEnd
		}
	}
}

// nextToken finds the earliest of a decision marker, a comment opener and a
// backtick at or after the cursor. Their first characters differ, so two
// tokens never start at the same position.
func nextToken(line string, cursor int) (int, inlineToken) {
	rest := line[cursor:]
	position, token := -1, tokenNone
	for _, candidate := range [...]struct {
		offset int
		token  inlineToken
	}{
		{strings.Index(rest, decisionMarker), tokenMarker},
		{strings.Index(rest, commentOpener), tokenOpener},
		{strings.IndexByte(rest, '`'), tokenBackticks},
	} {
		if candidate.offset >= 0 && (position < 0 || candidate.offset < position) {
			position, token = candidate.offset, candidate.token
		}
	}
	if token == tokenNone {
		return 0, tokenNone
	}
	return cursor + position, token
}

// closingBackticks finds the next run of exactly length backticks on the
// line and returns the position after it.
func closingBackticks(line string, from, length int) (int, bool) {
	for from < len(line) {
		offset := strings.IndexByte(line[from:], '`')
		if offset < 0 {
			return 0, false
		}
		start := from + offset
		run := runLength(line, start, '`')
		if run == length {
			return start + run, true
		}
		from = start + run
	}
	return 0, false
}

// commentEnd finds the first terminator after the opener at position on the
// line at index, searching from the opener itself so that its own dashes
// can close it, and returns the line and the position after the terminator.
func commentEnd(lines []string, index, position int) (int, int, bool) {
	if offset := strings.Index(lines[index][position:], commentTerminator); offset >= 0 {
		return index, position + offset + len(commentTerminator), true
	}
	for next := index + 1; next < len(lines); next++ {
		if offset := strings.Index(lines[next], commentTerminator); offset >= 0 {
			return next, offset + len(commentTerminator), true
		}
	}
	return 0, 0, false
}

func runLength(text string, start int, char byte) int {
	end := start
	for end < len(text) && text[end] == char {
		end++
	}
	return end - start
}
