package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/andrearaponi/walden/internal/trace"
)

// deliveryProofMark marks a proof step of a leaf that declares a delivery
// proof, as the evidence reports mark such a task.
const deliveryProofMark = " (delivery proof)"

func proofStep(proof trace.Proof) string {
	return fmt.Sprintf("%s step %d", proof.TaskID, proof.Step)
}

// namedBy lists the leaves that name a statement, or says that none does.
func namedBy(leaves []string) string {
	if len(leaves) == 0 {
		return "no leaf"
	}
	return strings.Join(leaves, ", ")
}

// holeText says what a hole is, after the statement it belongs to.
func holeText(hole trace.Hole) string {
	switch hole.Kind {
	case trace.HoleUnnamed:
		if len(hole.NamedBy) > 0 {
			// The parser read the statement on a leaf, and the validator
			// does not: an identifier written without backticks.
			return fmt.Sprintf("named by %s, which walden validate does not count", namedBy(hole.NamedBy))
		}
		return "no leaf names it"
	case trace.HoleUncovered:
		return fmt.Sprintf("named by %s, covered by no proof step", namedBy(hole.NamedBy))
	default:
		return fmt.Sprintf("%s step %d is %s", hole.TaskID, hole.Step, hole.State)
	}
}

// printTrace renders the trace section of a text result: one entry for each
// statement with its proof steps, then the holes.
func printTrace(w io.Writer, report *trace.Report) {
	if report == nil {
		return
	}
	_, _ = fmt.Fprintln(w, "Trace:")
	for _, statement := range report.Statements {
		_, _ = fmt.Fprintf(w, "- %s: named by %s\n", statement.ID, namedBy(statement.NamedBy))
		for _, proof := range statement.Proofs {
			mark := ""
			if proof.Delivery {
				mark = deliveryProofMark
			}
			_, _ = fmt.Fprintf(w, "  - %s, %s%s: %s\n", proofStep(proof), proof.State, mark, trace.Shown(proof.Command))
		}
	}
	if len(report.Holes) == 0 {
		_, _ = fmt.Fprintln(w, "Holes: none")
		return
	}
	_, _ = fmt.Fprintln(w, "Holes:")
	for _, hole := range report.Holes {
		_, _ = fmt.Fprintf(w, "- %s: %s\n", hole.Statement, holeText(hole))
	}
}

// PrintTraceMarkdown renders a trace as a Markdown table with one row for
// each pair of statement and proof step, then the holes, and nothing else:
// the output is made to be pasted into a pull request.
//
// The second and later rows of a statement leave its first two cells empty,
// and a statement that no step covers has one row whose last three cells are
// empty.
func PrintTraceMarkdown(w io.Writer, report trace.Report) {
	_, _ = fmt.Fprintln(w, "| Statement | Named by | Proof step | Evidence | Command |")
	_, _ = fmt.Fprintln(w, "| --- | --- | --- | --- | --- |")
	for _, statement := range report.Statements {
		first := fmt.Sprintf("| `%s` | %s |", statement.ID, namedBy(statement.NamedBy))
		if len(statement.Proofs) == 0 {
			_, _ = fmt.Fprintf(w, "%s | | |\n", first)
			continue
		}
		for index, proof := range statement.Proofs {
			lead := first
			if index > 0 {
				lead = "| | |"
			}
			step := proofStep(proof)
			if proof.Delivery {
				step += deliveryProofMark
			}
			_, _ = fmt.Fprintf(w, "%s %s | %s | %s |\n", lead, step, proof.State, markdownCode(trace.Shown(proof.Command)))
		}
	}
	_, _ = fmt.Fprintln(w)
	if len(report.Holes) == 0 {
		_, _ = fmt.Fprintln(w, "Holes: none")
		return
	}
	_, _ = fmt.Fprintf(w, "Holes: %d (%s)\n\n", len(report.Holes), report.HoleBreakdown())
	for _, hole := range report.Holes {
		_, _ = fmt.Fprintf(w, "- `%s`: %s\n", hole.Statement, holeText(hole))
	}
}

// markdownCode writes text as a code span of a table cell. Every pipe is
// escaped: a GitHub table reads `\|` as a pipe inside the span and not as
// the end of the cell. A text that holds a backtick is delimited by one
// backtick more than its longest run of backticks, with a space inside each
// delimiter, which a renderer strips.
func markdownCode(text string) string {
	longest, run := 0, 0
	for _, r := range text {
		if r != '`' {
			run = 0
			continue
		}
		run++
		if run > longest {
			longest = run
		}
	}
	text = strings.ReplaceAll(text, "|", `\|`)
	if longest == 0 {
		return "`" + text + "`"
	}
	fence := strings.Repeat("`", longest+1)
	return fence + " " + text + " " + fence
}
