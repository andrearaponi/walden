package output

import (
	"fmt"
	"io"
	"strings"
)

// ConsolidationStatus is the JSON and text view of the consolidation state.
// Scope, hubs, findings and identifiers appear only in the consolidation report.
type ConsolidationStatus struct {
	Tracking       string                    `json:"tracking"`
	Pending        []ConsolidationChange     `json:"pending"`
	Backlog        []string                  `json:"backlog"`
	Unconsolidated int                       `json:"unconsolidated,omitempty"`
	Threshold      string                    `json:"threshold"`
	View           string                    `json:"view"`
	Problem        string                    `json:"problem,omitempty"`
	Remedy         string                    `json:"remedy,omitempty"`
	Scope          []ConsolidationScopeEntry `json:"scope,omitempty"`
	Hubs           []ConsolidationHub        `json:"hubs,omitempty"`
	Findings       []ConsolidationFinding    `json:"findings,omitempty"`
	Identifiers    map[string][]string       `json:"identifiers,omitempty"`
	Comparisons    []ConsolidationComparison `json:"comparisons,omitempty"`
	// Brief renders one text line, and only when there is something to say.
	Brief bool `json:"-"`
}

// ConsolidationChange is one pending contract change.
type ConsolidationChange struct {
	Feature string `json:"feature"`
	Kind    string `json:"kind"`
}

// ConsolidationScopeEntry is one feature of the review scope.
type ConsolidationScopeEntry struct {
	Feature string              `json:"feature"`
	Pending string              `json:"pending,omitempty"`
	HubOnly bool                `json:"hub_only,omitempty"`
	Links   []ConsolidationLink `json:"links,omitempty"`
}

// ConsolidationLink explains why a feature is in scope.
type ConsolidationLink struct {
	Kind    string `json:"kind"`
	Feature string `json:"feature"`
	Path    string `json:"path,omitempty"`
}

// ConsolidationHub is a file cited by many features.
type ConsolidationHub struct {
	Path     string   `json:"path"`
	Features []string `json:"features"`
}

// ConsolidationComparison puts a pending feature's statements next to the
// statements of the features linked to it.
type ConsolidationComparison struct {
	Feature    string                   `json:"feature"`
	Kind       string                   `json:"kind"`
	Statements []ConsolidationStatement `json:"statements"`
	Linked     []ConsolidationLinked    `json:"linked,omitempty"`
}

// ConsolidationStatement is one criterion, NFR or constraint with its text;
// a changed statement also carries its recorded previous text.
type ConsolidationStatement struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Mark     string `json:"mark,omitempty"`
	Previous string `json:"previous,omitempty"`
}

// ConsolidationLinked is a linked feature; hub-only features carry no statements.
type ConsolidationLinked struct {
	Feature    string                   `json:"feature"`
	HubOnly    bool                     `json:"hub_only,omitempty"`
	Links      []ConsolidationLink      `json:"links,omitempty"`
	Statements []ConsolidationStatement `json:"statements,omitempty"`
}

// ConsolidationFinding is one advisory deterministic finding.
type ConsolidationFinding struct {
	Kind    string `json:"kind"`
	Feature string `json:"feature"`
	ID      string `json:"id,omitempty"`
	Subject string `json:"subject,omitempty"`
	Message string `json:"message"`
}

func printConsolidation(w io.Writer, status *ConsolidationStatus) {
	pending := make([]string, 0, len(status.Pending))
	for _, change := range status.Pending {
		pending = append(pending, fmt.Sprintf("%s (%s)", change.Feature, change.Kind))
	}
	if status.Brief {
		switch {
		case status.Tracking == "not-started" && status.Unconsolidated > 0:
			_, _ = fmt.Fprintf(w, "Consolidation: not started; %d approved feature(s) not consolidated (walden consolidate start)\n", status.Unconsolidated)
		case status.Tracking == "unknown":
			_, _ = fmt.Fprintf(w, "Consolidation: unknown state\n")
		case len(pending) > 0 || status.View == "stale":
			_, _ = fmt.Fprintf(w, "Consolidation: %d pending change(s)%s; view %s\n", len(pending), joinedSuffix(pending), status.View)
		}
		return
	}

	_, _ = fmt.Fprintf(w, "Consolidation: %s; threshold %s; view %s\n", status.Tracking, status.Threshold, status.View)
	if len(pending) > 0 {
		_, _ = fmt.Fprintf(w, "- pending: %s\n", strings.Join(pending, ", "))
	}
	if len(status.Backlog) > 0 {
		_, _ = fmt.Fprintf(w, "- backlog: %d feature(s): %s\n", len(status.Backlog), strings.Join(status.Backlog, ", "))
	}
	if status.Unconsolidated > 0 {
		_, _ = fmt.Fprintf(w, "- not consolidated: %d approved feature(s)\n", status.Unconsolidated)
	}
	if status.Problem != "" {
		_, _ = fmt.Fprintf(w, "- problem: %s\n- remedy: %s\n", status.Problem, status.Remedy)
	}
	if len(status.Scope) > 0 {
		_, _ = fmt.Fprintln(w, "Scope:")
		hubOnly := []string{}
		for _, entry := range status.Scope {
			if entry.HubOnly {
				hubOnly = append(hubOnly, entry.Feature)
				continue
			}
			reasons := []string{}
			if entry.Pending != "" {
				reasons = append(reasons, "pending ("+entry.Pending+")")
			}
			for _, link := range entry.Links {
				switch link.Kind {
				case "cited-by":
					reasons = append(reasons, "cited by "+link.Feature)
				case "cites":
					reasons = append(reasons, "cites "+link.Feature)
				case "shared-file":
					reasons = append(reasons, "shares "+link.Path+" with "+link.Feature)
				}
			}
			_, _ = fmt.Fprintf(w, "- %s: %s\n", entry.Feature, strings.Join(reasons, "; "))
		}
		if len(hubOnly) > 0 {
			_, _ = fmt.Fprintf(w, "- linked only through widely cited files: %s\n", strings.Join(hubOnly, ", "))
		}
	}
	if len(status.Hubs) > 0 {
		_, _ = fmt.Fprintln(w, "Widely cited files:")
		for _, hub := range status.Hubs {
			_, _ = fmt.Fprintf(w, "- %s: %d feature(s)\n", hub.Path, len(hub.Features))
		}
	}
	if len(status.Findings) > 0 {
		_, _ = fmt.Fprintln(w, "Findings:")
		for _, finding := range status.Findings {
			_, _ = fmt.Fprintf(w, "- [%s] %s\n", finding.Kind, finding.Message)
		}
	}
	if len(status.Comparisons) > 0 {
		_, _ = fmt.Fprintln(w, "Comparisons:")
		for _, comparison := range status.Comparisons {
			_, _ = fmt.Fprintf(w, "- %s (%s):\n", comparison.Feature, comparison.Kind)
			for _, statement := range comparison.Statements {
				_, _ = fmt.Fprintf(w, "    [%s] %s %s\n", statement.Mark, statement.ID, statement.Text)
				if statement.Previous != "" {
					_, _ = fmt.Fprintf(w, "      was: %s\n", statement.Previous)
				}
			}
			for _, linked := range comparison.Linked {
				if linked.HubOnly {
					_, _ = fmt.Fprintf(w, "  linked %s: only through widely cited files; read its view entry\n", linked.Feature)
					continue
				}
				_, _ = fmt.Fprintf(w, "  linked %s:\n", linked.Feature)
				for _, statement := range linked.Statements {
					_, _ = fmt.Fprintf(w, "    %s %s\n", statement.ID, statement.Text)
				}
			}
		}
	}
	if len(status.Identifiers) > 0 {
		_, _ = fmt.Fprintln(w, "Identifiers:")
		for _, entry := range status.Scope {
			if ids, ok := status.Identifiers[entry.Feature]; ok {
				_, _ = fmt.Fprintf(w, "- %s: %s\n", entry.Feature, strings.Join(ids, ", "))
			}
		}
	}
}

func joinedSuffix(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return " (" + strings.Join(items, ", ") + ")"
}
