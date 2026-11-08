package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Testability companion contract (testability-skill). Every checker is a pure
// function over content, and each subtest runs it on fixtures it must reject
// before it reads the real files, so a broken matcher fails loudly instead of
// passing vacuously. These tests hold the instruction text in place; they do
// not establish that an agent follows it, which the behavioral evaluation
// observes separately.

const (
	testabilitySkillPath = "skill/walden-testability/SKILL.md"
	testabilitySeamsPath = "skill/walden-testability/references/seams.md"
)

func readTestabilityFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(authoringRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// testabilityFold lowercases text and collapses its whitespace, so a rule
// wrapped across lines still matches.
func testabilityFold(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

// requireTestabilityFragments names every fragment missing from text.
func requireTestabilityFragments(text string, fragments ...string) error {
	folded := testabilityFold(text)
	var missing []string
	for _, fragment := range fragments {
		if !strings.Contains(folded, testabilityFold(fragment)) {
			missing = append(missing, fragment)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing %q", missing)
	}
	return nil
}

// testabilitySection returns the body of the level-two section with the given
// heading, up to the next level-two heading outside a fenced block.
func testabilitySection(content, heading string) (string, error) {
	lines := strings.Split(content, "\n")
	start, end := -1, len(lines)
	fenced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if fenced || !strings.HasPrefix(line, "## ") {
			continue
		}
		if start >= 0 {
			end = i
			break
		}
		if strings.TrimSpace(strings.TrimPrefix(line, "## ")) == heading {
			start = i + 1
		}
	}
	if start < 0 {
		return "", fmt.Errorf("no section %q", heading)
	}
	return strings.Join(lines[start:end], "\n"), nil
}

// testabilityRule is one stated rule: the phrases that state it and the
// level-two section they must sit under. An ordered rule also requires its
// phrases in the given order, as a ladder is.
type testabilityRule struct {
	name      string
	section   string
	fragments []string
	ordered   bool
}

func testabilityStated(name, section string, fragments ...string) testabilityRule {
	return testabilityRule{name: name, section: section, fragments: fragments}
}

func testabilityLadder(name, section string, fragments ...string) testabilityRule {
	return testabilityRule{name: name, section: section, fragments: fragments, ordered: true}
}

func checkTestabilityRules(content string, rules []testabilityRule) error {
	for _, rule := range rules {
		section, err := testabilitySection(content, rule.section)
		if err != nil {
			return fmt.Errorf("%s: %w", rule.name, err)
		}
		if err := requireTestabilityFragments(section, rule.fragments...); err != nil {
			return fmt.Errorf("%s (section %q): %w", rule.name, rule.section, err)
		}
		if !rule.ordered {
			continue
		}
		folded, last := testabilityFold(section), -1
		for _, fragment := range rule.fragments {
			at := strings.Index(folded, testabilityFold(fragment))
			if at < last {
				return fmt.Errorf("%s (section %q): %q is out of order", rule.name, rule.section, fragment)
			}
			last = at
		}
	}
	return nil
}

// testabilityRuleFixture writes a document that states every rule except the
// one at index omit; omit -1 keeps them all.
func testabilityRuleFixture(rules []testabilityRule, omit int) string {
	var order []string
	bodies := map[string][]string{}
	for i, rule := range rules {
		if _, seen := bodies[rule.section]; !seen {
			order = append(order, rule.section)
			bodies[rule.section] = []string{}
		}
		if i != omit {
			bodies[rule.section] = append(bodies[rule.section], strings.Join(rule.fragments, ". ")+".")
		}
	}
	var b strings.Builder
	b.WriteString("# Fixture\n")
	for _, section := range order {
		fmt.Fprintf(&b, "\n## %s\n\n%s\n", section, strings.Join(bodies[section], "\n"))
	}
	return b.String()
}

// assertTestabilityRules runs the rule checker on its fixtures, then on the
// real content: the complete fixture passes, every fixture missing one rule
// fails, and so does the complete fixture with its sections renamed.
func assertTestabilityRules(t *testing.T, rules []testabilityRule, real string) {
	t.Helper()
	complete := testabilityRuleFixture(rules, -1)
	if err := checkTestabilityRules(complete, rules); err != nil {
		t.Fatalf("the complete fixture was rejected: %v", err)
	}
	for i, rule := range rules {
		if checkTestabilityRules(testabilityRuleFixture(rules, i), rules) == nil {
			t.Errorf("a fixture without the rule %q was accepted", rule.name)
		}
	}
	if checkTestabilityRules(strings.ReplaceAll(complete, "\n## ", "\n### "), rules) == nil {
		t.Error("a fixture whose rules sit outside their sections was accepted")
	}
	for i, rule := range rules {
		if !rule.ordered {
			continue
		}
		reversed := append([]testabilityRule(nil), rules...)
		reversed[i].fragments = nil
		for j := len(rule.fragments) - 1; j >= 0; j-- {
			reversed[i].fragments = append(reversed[i].fragments, rule.fragments[j])
		}
		if checkTestabilityRules(testabilityRuleFixture(reversed, -1), rules) == nil {
			t.Errorf("a fixture with the ladder %q reversed was accepted", rule.name)
		}
	}
	if err := checkTestabilityRules(real, rules); err != nil {
		t.Error(err)
	}
}

// The triggers R7.AC1 names, as the description must carry them.
var testabilityTriggers = []string{"testability", "verifiability", "Walden design", "controllability", "observability", "proof cost", "design for testability"}

// testabilityDescription returns the unquoted description of a SKILL.md.
func testabilityDescription(content string) (string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return "", fmt.Errorf("no frontmatter block")
	}
	rest := content[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", fmt.Errorf("unterminated frontmatter block")
	}
	for _, line := range strings.Split(rest[:end], "\n") {
		if value, found := strings.CutPrefix(line, "description:"); found {
			return unquoteFrontmatter(value), nil
		}
	}
	return "", fmt.Errorf("frontmatter has no description")
}

// checkTestabilityDescription applies the Agent Skills frontmatter rules and
// requires every trigger of R7.AC1 in the description.
func checkTestabilityDescription(content string) error {
	if err := checkSkillFrontmatter("walden-testability", content); err != nil {
		return err
	}
	description, err := testabilityDescription(content)
	if err != nil {
		return err
	}
	if err := requireTestabilityFragments(description, testabilityTriggers...); err != nil {
		return fmt.Errorf("description: %w", err)
	}
	return nil
}

var testabilityBoundaryRules = []testabilityRule{
	testabilityStated("stop without the CLI", "Prerequisites",
		"Run `walden version` before anything else", "If no walden executable answers, stop", "produce no review",
		"install instructions", "https://github.com/andrearaponi/walden", "never installs the CLI"),
	testabilityStated("state only from the CLI", "Product Boundary",
		"Workflow state comes only from `walden status <feature> --json`", "never inferred from frontmatter text"),
	testabilityStated("single write path", "Product Boundary",
		"The only write path is the body of a draft design", "only after the user accepts",
		"Frontmatter, requirements, tasks, the constitution, lessons and evidence are never written"),
	testabilityStated("hand-off for a design in review or approved", "Product Boundary",
		"A design in review or approved is never edited", "proposed revision", "reconcile and re-review path"),
	testabilityStated("refusal to author, open for review or approve", "Product Boundary",
		"author, open for review or approve", "declined and routed to the walden skill", "runs no `walden review`"),
	testabilityStated("decline on unapproved or stale requirements", "Entry Check",
		"`walden status <feature> --json`", "Unless `requirements.md` reads `approved` with `fresh: true`",
		"decline the review", "route the user to the walden skill's requirements phase"),
}

// TestTestabilitySkillBoundary holds the description, the boundary and the
// entry check of the companion (R1.AC2, R1.AC3, R5.AC4, R5.AC5, R7.AC1, NFR1,
// C4).
func TestTestabilitySkillBoundary(t *testing.T) {
	t.Run("description", func(t *testing.T) {
		const compliant = "Reviews a testability or verifiability review of a Walden design: controllability, observability, proof cost and design for testability."
		wrap := func(description string) string {
			return "---\nname: walden-testability\ndescription: \"" + description + "\"\n---\n\n# Fixture\n"
		}
		if err := checkTestabilityDescription(wrap(compliant)); err != nil {
			t.Fatalf("a compliant description was rejected: %v", err)
		}
		for _, trigger := range testabilityTriggers {
			fixture := wrap(strings.ReplaceAll(compliant, trigger, "something else"))
			if checkTestabilityDescription(fixture) == nil {
				t.Errorf("a description without %q was accepted", trigger)
			}
		}
		if checkTestabilityDescription(wrap(compliant+strings.Repeat(" padding", 128))) == nil {
			t.Error("a description over 1024 characters was accepted")
		}
		if checkTestabilityDescription(strings.Replace(wrap(compliant), "---\n\n#", "version: 1\n---\n\n#", 1)) == nil {
			t.Error("a frontmatter key outside the Agent Skills set was accepted")
		}
		if checkTestabilityDescription(readTestabilityFile(t, "skill/walden-history/SKILL.md")) == nil {
			t.Error("the walden-history guide was accepted as the testability companion")
		}
		if err := checkTestabilityDescription(readTestabilityFile(t, testabilitySkillPath)); err != nil {
			t.Errorf("%s: %v", testabilitySkillPath, err)
		}
	})

	t.Run("boundary and entry check", func(t *testing.T) {
		if checkTestabilityRules(readTestabilityFile(t, "skill/walden-history/SKILL.md"), testabilityBoundaryRules) == nil {
			t.Error("the walden-history guide was accepted as stating the companion's boundary")
		}
		assertTestabilityRules(t, testabilityBoundaryRules, readTestabilityFile(t, testabilitySkillPath))
	})
}

var testabilityModelRules = []testabilityRule{
	testabilityStated("stimulus table with the six EARS forms", "The Five Questions",
		"| Form | What the proof must produce |", "| Ubiquitous | no trigger: the input domain", "| Event-driven |", "| State-driven |",
		"| Optional feature |", "| Unwanted behavior |", "| Complex | the state, then the event inside it |"),
	testabilityLadder("observation ladder", "The Five Questions",
		"a returned value or output", "an emitted event or message", "state read back through a public interface",
		"a persisted artifact read through its public path", "a rendered element"),
	testabilityStated("private fields excluded", "The Five Questions",
		"A private field never counts as an observation point", "A log counts only when the log is the response the criterion names"),
	testabilityStated("five oracle kinds", "The Five Questions",
		"one of five oracle kinds: a value stated by the criterion, a worked example, an invariant, a reference implementation, or a recorded fixture",
		"The oracle must come from outside the implementation under test"),
	testabilityStated("recorded output only for characterization", "The Five Questions",
		"A fixture recorded from the current implementation is an oracle only for characterization proofs", "never for new behavior"),
	testabilityStated("six proof kinds", "The Five Questions",
		"Name exactly one", "example, property, differential, contract, end-to-end, manual",
		"An invariant in ubiquitous form defaults to a property proof"),
	testabilityStated("reason for end-to-end and manual proofs", "The Five Questions",
		"An end-to-end or manual proof always carries its reason", "the response that no cheaper kind can see"),
	testabilityLadder("three loop classes", "The Five Questions",
		"**inner loop**: deterministic", "**checkpoint**: needs minutes or local services", "**manual**: needs a person or a live third party"),
	testabilityStated("container proofs are not inner loop", "The Five Questions",
		"A proof that starts a container is never inner loop", "no Walden proof can cover the criterion"),
	testabilityStated("three gap kinds with the eight sources", "Gaps And The Smallest Change",
		"**Controllability gap**", "**Observability gap**", "**Oracle gap**",
		"the clock, randomness, the environment, the network, the filesystem, concurrency, global state or an external service"),
	testabilityStated("a controlled source has no gap", "Gaps And The Smallest Change",
		"A source counts as controlled when the design injects it or the repository's test tooling already drives it"),
	testabilityStated("independent oracle", "Gaps And The Smallest Change",
		"Propose an independent oracle", "a question for the requirements phase"),
	testabilityLadder("control order", "Gaps And The Smallest Change",
		"**Platform control.**", "**One seam.**", "**Restructuring.**"),
	testabilityStated("no gap, no change", "Gaps And The Smallest Change",
		"No gap, no change", "a criterion without a gap gets no seam, no interface and no injection"),
	testabilityStated("anchored, idiomatic proposals", "Gaps And The Smallest Change",
		"Every proposal cites the design section or the `file#symbol` it changes",
		"When the repository's language is identified, show the change in that language's idiomatic form"),
	testabilityStated("unverifiable criteria routed to requirements", "Gaps And The Smallest Change",
		"**Unverifiable criteria are not gaps.**", "states no observable distinction between success and failure",
		"report the criterion as unverifiable", "propose no design change", "route it to the walden skill's requirements phase"),
}

// TestTestabilitySkillModel holds the five questions and the gap rules of the
// companion (R2, R3, R4).
func TestTestabilitySkillModel(t *testing.T) {
	if checkTestabilityRules(readTestabilityFile(t, "skill/walden-history/SKILL.md"), testabilityModelRules) == nil {
		t.Error("the walden-history guide was accepted as stating the testability model")
	}
	assertTestabilityRules(t, testabilityModelRules, readTestabilityFile(t, testabilitySkillPath))
}

// The fixed column set of the testability matrix (NFR3).
var testabilityMatrixColumns = []string{"Criterion", "Stimulus", "Observation", "Oracle", "Proof", "Loop", "Gap"}

// checkTestabilityMatrix requires the first table of The Testability Matrix
// section to have exactly the fixed columns, in order.
func checkTestabilityMatrix(content string) error {
	section, err := testabilitySection(content, "The Testability Matrix")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		var cells []string
		for _, cell := range strings.Split(strings.Trim(line, "|"), "|") {
			cells = append(cells, strings.TrimSpace(cell))
		}
		if strings.Join(cells, "|") != strings.Join(testabilityMatrixColumns, "|") {
			return fmt.Errorf("matrix columns %q, want exactly %q", cells, testabilityMatrixColumns)
		}
		return nil
	}
	return fmt.Errorf("The Testability Matrix holds no table")
}

var testabilityCeremonyRules = []testabilityRule{
	testabilityStated("footprint", "Recipe: Review",
		"the files and symbols the design names, plus the functions they call directly", "Open nothing else",
		"not the size of the repository", "never substitute a guessed file"),
	testabilityStated("one entry per criterion, anchored by mode", "Recipe: Review",
		"one entry per criterion, citing its ID", "every entry also cites a design section or a `file#symbol` anchor",
		"in design-input mode the entries cite the requirements only"),
	testabilityStated("anchors in the footprint", "Recipe: Review",
		"every controllability gap caused by that code cites its file and symbol"),
	testabilityStated("characterization before changing untested code", "Recipe: Review",
		"no test and no Walden proof command exercises", "propose characterization proofs that pin its current outputs before the change",
		"name the search behind the claim"),
	testabilityStated("requirements order", "The Testability Matrix", "One row per acceptance criterion, in requirements order"),
	testabilityStated("gap lines", "The Testability Matrix", "one line per gap", "proposed, accepted, or rejected with the user's reason"),
	testabilityStated("draft only, on acceptance", "The Anchoring Ceremony", "the CLI reads the design as `draft`", "Silence is not acceptance"),
	testabilityStated("level-three sub-heading replaced on every run", "The Anchoring Ceremony",
		"under the sub-heading `### Testability Matrix` at the end of the Verification Plan section",
		"replace everything under it", "never appends a second one"),
	testabilityStated("accepted changes in their sections", "The Anchoring Ceremony", "Write each accepted change into the design section it cites"),
	testabilityStated("rejected changes in Failure Modes And Tradeoffs", "The Anchoring Ceremony",
		"Record each rejected change as one entry in Failure Modes And Tradeoffs", "naming the criterion, the open gap and the user's reason"),
	testabilityStated("frontmatter untouched", "The Anchoring Ceremony", "Leave the frontmatter byte-identical"),
	testabilityStated("read-only validate after writing", "The Anchoring Ceremony",
		"Run `walden validate <feature>`, which is read-only", "never leave an invalid design behind"),
	testabilityStated("hand-back to the walden skill", "The Anchoring Ceremony", "Hand the design back to the walden skill's review loop"),
	testabilityStated("mode label on every review", "Output Standards",
		"Open every review with its mode label", "**design input**", "**review of the drafted design**", "**proposed revision**"),
	testabilityStated("next step", "Output Standards", "End with the next step and who takes it"),
}

// TestTestabilitySkillCeremony holds the review recipe, the matrix and the
// anchoring ceremony (R1.AC1, R1.AC4, R1.AC5, R5.AC1-AC3, R6, NFR2, NFR3).
func TestTestabilitySkillCeremony(t *testing.T) {
	t.Run("matrix columns", func(t *testing.T) {
		const header = "| Criterion | Stimulus | Observation | Oracle | Proof | Loop | Gap |"
		wrap := func(table string) string {
			return "# Fixture\n\n## The Testability Matrix\n\nOne row per criterion.\n\n" + table + "\n\n## Next\n\n| Other | Table |\n"
		}
		if err := checkTestabilityMatrix(wrap(header + "\n| --- | --- | --- | --- | --- | --- | --- |")); err != nil {
			t.Fatalf("the fixed column set was rejected: %v", err)
		}
		for name, table := range map[string]string{
			"missing column":   "| Criterion | Stimulus | Observation | Oracle | Proof | Gap |",
			"extra column":     "| Criterion | Stimulus | Observation | Oracle | Proof | Loop | Gap | Notes |",
			"reordered column": "| Criterion | Stimulus | Oracle | Observation | Proof | Loop | Gap |",
			"no table":         "Columns: Criterion, Stimulus, Observation, Oracle, Proof, Loop, Gap.",
		} {
			if checkTestabilityMatrix(wrap(table)) == nil {
				t.Errorf("a matrix template with a %s was accepted", name)
			}
		}
		if err := checkTestabilityMatrix(readTestabilityFile(t, testabilitySkillPath)); err != nil {
			t.Errorf("%s: %v", testabilitySkillPath, err)
		}
	})
	t.Run("recipe and ceremony", func(t *testing.T) {
		assertTestabilityRules(t, testabilityCeremonyRules, readTestabilityFile(t, testabilitySkillPath))
	})
}

// The ambient sources of R2.AC3, one seams entry each.
var testabilitySources = []string{"Clock", "Randomness", "Environment", "Network", "Filesystem", "Concurrency", "Global State", "External Service"}

// The language label each fenced example must carry, by info string.
var testabilityExampleLabels = map[string]string{
	"go": "Go", "ts": "TypeScript", "typescript": "TypeScript", "js": "JavaScript", "javascript": "JavaScript",
	"python": "Python", "py": "Python", "java": "Java", "csharp": "C#", "cs": "C#",
}

// labeledExamples counts the fenced blocks in a programming language whose
// nearest non-blank line above starts with that language's bold label.
func labeledExamples(section string) int {
	lines := strings.Split(section, "\n")
	count, fenced := 0, false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "```") {
			continue
		}
		if fenced {
			fenced = false
			continue
		}
		fenced = true
		label, known := testabilityExampleLabels[strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))]
		if !known {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			if above := strings.TrimSpace(lines[j]); above != "" {
				if strings.HasPrefix(above, "**"+label) && !strings.HasPrefix(above, "**"+label+"Script") {
					count++
				}
				break
			}
		}
	}
	return count
}

// checkTestabilitySeams requires one entry per ambient source with a platform
// control, a seam role, search hints and a labeled example (R2.AC6, NFR5).
func checkTestabilitySeams(content string) error {
	for _, source := range testabilitySources {
		section, err := testabilitySection(content, source)
		if err != nil {
			return err
		}
		if err := requireTestabilityFragments(section, "**Platform control.**", "**Seam role.**", "**Search hints.**"); err != nil {
			return fmt.Errorf("%s: %w", source, err)
		}
		if labeledExamples(section) == 0 {
			return fmt.Errorf("%s: no example under a language label", source)
		}
	}
	return nil
}

// checkTestabilitySkillCode requires SKILL.md to link the seams reference by
// a relative path and to hold no code block in a programming language: code
// lives in the reference, as labeled examples (NFR5).
func checkTestabilitySkillCode(content string) error {
	if !strings.Contains(content, "](references/seams.md)") {
		return fmt.Errorf("SKILL.md does not link references/seams.md")
	}
	fenced := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "```") {
			continue
		}
		if fenced {
			fenced = false
			continue
		}
		fenced = true
		switch info := strings.TrimSpace(strings.TrimPrefix(trimmed, "```")); info {
		case "markdown", "md", "text":
		default:
			return fmt.Errorf("SKILL.md holds a code block labeled %q", info)
		}
	}
	return nil
}

// TestTestabilitySeamsReference holds the seams reference and keeps code out
// of the always-loaded guide (R2.AC6, NFR5).
func TestTestabilitySeamsReference(t *testing.T) {
	entry := func(source, example string) string {
		return "## " + source + "\n\n- **Platform control.** A control.\n- **Seam role.** A role.\n- **Search hints.** Hints.\n\n" + example + "\n"
	}
	const goExample = "**Go**, a function value:\n\n```go\nvar now = time.Now\n```"
	reference := func(skip string, example string) string {
		var b strings.Builder
		b.WriteString("# Seams\n\n")
		for _, source := range testabilitySources {
			switch source {
			case skip:
			case testabilitySources[0]:
				b.WriteString(entry(source, example))
			default:
				b.WriteString(entry(source, goExample))
			}
		}
		return b.String()
	}

	t.Run("seams", func(t *testing.T) {
		if err := checkTestabilitySeams(reference("", goExample)); err != nil {
			t.Fatalf("a complete reference was rejected: %v", err)
		}
		for _, source := range testabilitySources {
			if checkTestabilitySeams(reference(source, goExample)) == nil {
				t.Errorf("a reference without %q was accepted", source)
			}
		}
		for name, fixture := range map[string]string{
			"no seam role":        strings.Replace(reference("", goExample), "- **Seam role.** A role.\n", "", 1),
			"no platform control": strings.Replace(reference("", goExample), "- **Platform control.** A control.\n", "", 1),
			"no search hints":     strings.Replace(reference("", goExample), "- **Search hints.** Hints.\n", "", 1),
			"unlabeled example":   reference("", "```go\nvar now = time.Now\n```"),
			"mislabeled example":  reference("", "**Java**, a function value:\n\n```go\nvar now = time.Now\n```"),
			"unlabeled language":  reference("", "**Go**, a function value:\n\n```\nvar now = time.Now\n```"),
		} {
			if checkTestabilitySeams(fixture) == nil {
				t.Errorf("a reference with %s was accepted", name)
			}
		}
		if err := checkTestabilitySeams(readTestabilityFile(t, testabilitySeamsPath)); err != nil {
			t.Errorf("%s: %v", testabilitySeamsPath, err)
		}
	})
	t.Run("guide holds no code", func(t *testing.T) {
		const linked = "# Guide\n\nShow the idiom from [references/seams.md](references/seams.md).\n"
		if err := checkTestabilitySkillCode(linked + "\n```markdown\n| Criterion |\n```\n"); err != nil {
			t.Fatalf("a guide with a Markdown template was rejected: %v", err)
		}
		for name, fixture := range map[string]string{
			"Go code block":      linked + "\n```go\nvar now = time.Now\n```\n",
			"unlabeled block":    linked + "\n```\nnow := time.Now()\n```\n",
			"no reference link":  "# Guide\n\nShow the idiom from the seams reference.\n",
			"link leaving skill": "# Guide\n\nShow the idiom from [seams](../references/seams.md).\n",
		} {
			if checkTestabilitySkillCode(fixture) == nil {
				t.Errorf("a guide with a %s was accepted", name)
			}
		}
		if err := checkTestabilitySkillCode(readTestabilityFile(t, testabilitySkillPath)); err != nil {
			t.Errorf("%s: %v", testabilitySkillPath, err)
		}
	})
}
