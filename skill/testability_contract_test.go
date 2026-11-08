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
// level-two section they must sit under.
type testabilityRule struct {
	name      string
	section   string
	fragments []string
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
	{"stop without the CLI", "Prerequisites", []string{
		"Run `walden version` before anything else", "If no walden executable answers, stop", "produce no review",
		"install instructions", "https://github.com/andrearaponi/walden", "never installs the CLI"}},
	{"state only from the CLI", "Product Boundary", []string{
		"Workflow state comes only from `walden status <feature> --json`", "never inferred from frontmatter text"}},
	{"single write path", "Product Boundary", []string{
		"The only write path is the body of a draft design", "only after the user accepts",
		"Frontmatter, requirements, tasks, the constitution, lessons and evidence are never written"}},
	{"hand-off for a design in review or approved", "Product Boundary", []string{
		"A design in review or approved is never edited", "proposed revision", "reconcile and re-review path"}},
	{"refusal to author, open for review or approve", "Product Boundary", []string{
		"author, open for review or approve", "declined and routed to the walden skill", "runs no `walden review`"}},
	{"decline on unapproved or stale requirements", "Entry Check", []string{
		"`walden status <feature> --json`", "Unless `requirements.md` reads `approved` with `fresh: true`",
		"decline the review", "route the user to the walden skill's requirements phase"}},
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
