package release

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/spec"
)

// fourStateTasks holds four leaves that one repository can hold in four
// states at once: the delivery leaf 1.1 and three regression leaves.
func fourStateTasks() string {
	leaf := func(id, title, requirements, covers string) string {
		return "  - [ ] " + id + " " + title + "\n    - Requirements: " + requirements + "\n    - Design: Overview\n    - Verification:\n" +
			"      - command: [\"sh\", \"-c\", \"true\"]\n        covers: [" + covers + "]\n"
	}
	return draftTasksHeader + "# Implementation Plan\n\n- [ ] 1. Markers\n" +
		"  - [ ] 1.1 Publish the notes\n    - Requirements: `R1.AC1`\n    - Design: Overview\n    - Proof: delivery\n    - Verification:\n" +
		"      - command: [\"grep\", \"-q\", \"released\", \"notes.txt\"]\n        covers: [\"R1.AC1\"]\n" +
		leaf("1.2", "Keep the marker", "`R1.AC2`", `"R1.AC2"`) +
		leaf("1.3", "Keep it again", "`R1.AC3`", `"R1.AC3"`) +
		leaf("1.4", "Left for later", "`R1.AC2`, `R1.AC3`", `"R1.AC2", "R1.AC3"`)
}

// fourStateRepo is a repository in which leaf 1.1 is delivered, 1.2 was
// verified and then made stale by a committed change, 1.3 was verified after
// that change, and 1.4 is pending.
func fourStateRepo(t *testing.T) string {
	t.Helper()
	root := isolatedGateRepoWith(t, fourStateTasks())
	completeGateTask(t, root, "1.1")
	completeGateTask(t, root, "1.2")
	write(t, root, "main.go", "package main // moved on\n")
	commitAll(t, root, "the code moves on")
	completeGateTask(t, root, "1.3")
	return root
}

func readingStates(reading FeatureEvidence) map[string]string {
	states := map[string]string{}
	for _, entry := range reading.Evidence {
		states[entry.TaskID] = entry.State
	}
	return states
}

// TestReadFeatureEvidenceStates: the reading returns, for each leaf of one
// repository that holds four states at once, the state stated here by hand,
// and it is the state the gate reports on the same repository.
func TestReadFeatureEvidenceStates(t *testing.T) {
	root := fourStateRepo(t)

	reading, err := ReadFeatureEvidence(context.Background(), root, deliveryGateFeature)
	if err != nil {
		t.Fatalf("ReadFeatureEvidence: %v", err)
	}
	want := map[string]string{
		"1.1": evidence.StateDelivered,
		"1.2": evidence.StateStaleCode,
		"1.3": evidence.StateVerified,
		"1.4": evidence.StatePending,
	}
	if got := readingStates(reading); !reflect.DeepEqual(got, want) {
		t.Errorf("states of the reading = %v, want %v", got, want)
	}

	_, cert := gateCertification(t, root, Options{})
	if got := gateStates(cert); !reflect.DeepEqual(got, want) {
		t.Errorf("states of the gate = %v, want %v", got, want)
	}

	// The reading carries what a join needs: the feature and its leaves.
	if reading.Feature.Name != deliveryGateFeature || !reading.Feature.Requirements.Exists {
		t.Errorf("the reading holds the feature %q with requirements read = %t", reading.Feature.Name, reading.Feature.Requirements.Exists)
	}
	leaves := []string{}
	for _, leaf := range reading.Tree.LeafTasks() {
		leaves = append(leaves, leaf.ID)
	}
	if wantLeaves := []string{"1.1", "1.2", "1.3", "1.4"}; !reflect.DeepEqual(leaves, wantLeaves) {
		t.Errorf("leaves of the reading = %v, want %v", leaves, wantLeaves)
	}
}

// TestReadFeatureEvidenceFailures: each input the reading cannot use fails
// it with a message that names the feature or the file and the cause, and no
// evidence is derived.
func TestReadFeatureEvidenceFailures(t *testing.T) {
	const specDir = ".walden/specs/" + deliveryGateFeature + "/"
	const ledger = ".walden/evidence/" + deliveryGateFeature + ".json"

	cases := []struct {
		name    string
		feature string
		// arrange changes the repository and returns the whole message, or
		// a prefix and a fragment of it when the cause comes from a library.
		arrange  func(t *testing.T, root string) (message, prefix, fragment string)
		notFound bool
	}{
		{
			name:    "a feature without a directory",
			feature: "nope",
			arrange: func(t *testing.T, root string) (string, string, string) {
				return `feature "nope" does not exist`, "", ""
			},
			notFound: true,
		},
		{
			name: "requirements.md is missing",
			arrange: func(t *testing.T, root string) (string, string, string) {
				remove(t, root, specDir+"requirements.md")
				return specDir + "requirements.md does not exist", "", ""
			},
		},
		{
			name: "tasks.md is missing",
			arrange: func(t *testing.T, root string) (string, string, string) {
				remove(t, root, specDir+"tasks.md")
				return specDir + "tasks.md does not exist", "", ""
			},
		},
		{
			name: "a document without frontmatter",
			arrange: func(t *testing.T, root string) (string, string, string) {
				write(t, root, specDir+"requirements.md", "# Requirements Document\n")
				return specDir + "requirements.md: missing YAML frontmatter", "", ""
			},
		},
		{
			name: "a plan the parser rejects",
			arrange: func(t *testing.T, root string) (string, string, string) {
				plan := strings.Replace(deliveryGateTasks(), "        covers: [\"R1.AC1\", \"R1.AC3\"]", "        coverz: [\"R1.AC1\"]", 1)
				write(t, root, specDir+"tasks.md", plan)
				feature, err := spec.LoadFeature(root, deliveryGateFeature)
				if err != nil {
					t.Fatalf("load the fixture: %v", err)
				}
				_, parseErr := spec.ParseTaskTree(feature.Tasks)
				if parseErr == nil || !strings.Contains(parseErr.Error(), "line ") || !strings.Contains(parseErr.Error(), "coverz") {
					t.Fatalf("the parser does not reject the plan on its line: %v", parseErr)
				}
				return specDir + "tasks.md: " + parseErr.Error(), "", ""
			},
		},
		{
			name: "a ledger that is not JSON",
			arrange: func(t *testing.T, root string) (string, string, string) {
				write(t, root, ledger, "<<<<<<< HEAD\n")
				return "", ledger + ": parse: ", "retain the ledger for diagnosis"
			},
		},
		{
			name: "a plan that cannot be read",
			arrange: func(t *testing.T, root string) (string, string, string) {
				previous := readInputFile
				t.Cleanup(func() { readInputFile = previous })
				readInputFile = func(path string) ([]byte, error) {
					if strings.HasSuffix(filepath.ToSlash(path), specDir+"tasks.md") {
						return nil, errors.New("input/output error")
					}
					return previous(path)
				}
				return specDir + "tasks.md: input/output error", "", ""
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := isolatedGateRepo(t)
			message, prefix, fragment := tc.arrange(t, root)
			feature := tc.feature
			if feature == "" {
				feature = deliveryGateFeature
			}

			reading, err := ReadFeatureEvidence(context.Background(), root, feature)
			if err == nil {
				t.Fatalf("the reading succeeded: %+v", readingStates(reading))
			}
			got := err.Error()
			if message != "" && got != message {
				t.Errorf("message = %q, want %q", got, message)
			}
			if message == "" && (!strings.HasPrefix(got, prefix) || !strings.Contains(got, fragment)) {
				t.Errorf("message = %q, want it to open with %q and hold %q", got, prefix, fragment)
			}
			if errors.Is(err, spec.ErrFeatureNotFound) != tc.notFound {
				t.Errorf("the error is the one of a feature that does not exist = %t, want %t", !tc.notFound, tc.notFound)
			}
			if len(reading.Evidence) != 0 || len(reading.Tree.LeafTasks()) != 0 {
				t.Errorf("a failed reading holds evidence or leaves: %+v", reading)
			}
		})
	}
}

func remove(t *testing.T, root, name string) {
	t.Helper()
	if err := os.Remove(filepath.Join(root, name)); err != nil {
		t.Fatalf("remove %s: %v", name, err)
	}
}

// readingGitSubcommands is every git subcommand a reading may start. Each
// reads the repository: none writes an object, a ref or the index.
var readingGitSubcommands = []string{"rev-parse", "status", "ls-tree", "hash-object", "log", "cat-file", "symbolic-ref", "show-ref"}

// TestReadFeatureEvidenceRunsReadOnlyGit: every git invocation of a reading
// is one of the stated subcommands that read, carries the options of the
// gate and never asks hash-object to write.
func TestReadFeatureEvidenceRunsReadOnlyGit(t *testing.T) {
	root := isolatedGateRepo(t)
	completeGateTask(t, root, "1.1")
	completeGateTask(t, root, "1.2")
	commitAll(t, root, "evidence")

	record := func() *recordingGateGit {
		t.Helper()
		return recordGateGit(t, func() {
			if _, err := ReadFeatureEvidence(context.Background(), root, deliveryGateFeature); err != nil {
				t.Fatalf("ReadFeatureEvidence: %v", err)
			}
		})
	}
	requireReads := func(recorder *recordingGateGit) []string {
		t.Helper()
		subcommands := recorder.subcommands(t)
		if len(subcommands) == 0 {
			t.Fatal("the reading started no git invocation: the recorder is not on its path")
		}
		for index, subcommand := range subcommands {
			if !slices.Contains(readingGitSubcommands, subcommand) {
				t.Errorf("git %s is not a stated subcommand that reads: %q", subcommand, recorder.argv[index])
			}
			if subcommand == "hash-object" && slices.Contains(recorder.argv[index], "-w") {
				t.Errorf("hash-object is asked to write an object: %q", recorder.argv[index])
			}
		}
		return subcommands
	}

	// A dirty tree: the changed and the new file are hashed.
	write(t, root, "main.go", "package main // changed\n")
	write(t, root, "new.go", "package main\n")
	if subcommands := requireReads(record()); !slices.Contains(subcommands, "hash-object") {
		t.Errorf("a dirty tree was read without hash-object: %v", subcommands)
	}

	// A record rewritten as one written before task fingerprints: the
	// history of the plan is read.
	ledger, err := evidence.Load(root, deliveryGateFeature)
	if err != nil {
		t.Fatalf("load the ledger: %v", err)
	}
	legacy := ledger.Tasks["1.2"]
	legacy.TaskFingerprintScheme = ""
	legacy.TasksFingerprint = "sha256:" + strings.Repeat("0", 64)
	ledger.Tasks["1.2"] = legacy
	if err := evidence.Save(root, ledger); err != nil {
		t.Fatalf("save the ledger: %v", err)
	}
	if subcommands := requireReads(record()); !slices.Contains(subcommands, "log") {
		t.Errorf("the history of the plan was not read: %v", subcommands)
	}
}
