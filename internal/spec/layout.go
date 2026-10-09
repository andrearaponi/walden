package spec

import (
	"fmt"
	"strings"
)

// CheckTaskLayout validates the structure of a tasks document against the
// execution parser's rules: metadata lines at their owner task's legal
// offsets, and structured proof lines relative to their Verification: line,
// read with the parser's proof-line grammar, value parsers and block rules,
// so the two verdicts and their messages cannot drift. It deliberately
// ignores completeness — missing metadata, empty proof blocks, coverage — so
// incremental drafts stay legal.
func CheckTaskLayout(body string) error {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")

	type owner struct {
		id    string
		level int
	}
	var currentTask *owner
	verificationIndent := -1 // -1 means no open structured verification block

	for index, line := range lines {
		if verificationIndent >= 0 {
			stepIndent := verificationIndent + 2
			attrIndent := verificationIndent + 4
			proof := classifyProofLine(line)
			switch proof.kind {
			case proofLineBlank, proofLineComment:
				continue
			case proofLineStep:
				if proof.indent != stepIndent {
					return fmt.Errorf(
						"line %d: invalid proof step indentation for task %q: expected %d spaces",
						index+1, currentTask.id, stepIndent,
					)
				}
				if _, err := parseProofArgv(proof.value, index, currentTask.id); err != nil {
					return err
				}
				continue
			case proofLineAttribute:
				if proof.indent != attrIndent {
					return fmt.Errorf(
						"line %d: invalid proof attribute indentation for task %q: expected %d spaces",
						index+1, currentTask.id, attrIndent,
					)
				}
				if _, err := parseProofAttribute(proof, index, currentTask.id); err != nil {
					return err
				}
				continue
			}
			if !isStructuralLine(line) && proof.indent > verificationIndent {
				return unrecognizedProofLineError(index, currentTask.id, line)
			}
			verificationIndent = -1
		}

		if match := taskLinePattern.FindStringSubmatch(line); match != nil {
			id := match[4]
			currentTask = &owner{id: id, level: strings.Count(id, ".") + 1}
			continue
		}

		if match := metadataLinePattern.FindStringSubmatch(line); match != nil {
			if currentTask == nil {
				// Ownerless metadata is a structural defect for the full
				// parser, not an indentation concern.
				continue
			}
			indent := len(match[1])
			if !metadataOffsetLegal(currentTask.level, indent) {
				return fmt.Errorf(
					"line %d: invalid metadata indentation for task %q: expected %s",
					index+1, currentTask.id, metadataOffsetsLabel(currentTask.level),
				)
			}
			if match[2] == "Verification" && strings.TrimSpace(match[3]) == "" {
				verificationIndent = indent
			}
			continue
		}

		trimmed := strings.TrimSpace(line)
		if currentTask != nil &&
			(strings.HasPrefix(trimmed, "- Requirements:") ||
				strings.HasPrefix(trimmed, "- Design:") ||
				strings.HasPrefix(trimmed, "- Verification:")) {
			// Column-zero metadata: the indented pattern cannot match it.
			return fmt.Errorf(
				"line %d: invalid metadata indentation for task %q: expected %s",
				index+1, currentTask.id, metadataOffsetsLabel(currentTask.level),
			)
		}
		if isProofKeywordText(trimmed) {
			afterTask := ""
			if currentTask != nil {
				afterTask = currentTask.id
			}
			return outsideBlockProofLineError(index, afterTask, line)
		}
	}

	return nil
}
