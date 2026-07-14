package spec

import (
	"fmt"
	"strings"
)

// CheckTaskLayout validates only the indentation structure of a tasks
// document against the execution parser's offset rules: metadata lines at
// their owner task's legal offsets and structured proof lines relative to
// their Verification: line. It shares the parser's patterns and offset
// helpers so the two verdicts cannot drift. It deliberately ignores
// completeness — missing metadata, empty proof blocks, coverage — so
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
			if match := commandStepPattern.FindStringSubmatch(line); match != nil {
				if len(match[1]) != stepIndent {
					return fmt.Errorf(
						"line %d: invalid proof step indentation for task %q: expected %d spaces",
						index+1, currentTask.id, stepIndent,
					)
				}
				continue
			}
			attrMatch := expectExitPattern.FindStringSubmatch(line)
			if attrMatch == nil {
				attrMatch = expectOutputPattern.FindStringSubmatch(line)
			}
			if attrMatch == nil {
				attrMatch = coversPattern.FindStringSubmatch(line)
			}
			if attrMatch != nil {
				if len(attrMatch[1]) != attrIndent {
					return fmt.Errorf(
						"line %d: invalid proof attribute indentation for task %q: expected %d spaces",
						index+1, currentTask.id, attrIndent,
					)
				}
				continue
			}
			if strings.TrimSpace(line) == "" {
				continue
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
	}

	return nil
}
