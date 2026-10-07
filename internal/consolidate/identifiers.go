// Package consolidate derives the consolidation state of a specification
// portfolio: pending contract changes, the bounded review scope, deterministic
// findings and the current-contract view.
package consolidate

import (
	"regexp"
	"strings"

	"github.com/andrearaponi/walden/internal/spec"
)

var (
	criterionDefinition  = regexp.MustCompile("^\\s*\\d+\\.\\s+`(" + spec.CriterionIDExpr + ")`")
	nfrDefinition        = regexp.MustCompile("^\\s*-\\s+`(" + spec.NFRIDExpr + ")`")
	constraintDefinition = regexp.MustCompile("^\\s*-\\s+`(" + spec.ConstraintIDExpr + ")`")
	requirementHeading   = regexp.MustCompile(`^### (` + spec.RequirementIDExpr + `)\b`)
	inlineCode           = regexp.MustCompile("`([^`\n]+)`")
	citedPath            = regexp.MustCompile(`^(?:\.?[A-Za-z0-9_][A-Za-z0-9._-]*/)+[A-Za-z0-9_][A-Za-z0-9._-]*\.[A-Za-z][A-Za-z0-9]*$`)
	qualifiedReference   = regexp.MustCompile(`^([a-z0-9]+(?:-[a-z0-9]+)*)#(` + spec.RequirementIDExpr + `(?:\.AC\d+)?|` + spec.NFRIDExpr + `|` + spec.ConstraintIDExpr + `)$`)
)

// Definitions lists the identifiers a requirements body defines, in document
// order. Mentions elsewhere — bridges, tables, prose — are not definitions.
type Definitions struct {
	Requirements []string
	Criteria     []string
	NFRs         []string
	Constraints  []string
}

// Active returns the criteria, NFRs and constraints a current-contract view lists.
func (d Definitions) Active() []string {
	active := make([]string, 0, len(d.Criteria)+len(d.NFRs)+len(d.Constraints))
	active = append(active, d.Criteria...)
	active = append(active, d.NFRs...)
	return append(active, d.Constraints...)
}

// Contains reports whether id is defined, including requirement headings.
func (d Definitions) Contains(id string) bool {
	for _, list := range [][]string{d.Requirements, d.Criteria, d.NFRs, d.Constraints} {
		for _, defined := range list {
			if defined == id {
				return true
			}
		}
	}
	return false
}

// DefinedIdentifiers parses definition lines only: requirement headings,
// numbered criterion items and NFR or constraint bullets. Fenced blocks are
// examples, never definitions.
func DefinedIdentifiers(body string) Definitions {
	var definitions Definitions
	seen := map[string]bool{}
	add := func(list *[]string, id string) {
		if !seen[id] {
			seen[id] = true
			*list = append(*list, id)
		}
	}
	forEachLine(body, func(line string) {
		if match := requirementHeading.FindStringSubmatch(line); match != nil {
			add(&definitions.Requirements, match[1])
		} else if match := criterionDefinition.FindStringSubmatch(line); match != nil {
			add(&definitions.Criteria, match[1])
		} else if match := nfrDefinition.FindStringSubmatch(line); match != nil {
			add(&definitions.NFRs, match[1])
		} else if match := constraintDefinition.FindStringSubmatch(line); match != nil {
			add(&definitions.Constraints, match[1])
		}
	})
	return definitions
}

// FileCitation is a repository file cited in a requirements body. CitedBy is
// the identifier whose definition contains the citation, empty at feature level.
type FileCitation struct {
	Path    string
	CitedBy string
}

// Reference is a feature-qualified identifier cited in a requirements body.
type Reference struct {
	Feature string
	ID      string
	CitedBy string
}

// Citations lists the cited files and qualified references of a body.
type Citations struct {
	Files      []FileCitation
	References []Reference
}

// ParseCitations collects citations from inline code outside fenced blocks
// and acceptance-check lines, where examples usually live. A cited file is a
// relative path with a directory and an extension that begins with a letter,
// so that a version or model identifier such as `claude-opus-5.5` is not one; a
// qualified reference is `feature#ID`.
func ParseCitations(body string) Citations {
	var citations Citations
	seen := map[string]bool{}
	current := ""
	forEachLine(body, func(line string) {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "#"):
			current = ""
			if match := requirementHeading.FindStringSubmatch(line); match != nil {
				current = match[1]
			}
		case criterionDefinition.MatchString(line):
			current = criterionDefinition.FindStringSubmatch(line)[1]
		case nfrDefinition.MatchString(line):
			current = nfrDefinition.FindStringSubmatch(line)[1]
		case constraintDefinition.MatchString(line):
			current = constraintDefinition.FindStringSubmatch(line)[1]
		}
		if strings.HasPrefix(trimmed, "- Acceptance check:") {
			return
		}
		for _, span := range inlineCode.FindAllStringSubmatch(line, -1) {
			text := strings.TrimSpace(span[1])
			if match := qualifiedReference.FindStringSubmatch(text); match != nil {
				key := "ref\x00" + match[1] + "\x00" + match[2] + "\x00" + current
				if !seen[key] {
					seen[key] = true
					citations.References = append(citations.References, Reference{Feature: match[1], ID: match[2], CitedBy: current})
				}
			} else if citedPath.MatchString(text) {
				key := "file\x00" + text + "\x00" + current
				if !seen[key] {
					seen[key] = true
					citations.Files = append(citations.Files, FileCitation{Path: text, CitedBy: current})
				}
			}
		}
	})
	return citations
}

// forEachLine visits the lines outside fenced code blocks.
func forEachLine(body string, visit func(line string)) {
	fenced := false
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
			continue
		}
		if !fenced {
			visit(line)
		}
	}
}
