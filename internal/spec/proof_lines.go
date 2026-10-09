package spec

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// A proof line is recognized by its keyword alone; its value is parsed
// afterwards. A malformed value therefore fails with the form its keyword
// accepts instead of making the line unrecognizable, which used to end the
// Verification block and drop every line after it.
var (
	proofStepPattern      = regexp.MustCompile(`^( +)- (command|argv):(.*)$`)
	proofAttributePattern = regexp.MustCompile(`^( +)(expect_exit|expect_output|covers|timeout):(.*)$`)
)

// The forms each keyword accepts, as error messages name them.
const (
	argvForm         = `a non-empty JSON array of non-empty strings such as ["go", "test", "./..."]`
	expectExitForm   = "a non-negative integer exit code"
	expectOutputForm = "non-empty text, optionally in double quotes"
	coversForm       = `a JSON array of criterion IDs such as ["R1.AC1"], or [] for none`
	timeoutForm      = "a positive Go duration such as 90s or 30m"
)

type proofLineKind int

const (
	proofLineOther proofLineKind = iota
	proofLineBlank
	proofLineComment
	proofLineStep
	proofLineAttribute
)

// proofLine is one line of tasks.md classified for proof-block reading.
type proofLine struct {
	kind    proofLineKind
	indent  int
	keyword string
	value   string
}

func classifyProofLine(line string) proofLine {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return proofLine{kind: proofLineBlank}
	}
	// A single-line HTML comment, such as an inline assumption, is not proof
	// content: it neither counts nor ends the block.
	if strings.HasPrefix(trimmed, "<!--") && strings.HasSuffix(trimmed, "-->") {
		return proofLine{kind: proofLineComment}
	}
	if match := proofStepPattern.FindStringSubmatch(line); match != nil {
		return proofLine{kind: proofLineStep, indent: len(match[1]), keyword: match[2], value: strings.TrimSpace(match[3])}
	}
	if match := proofAttributePattern.FindStringSubmatch(line); match != nil {
		return proofLine{kind: proofLineAttribute, indent: len(match[1]), keyword: match[2], value: strings.TrimSpace(match[3])}
	}
	return proofLine{kind: proofLineOther, indent: len(line) - len(strings.TrimLeft(line, " "))}
}

// isStructuralLine reports a task or metadata line, misindented or not: it
// ends a Verification block and keeps the errors the task parser reports
// for its indentation.
func isStructuralLine(line string) bool {
	if taskLinePattern.MatchString(line) || metadataLinePattern.MatchString(line) {
		return true
	}
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "- [") ||
		strings.HasPrefix(trimmed, "- Requirements:") ||
		strings.HasPrefix(trimmed, "- Design:") ||
		strings.HasPrefix(trimmed, "- Verification:")
}

// proofKeywordPrefixes start every proof step and attribute line once its
// indentation is trimmed.
var proofKeywordPrefixes = []string{"- command:", "- argv:", "expect_exit:", "expect_output:", "covers:", "timeout:"}

// isProofKeywordText reports trimmed text that only a proof block may hold.
func isProofKeywordText(trimmed string) bool {
	for _, prefix := range proofKeywordPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

// outsideBlockProofLineError names a proof line that no Verification block
// owns; afterTask is the task it follows, if any.
func outsideBlockProofLineError(index int, afterTask, line string) error {
	if afterTask == "" {
		return fmt.Errorf("line %d: proof line outside a Verification block: %q — expected under its task's \"- Verification:\" line", index+1, strings.TrimSpace(line))
	}
	return fmt.Errorf("line %d: proof line outside a Verification block, after task %q: %q — expected under its task's \"- Verification:\" line", index+1, afterTask, strings.TrimSpace(line))
}

func unrecognizedProofLineError(index int, taskID, line string) error {
	return fmt.Errorf(
		"line %d: unrecognized line in the Verification block of task %q: %q — expected a \"- command:\" step, an expect_exit, expect_output, covers or timeout attribute, a blank line or a single-line HTML comment",
		index+1, taskID, strings.TrimSpace(line),
	)
}

func proofValueError(index int, label, taskID, form, value string) error {
	return fmt.Errorf("line %d: invalid %s for task %q: expected %s; got %q", index+1, label, taskID, form, value)
}

// parseProofArgv reads a step's argv. The decoding stays in
// parseVerificationArgv, which environment probes share.
func parseProofArgv(value string, index int, taskID string) ([]string, error) {
	argv, err := parseVerificationArgv(value, index)
	if err != nil {
		return nil, proofValueError(index, "argv verification step", taskID, argvForm, value)
	}
	return argv, nil
}

// parseProofAttribute reads an attribute value and returns how it applies to
// the step it belongs to.
func parseProofAttribute(proof proofLine, index int, taskID string) (func(*VerificationStep), error) {
	switch proof.keyword {
	case "expect_exit":
		if proof.value == "" || strings.TrimLeft(proof.value, "0123456789") != "" {
			return nil, proofValueError(index, "expect_exit", taskID, expectExitForm, proof.value)
		}
		exitCode, err := strconv.Atoi(proof.value)
		if err != nil {
			return nil, proofValueError(index, "expect_exit", taskID, expectExitForm, proof.value)
		}
		return func(step *VerificationStep) { step.ExpectExit = &exitCode }, nil
	case "expect_output":
		expected := unquote(proof.value)
		if expected == "" {
			return nil, proofValueError(index, "expect_output", taskID, expectOutputForm, proof.value)
		}
		return func(step *VerificationStep) { step.ExpectOutput = &expected }, nil
	case "covers":
		var covers []string
		if !strings.HasPrefix(proof.value, "[") || json.Unmarshal([]byte(proof.value), &covers) != nil {
			return nil, proofValueError(index, "covers", taskID, coversForm, proof.value)
		}
		if len(covers) == 0 {
			// An empty array asserts no criterion, exactly like an absent covers.
			covers = nil
		}
		return func(step *VerificationStep) { step.Covers = covers }, nil
	default: // timeout
		value := unquote(proof.value)
		duration, err := time.ParseDuration(value)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid timeout value for task %q: %v; expected %s", index+1, taskID, err, timeoutForm)
		}
		if duration <= 0 {
			return nil, fmt.Errorf("line %d: invalid timeout value for task %q: must be positive; expected %s", index+1, taskID, timeoutForm)
		}
		return func(step *VerificationStep) { step.Timeout = &value }, nil
	}
}

func unquote(value string) string {
	if len(value) >= 2 && strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		return value[1 : len(value)-1]
	}
	return value
}
