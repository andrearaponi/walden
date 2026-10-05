package consolidate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Scope link kinds.
const (
	LinkCitedBy    = "cited-by"
	LinkCites      = "cites"
	LinkSharedFile = "shared-file"
)

// Finding kinds.
const (
	FindingMissingFile       = "missing-file"
	FindingDanglingReference = "dangling-reference"
	FindingReservedReuse     = "reserved-reuse"
	FindingViewMismatch      = "view-mismatch"
)

// HubThreshold is the number of citing features that makes a cited file a
// hub. Features linked only through hubs stay in scope but are listed apart.
const HubThreshold = 5

// ScopeLink explains why a feature is in the review scope: it is cited by a
// pending feature, cites one, or shares a cited file with one.
type ScopeLink struct {
	Kind    string
	Feature string
	Path    string
}

// ScopeEntry is one feature of the review scope. Pending holds the change
// kind of a pending feature, empty for a linked one.
type ScopeEntry struct {
	Feature string
	Pending string
	Links   []ScopeLink
	HubOnly bool
}

// Hub is a file cited by at least HubThreshold features.
type Hub struct {
	Path     string
	Features []string
}

// Finding is one deterministic, advisory problem. ID is the citing or
// affected identifier; Subject is the cited path or qualified reference.
type Finding struct {
	Kind    string
	Feature string
	ID      string
	Subject string
	Message string
}

// Report is the read-only consolidation report.
type Report struct {
	State       State
	Scope       []ScopeEntry
	Hubs        []Hub
	Findings    []Finding
	Identifiers map[string][]string
	Comparisons []Comparison
}

// BuildReport assembles the bounded scope, the deterministic findings and the
// defined identifiers of the scoped features. It writes nothing.
func BuildReport(root string, snapshot Snapshot, record *Record, state State) Report {
	report := Report{State: state, Identifiers: map[string][]string{}}
	report.Scope, report.Hubs = Scope(snapshot, state)
	report.Findings = append(report.Findings, MissingCitedFiles(root, snapshot)...)
	report.Findings = append(report.Findings, DanglingReferences(snapshot)...)
	if state.Tracking == TrackingActive {
		report.Findings = append(report.Findings, ReservedReuse(snapshot, record)...)
		report.Comparisons = Comparisons(snapshot, record, state, report.Scope)
	}
	for _, entry := range report.Scope {
		if feature, ok := snapshot.Feature(entry.Feature); ok && feature.HasRequirements {
			report.Identifiers[entry.Feature] = feature.Definitions.Active()
		}
	}
	return report
}

// Scope lists the pending features, then every feature linked to them in
// either direction by a qualified reference or a shared cited file.
func Scope(snapshot Snapshot, state State) ([]ScopeEntry, []Hub) {
	citers := fileCiters(snapshot)
	hubs := hubFiles(snapshot)
	hubPaths := map[string]bool{}
	for _, hub := range hubs {
		hubPaths[hub.Path] = true
	}

	entries := map[string]*ScopeEntry{}
	order := []string{}
	ensure := func(name string) *ScopeEntry {
		if entry, ok := entries[name]; ok {
			return entry
		}
		entries[name] = &ScopeEntry{Feature: name}
		order = append(order, name)
		return entries[name]
	}
	link := func(name string, value ScopeLink) {
		entry := ensure(name)
		for _, existing := range entry.Links {
			if existing == value {
				return
			}
		}
		entry.Links = append(entry.Links, value)
	}

	for _, change := range state.Pending {
		ensure(change.Feature).Pending = change.Kind
	}
	pendingCount := len(order)
	for _, change := range state.Pending {
		if feature, ok := snapshot.Feature(change.Feature); ok {
			for _, reference := range feature.Citations.References {
				if _, exists := snapshot.Feature(reference.Feature); exists && reference.Feature != change.Feature {
					link(reference.Feature, ScopeLink{Kind: LinkCitedBy, Feature: change.Feature})
				}
			}
			for _, file := range feature.Citations.Files {
				for _, other := range citers[file.Path] {
					if other != change.Feature {
						link(other, ScopeLink{Kind: LinkSharedFile, Feature: change.Feature, Path: file.Path})
					}
				}
			}
		}
		for _, other := range snapshot.Features {
			if other.Name == change.Feature {
				continue
			}
			for _, reference := range other.Citations.References {
				if reference.Feature == change.Feature {
					link(other.Name, ScopeLink{Kind: LinkCites, Feature: change.Feature})
				}
			}
		}
	}

	linked := append([]string{}, order[pendingCount:]...)
	sort.Strings(linked)
	scope := []ScopeEntry{}
	for _, name := range append(order[:pendingCount:pendingCount], linked...) {
		entry := entries[name]
		if entry.Pending == "" {
			entry.HubOnly = len(entry.Links) > 0
			for _, value := range entry.Links {
				if value.Kind != LinkSharedFile || !hubPaths[value.Path] {
					entry.HubOnly = false
				}
			}
		}
		scope = append(scope, *entry)
	}
	return scope, hubs
}

func fileCiters(snapshot Snapshot) map[string][]string {
	citers := map[string][]string{}
	for _, feature := range snapshot.Features {
		for _, file := range feature.Citations.Files {
			if !containsString(citers[file.Path], feature.Name) {
				citers[file.Path] = append(citers[file.Path], feature.Name)
			}
		}
	}
	return citers
}

// hubFiles lists the files cited by at least HubThreshold features.
func hubFiles(snapshot Snapshot) []Hub {
	hubs := []Hub{}
	for path, features := range fileCiters(snapshot) {
		if len(features) >= HubThreshold {
			sorted := append([]string{}, features...)
			sort.Strings(sorted)
			hubs = append(hubs, Hub{Path: path, Features: sorted})
		}
	}
	sort.Slice(hubs, func(i, j int) bool { return hubs[i].Path < hubs[j].Path })
	return hubs
}

// MissingCitedFiles reports every cited repository file that does not exist,
// for every feature with requirements.
func MissingCitedFiles(root string, snapshot Snapshot) []Finding {
	findings := []Finding{}
	for _, feature := range snapshot.Features {
		for _, file := range feature.Citations.Files {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(file.Path))); err == nil {
				continue
			}
			findings = append(findings, Finding{Kind: FindingMissingFile, Feature: feature.Name, ID: file.CitedBy, Subject: file.Path,
				Message: fmt.Sprintf("%s cites %s, which does not exist", label(feature.Name, file.CitedBy), file.Path)})
		}
	}
	return findings
}

// DanglingReferences reports qualified references whose feature or
// identifier does not exist.
func DanglingReferences(snapshot Snapshot) []Finding {
	findings := []Finding{}
	for _, feature := range snapshot.Features {
		for _, reference := range feature.Citations.References {
			subject := reference.Feature + "#" + reference.ID
			problem := ""
			target, ok := snapshot.Feature(reference.Feature)
			switch {
			case !ok:
				problem = fmt.Sprintf("feature %s does not exist", reference.Feature)
			case !target.HasRequirements || target.LoadError != "":
				problem = fmt.Sprintf("the requirements of %s cannot be read", reference.Feature)
			case !target.Definitions.Contains(reference.ID):
				problem = fmt.Sprintf("%s does not define %s", reference.Feature, reference.ID)
			default:
				continue
			}
			findings = append(findings, Finding{Kind: FindingDanglingReference, Feature: feature.Name, ID: reference.CitedBy, Subject: subject,
				Message: fmt.Sprintf("%s cites %s, but %s", label(feature.Name, reference.CitedBy), subject, problem)})
		}
	}
	return findings
}

// ReservedReuse reports identifiers that the last consolidation recorded as
// removed or reserved and that the feature's requirements define again.
func ReservedReuse(snapshot Snapshot, record *Record) []Finding {
	findings := []Finding{}
	if record == nil {
		return findings
	}
	for _, feature := range snapshot.Features {
		for _, id := range record.Features[feature.Name].ReservedIDs {
			if feature.Definitions.Contains(id) {
				findings = append(findings, Finding{Kind: FindingReservedReuse, Feature: feature.Name, ID: id,
					Message: fmt.Sprintf("%s defines %s again, but the last consolidation reserved it", feature.Name, id)})
			}
		}
	}
	return findings
}

func label(feature, id string) string {
	if id == "" {
		return feature
	}
	return feature + " " + id
}

func containsString(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}
