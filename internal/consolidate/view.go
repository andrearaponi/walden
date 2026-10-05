package consolidate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/andrearaponi/walden/internal/spec"
)

var (
	viewHeading    = regexp.MustCompile("^##\\s+`?([^`\\s]+)`?\\s*$")
	viewField      = regexp.MustCompile(`^(?i)(purpose|active|reserved|sources|related)\s*:\s*(.*)$`)
	viewIdentifier = regexp.MustCompile(`^(?:` + spec.CriterionIDExpr + `|` + spec.NFRIDExpr + `|` + spec.ConstraintIDExpr + `)\b`)
)

// ViewSection is one feature's entry in the current-contract view. Invalid
// holds Active or Reserved items that are not identifiers.
type ViewSection struct {
	Feature  string
	Purpose  string
	Active   []string
	Reserved []string
	Sources  []string
	Related  []string
	Invalid  []string
}

// View is the parsed current-contract view. The agent writes its body; the
// CLI owns its frontmatter.
type View struct {
	Exists   bool
	Document spec.Document
	Sections []ViewSection
}

// LoadView reads the view; an absent view is not an error.
func LoadView(root string) (View, error) {
	path := filepath.Join(root, ViewPath)
	text, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return View{}, nil
	}
	if err != nil {
		return View{}, fmt.Errorf("read %s: %w", ViewPath, err)
	}
	document, err := spec.ParseDocument(path, text)
	if err != nil {
		return View{}, fmt.Errorf("parse %s: %w", ViewPath, err)
	}
	return View{Exists: true, Document: document, Sections: ParseViewBody(document.Body)}, nil
}

// ParseViewBody parses `## <feature>` sections and their Purpose, Active,
// Reserved, Sources and Related lines. Other text and comments are ignored.
func ParseViewBody(body string) []ViewSection {
	sections := []ViewSection{}
	current := -1
	inComment := false
	forEachLine(body, func(line string) {
		trimmed := strings.TrimSpace(line)
		if inComment {
			inComment = !strings.Contains(trimmed, "-->")
			return
		}
		if strings.HasPrefix(trimmed, "<!--") {
			inComment = !strings.Contains(trimmed, "-->")
			return
		}
		if strings.HasPrefix(trimmed, "## ") {
			if match := viewHeading.FindStringSubmatch(trimmed); match != nil {
				sections = append(sections, ViewSection{Feature: match[1]})
				current = len(sections) - 1
				return
			}
		}
		if strings.HasPrefix(trimmed, "#") {
			current = -1
			return
		}
		match := viewField.FindStringSubmatch(trimmed)
		if current < 0 || match == nil {
			return
		}
		section := &sections[current]
		value := strings.TrimSpace(match[2])
		switch strings.ToLower(match[1]) {
		case "purpose":
			section.Purpose = value
		case "active":
			ids, invalid := parseIdentifierList(value)
			section.Active = append(section.Active, ids...)
			section.Invalid = append(section.Invalid, invalid...)
		case "reserved":
			ids, invalid := parseIdentifierList(value)
			section.Reserved = append(section.Reserved, ids...)
			section.Invalid = append(section.Invalid, invalid...)
		case "sources":
			section.Sources = append(section.Sources, parseList(value)...)
		case "related":
			section.Related = append(section.Related, parseList(value)...)
		}
	})
	return sections
}

func parseList(value string) []string {
	items := []string{}
	for _, item := range strings.Split(value, ",") {
		item = strings.Trim(strings.TrimSpace(item), "`")
		if item == "" || item == "-" || item == "\u2014" || strings.EqualFold(item, "none") {
			continue
		}
		items = append(items, item)
	}
	return items
}

func parseIdentifierList(value string) (ids, invalid []string) {
	for _, item := range parseList(value) {
		if id := viewIdentifier.FindString(item); id != "" {
			ids = append(ids, id)
		} else {
			invalid = append(invalid, item)
		}
	}
	return ids, invalid
}

// Status derives the view's review state; an approved view whose body no
// longer matches its approval fingerprint is stale.
func (v View) Status() ViewStatus {
	if !v.Exists {
		return ViewStatus{State: ViewAbsent}
	}
	status := ViewStatus{ApprovedFingerprint: v.Document.ApprovedFingerprint}
	switch v.Document.Status {
	case "approved":
		status.State = ViewStale
		if spec.BodyMatchesFingerprint(v.Document.Path, v.Document.Body, v.Document.ApprovedFingerprint) {
			status.State = ViewApproved
		}
	case "in-review":
		status.State = ViewInReview
	default:
		status.State = ViewDraft
	}
	return status
}

// CheckView reports every mismatch between the view and the specifications.
// Mismatches are reported, never resolved.
func CheckView(view View, snapshot Snapshot, record *Record) []Finding {
	findings := []Finding{}
	mismatch := func(feature, id, message string) {
		findings = append(findings, Finding{Kind: FindingViewMismatch, Feature: feature, ID: id, Message: message})
	}
	seen := map[string]bool{}
	for _, section := range view.Sections {
		feature, ok := snapshot.Feature(section.Feature)
		switch {
		case !ok || !feature.HasRequirements:
			mismatch(section.Feature, "", fmt.Sprintf("the view lists %s, which has no requirements under .walden/specs/", section.Feature))
			continue
		case seen[section.Feature]:
			mismatch(section.Feature, "", fmt.Sprintf("the view lists %s more than once", section.Feature))
			continue
		}
		seen[section.Feature] = true

		listed := toSet(section.Active)
		for _, id := range feature.Definitions.Active() {
			if !listed[id] {
				mismatch(section.Feature, id, fmt.Sprintf("the view omits %s %s, which the requirements define", section.Feature, id))
			}
		}
		for _, id := range section.Active {
			if !feature.Definitions.Contains(id) {
				mismatch(section.Feature, id, fmt.Sprintf("the view lists %s %s as active, but the requirements do not define it", section.Feature, id))
			}
		}
		for _, item := range section.Invalid {
			mismatch(section.Feature, item, fmt.Sprintf("the view lists %s %s, which is not a criterion, NFR or constraint identifier", section.Feature, item))
		}
		for _, id := range section.Reserved {
			if feature.Definitions.Contains(id) {
				mismatch(section.Feature, id, fmt.Sprintf("the view reserves %s %s, but the requirements define it", section.Feature, id))
			}
		}
		if section.Purpose == "" {
			mismatch(section.Feature, "", fmt.Sprintf("the view section %s has no Purpose line", section.Feature))
		}
		if record != nil {
			reserved := toSet(section.Reserved)
			for _, id := range record.Features[section.Feature].ActiveIDs {
				if !feature.Definitions.Contains(id) && !reserved[id] {
					mismatch(section.Feature, id, fmt.Sprintf("%s %s was active at the last consolidation and is no longer defined; list it under Reserved", section.Feature, id))
				}
			}
			for _, id := range record.Features[section.Feature].ReservedIDs {
				if !reserved[id] && !feature.Definitions.Contains(id) {
					mismatch(section.Feature, id, fmt.Sprintf("%s %s was reserved at the last consolidation; keep it under Reserved", section.Feature, id))
				}
			}
		}
	}
	if record != nil {
		names := make([]string, 0, len(record.Features))
		for name := range record.Features {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			feature, exists := snapshot.Feature(name)
			if record.Features[name].State == StateConsolidated && exists && feature.HasRequirements && !seen[name] {
				mismatch(name, "", fmt.Sprintf("the view omits the consolidated feature %s", name))
			}
		}
	}
	return findings
}

func toSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return set
}
