package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// StatementScheme names how a statement block is built and hashed. A record
// stores it beside its statement fingerprints, so a later scheme can be told
// from this one instead of making every statement read as changed.
const StatementScheme = "walden/statement-block/v1"

// A definition line is a numbered item or a bullet whose first token is a
// backticked criterion, NFR or constraint identifier — the same lines the
// consolidation report reads. The first group is the line's indentation.
var (
	criterionStatement = regexp.MustCompile("^(\\s*)\\d+\\.\\s+`(" + CriterionIDExpr + ")`")
	bulletStatement    = regexp.MustCompile("^(\\s*)-\\s+`(" + NFRIDExpr + "|" + ConstraintIDExpr + ")`")
)

// StatementBlocks returns, for each criterion, NFR and constraint a
// requirements body defines, the text that states it: the definition line
// after the identifier, then every directly following line indented deeper
// than the definition line, which takes in the acceptance check and any other
// continuation. Each line is trimmed and the lines are joined with one LF.
// The first line that is blank, opens a fence, or is not deeper ends the
// block. Lines inside fenced blocks are examples and define nothing. A
// repeated identifier keeps its first definition.
//
// The bytes are fixed on purpose: StatementFingerprint hashes them, and a
// change here would make every recorded statement read as changed.
func StatementBlocks(body string) map[string]string {
	normalized := strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(normalized, "\n")

	blocks := map[string]string{}
	fenced := false
	for index := 0; index < len(lines); index++ {
		line := lines[index]
		if opensFence(line) {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		indent, id, rest, defined := statementDefinition(line)
		if !defined {
			continue
		}

		parts := []string{strings.TrimSpace(rest)}
		next := index + 1
		for ; next < len(lines); next++ {
			following := lines[next]
			if strings.TrimSpace(following) == "" || opensFence(following) || indentation(following) <= indent {
				break
			}
			parts = append(parts, strings.TrimSpace(following))
		}
		if _, seen := blocks[id]; !seen {
			blocks[id] = strings.Join(parts, "\n")
		}
		// Resume at the line that ended the block: it may open a fence or
		// define the next statement.
		index = next - 1
	}
	return blocks
}

// StatementFingerprint is the fingerprint of one statement block: sha256 of
// its bytes, in the same rendering as a document fingerprint.
func StatementFingerprint(block string) string {
	sum := sha256.Sum256([]byte(block))
	return fingerprintPrefix + hex.EncodeToString(sum[:])
}

// StatementFingerprints fingerprints every statement a requirements body
// defines.
func StatementFingerprints(body string) map[string]string {
	blocks := StatementBlocks(body)
	fingerprints := make(map[string]string, len(blocks))
	for id, block := range blocks {
		fingerprints[id] = StatementFingerprint(block)
	}
	return fingerprints
}

func statementDefinition(line string) (indent int, id, rest string, defined bool) {
	for _, pattern := range []*regexp.Regexp{criterionStatement, bulletStatement} {
		if location := pattern.FindStringSubmatchIndex(line); location != nil {
			return location[3] - location[2], line[location[4]:location[5]], line[location[1]:], true
		}
	}
	return 0, "", "", false
}

func opensFence(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

func indentation(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}
