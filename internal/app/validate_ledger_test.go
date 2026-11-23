package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readableLedger is a ledger of the feature demo that every command can
// load. Its one record is bound to fingerprints no document has, so it is
// stale: validation reads it and does not judge it.
const readableLedger = `{
  "schema_version": "v1alpha2",
  "feature": "demo",
  "tasks": {
    "1.1": {
      "task_fingerprint": "sha256:0000000000000000000000000000000000000000000000000000000000000001",
      "requirements_fingerprint": "sha256:0000000000000000000000000000000000000000000000000000000000000002",
      "design_fingerprint": "sha256:0000000000000000000000000000000000000000000000000000000000000003",
      "tasks_fingerprint": "sha256:0000000000000000000000000000000000000000000000000000000000000004",
      "steps": [
        {
          "command": ["go", "test", "./..."],
          "expected_exit": 0,
          "actual_exit": 0
        }
      ],
      "result": "passed",
      "verified_at": "2026-03-21T15:00:00Z"
    }
  }
}
`

// validateLedgerRepo returns a temporary directory that holds the valid feature of
// the validate tests under each given name, and makes it the working
// directory.
func validateLedgerRepo(t *testing.T, features ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, feature := range features {
		writeValidValidateFeature(t, root, feature)
	}
	chdirForPortfolio(t, root)
	return root
}

func validateLedgerPath(feature string) string {
	return ".walden/evidence/" + feature + ".json"
}

func writeValidateLedger(t *testing.T, root, feature, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(validateLedgerPath(feature)))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create the evidence directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write the ledger of %s: %v", feature, err)
	}
}

func readValidateLedger(t *testing.T, root, feature string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(validateLedgerPath(feature))))
	if err != nil {
		t.Fatalf("read the ledger of %s: %v", feature, err)
	}
	return string(content)
}

// validateResultKeys lists the keys of the result object of a JSON envelope.
func validateResultKeys(t *testing.T, stdout string) map[string]bool {
	t.Helper()
	_, result := rerunResult(t, stdout)
	keys := map[string]bool{}
	for key := range result {
		keys[key] = true
	}
	return keys
}

func validateResultSummary(t *testing.T, stdout string) string {
	t.Helper()
	_, result := rerunResult(t, stdout)
	var summary string
	if err := json.Unmarshal(result["summary"], &summary); err != nil {
		t.Fatalf("decode the summary: %v\n%s", err, stdout)
	}
	return summary
}

func TestValidateFailsOnUnreadableLedger(t *testing.T) {
	cases := []struct {
		name    string
		ledger  string
		cause   []string
		markers bool
	}{
		{
			name:    "conflict markers around a valid ledger",
			ledger:  "<<<<<<< ours\n" + readableLedger + "=======\n{}\n>>>>>>> theirs\n",
			cause:   []string{"git conflict markers", "parse"},
			markers: true,
		},
		{
			name:   "truncated ledger",
			ledger: readableLedger[:len(readableLedger)/2],
			cause:  []string{"parse"},
		},
		{
			name:   "unknown schema version",
			ledger: `{"schema_version": "v9", "feature": "demo", "tasks": {}}` + "\n",
			cause:  []string{"schema v9", "unsupported"},
		},
		{
			name:   "ledger of another feature",
			ledger: `{"schema_version": "v1alpha2", "feature": "other", "tasks": {}}` + "\n",
			cause:  []string{`"other"`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := validateLedgerRepo(t, "demo")
			valid, _ := rerunCommand(t, 0, "validate", "demo", "--json")
			validKeys := validateResultKeys(t, valid)
			writeValidateLedger(t, root, "demo", tc.ledger)

			for _, args := range [][]string{{"validate", "demo"}, {"validate", "demo", "--all"}} {
				_, stderr := rerunCommand(t, 1, args...)
				if !strings.Contains(stderr, "Summary: INVALID: ") {
					t.Errorf("walden %s does not report the feature as invalid:\n%s", strings.Join(args, " "), stderr)
				}
				for _, fragment := range append([]string{validateLedgerPath("demo")}, tc.cause...) {
					if !strings.Contains(stderr, fragment) {
						t.Errorf("walden %s does not mention %q:\n%s", strings.Join(args, " "), fragment, stderr)
					}
				}
			}

			stdout, _ := rerunCommand(t, 1, "validate", "demo", "--all", "--json")
			if ok, _ := rerunResult(t, stdout); ok {
				t.Errorf("the JSON result reports ok for an unreadable ledger: %s", stdout)
			}
			summary := validateResultSummary(t, stdout)
			if !strings.HasPrefix(summary, "INVALID: ") {
				t.Errorf("the summary does not begin with INVALID: %s", summary)
			}
			for _, fragment := range append([]string{validateLedgerPath("demo")}, tc.cause...) {
				if !strings.Contains(summary, fragment) {
					t.Errorf("the summary does not mention %q: %s", fragment, summary)
				}
			}
			if got := strings.Contains(summary, "git conflict markers"); got != tc.markers {
				t.Errorf("the summary names conflict markers = %v, want %v: %s", got, tc.markers, summary)
			}
			for key := range validateResultKeys(t, stdout) {
				if !validKeys[key] {
					t.Errorf("the result of the invalid feature holds the key %q, which the valid one lacks", key)
				}
			}

			if got := readValidateLedger(t, root, "demo"); got != tc.ledger {
				t.Errorf("validation changed the ledger:\n got %q\nwant %q", got, tc.ledger)
			}

			if err := os.Remove(filepath.Join(root, filepath.FromSlash(validateLedgerPath("demo")))); err != nil {
				t.Fatalf("remove the ledger: %v", err)
			}
			rerunCommand(t, 0, "validate", "demo")
			rerunCommand(t, 0, "validate", "demo", "--all")
		})
	}
}

func TestValidateIgnoresMissingOrReadableLedger(t *testing.T) {
	root := validateLedgerRepo(t, "demo")

	for _, args := range [][]string{
		{"validate", "demo", "--json"},
		{"validate", "demo", "--all", "--json"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			_ = os.Remove(filepath.Join(root, filepath.FromSlash(validateLedgerPath("demo"))))

			without, _ := rerunCommand(t, 0, args...)
			if got := validateResultSummary(t, without); got != "VALID: .walden/specs/demo" {
				t.Errorf("summary with no ledger = %q, want %q", got, "VALID: .walden/specs/demo")
			}

			writeValidateLedger(t, root, "demo", readableLedger)
			with, _ := rerunCommand(t, 0, args...)
			if with != without {
				t.Errorf("a readable ledger changed the result:\n got %s\nwant %s", with, without)
			}
			if got := readValidateLedger(t, root, "demo"); got != readableLedger {
				t.Errorf("validation changed the ledger:\n got %q\nwant %q", got, readableLedger)
			}
		})
	}
}

func TestValidatePortfolioCoversLedgers(t *testing.T) {
	root := validateLedgerRepo(t, "aaa-feature", "bbb-feature")
	valid, _ := rerunCommand(t, 0, "validate", "--json")
	validKeys := validateResultKeys(t, valid)
	writeValidateLedger(t, root, "bbb-feature", "this is not a ledger\n")

	stdout, _ := rerunCommand(t, 1, "validate", "--json")

	_, result := rerunResult(t, stdout)
	var features []struct {
		Feature string `json:"feature"`
		Valid   bool   `json:"valid"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(result["features"], &features); err != nil {
		t.Fatalf("decode the features: %v\n%s", err, stdout)
	}
	if len(features) != 2 {
		t.Fatalf("features = %+v, want two verdicts", features)
	}
	if features[0].Feature != "aaa-feature" || !features[0].Valid {
		t.Errorf("the feature with no ledger is not valid: %+v", features[0])
	}
	if features[1].Feature != "bbb-feature" || features[1].Valid {
		t.Errorf("the feature with an unreadable ledger is not invalid: %+v", features[1])
	}
	for _, fragment := range []string{"INVALID: ", validateLedgerPath("bbb-feature"), "parse"} {
		if !strings.Contains(features[1].Summary, fragment) {
			t.Errorf("the verdict of bbb-feature does not mention %q: %s", fragment, features[1].Summary)
		}
	}
	for key := range validateResultKeys(t, stdout) {
		if !validKeys[key] {
			t.Errorf("the result of the invalid portfolio holds the key %q, which the valid one lacks", key)
		}
	}

	_, stderr := rerunCommand(t, 1, "validate")
	for _, fragment := range []string{"aaa-feature: VALID", "bbb-feature: INVALID: evidence ledger " + validateLedgerPath("bbb-feature")} {
		if !strings.Contains(stderr, fragment) {
			t.Errorf("the text output does not hold %q:\n%s", fragment, stderr)
		}
	}
}

// TestReviewApproveIgnoresLedger holds what this feature leaves alone:
// approval judges the documents of a phase and never opens the ledger.
func TestReviewApproveIgnoresLedger(t *testing.T) {
	root := rerunRoot(t)
	rerunCommand(t, 0, "repo", "init")
	rerunCommand(t, 0, "feature", "init", "demo")
	setRequirementsBody(t, root, gateRequirementsBody)
	rerunCommand(t, 0, "review", "open", "demo", "--phase", "requirements")
	writeValidateLedger(t, root, "demo", "<<<<<<< ours\n{}\n=======\n{}\n>>>>>>> theirs\n")

	rerunCommand(t, 0, "review", "approve", "demo", "--phase", "requirements")

	stdout, _ := rerunCommand(t, 0, "status", "demo", "--json")
	if !strings.Contains(stdout, `"status": "approved"`) {
		t.Errorf("the requirements are not approved after review approve: %s", stdout)
	}
}

func TestValidateHelpNamesLedger(t *testing.T) {
	stdout, _ := rerunCommand(t, 0, "validate", "--help")

	// The check, the file it reads and the typical way to get there.
	for _, statement := range []string{
		"fails when the evidence ledger of a feature",
		".walden/evidence/<feature>.json",
		"cannot be read",
		"conflict markers",
	} {
		if !strings.Contains(stdout, statement) {
			t.Errorf("the help does not state %q:\n%s", statement, stdout)
		}
	}
	// What the help already carried stays.
	for _, kept := range []string{"validate [<feature>] [--all] [--json]", "Check EARS, traceability, and freshness", "--all", "--json"} {
		if !strings.Contains(stdout, kept) {
			t.Errorf("the help lost %q:\n%s", kept, stdout)
		}
	}
}
