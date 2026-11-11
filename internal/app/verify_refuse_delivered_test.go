package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
	"github.com/andrearaponi/walden/internal/testutil"
)

// deliveredSelectionRepo returns a repository holding the delivered task 1.1
// and the verified regression task 1.2, on a root this process has not
// profiled, and the path of a file that only a declared environment probe
// creates.
func deliveredSelectionRepo(t *testing.T) (root, head, sentinel string) {
	t.Helper()
	first, head := deliveredStatusRepo(t)

	// A process captures the profile once for each root, so the completions
	// above would keep the probe from ever running here. Moving the
	// repository hands the command under test a root with no capture.
	root = filepath.Join(t.TempDir(), "repo")
	if err := os.Rename(first, root); err != nil {
		t.Fatalf("move the repository: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("enter the moved repository: %v", err)
	}
	sentinel = filepath.Join(t.TempDir(), "probe-ran")
	probes := fmt.Sprintf("- sentinel: [\"touch\", %q]\n", sentinel)
	if err := os.WriteFile(filepath.Join(root, ".walden", "environment.md"), []byte(probes), 0o644); err != nil {
		t.Fatalf("declare the probe: %v", err)
	}
	return root, head, sentinel
}

// TestVerifyTaskRefusesDeliveredCommand: `walden verify --task` on a
// delivered task exits non-zero and reaches no probe, no proof and no ledger
// write, with and without --check, in text and in JSON.
func TestVerifyTaskRefusesDeliveredCommand(t *testing.T) {
	for _, args := range [][]string{
		{"verify", verifyTaskFeature, "--task", "1.1"},
		{"verify", verifyTaskFeature, "--task", "1.1", "--check"},
		{"verify", verifyTaskFeature, "--task", "1.1", "--json"},
		{"verify", verifyTaskFeature, "--check", "--json", "--task", "1.1"},
	} {
		root, head, sentinel := deliveredSelectionRepo(t)
		runner := overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}))
		before := readLedger(t, root)

		var stdout, stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		if code == 0 {
			t.Errorf("%v: exit 0, want a refusal: %s", args, stdout.String())
		}
		message := stderr.String()
		if strings.Contains(strings.Join(args, " "), "--json") {
			var envelope output.Envelope
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("%v: no JSON envelope: %v (%s)", args, err, stdout.String())
			}
			if envelope.OK || envelope.Result.ExitCode == 0 {
				t.Errorf("%v: the envelope reports success: %+v", args, envelope)
			}
			message = envelope.Result.Summary
		}
		for _, want := range []string{
			`task "1.1" is delivered`, "attested at commit " + head,
			"only after its declaration or its proof changes in the approved plan, or after a statement it names changes",
		} {
			if !strings.Contains(message, want) {
				t.Errorf("%v: the refusal %q does not hold %q", args, message, want)
			}
		}
		if calls := runner.Calls(); len(calls) != 0 {
			t.Errorf("%v: proofs ran: %v", args, calls)
		}
		if !bytes.Equal(before, readLedger(t, root)) {
			t.Errorf("%v: the ledger changed", args)
		}
		if _, err := os.Stat(sentinel); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%v: the environment probe ran (stat: %v)", args, err)
		}
	}

	// Control: the same repository, with the regression task named, reaches
	// the probe, the proof and the ledger, so the checks above can fail.
	root, _, sentinel := deliveredSelectionRepo(t)
	runner := overrideCommandRunner(t, testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}))
	before := readLedger(t, root)
	integrityMustCLI(t, "verify", verifyTaskFeature, "--task", "1.2")
	if calls := runner.Calls(); len(calls) != 1 {
		t.Errorf("control: proofs run = %v, want one", calls)
	}
	if bytes.Equal(before, readLedger(t, root)) {
		t.Error("control: the ledger did not change")
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Errorf("control: the environment probe did not run: %v", err)
	}
}
