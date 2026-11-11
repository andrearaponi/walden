package app

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// preLifecycleBinaryEnv names the released binary the recording is made with.
const preLifecycleBinaryEnv = "WALDEN_PRE_LIFECYCLE_BINARY"

// TestRecordPreLifecycle regenerates testdata/pre_lifecycle/recorded from the
// portfolio beside it. It runs only when WALDEN_PRE_LIFECYCLE_BINARY names a
// walden binary, and refuses any release but preLifecycleVersion: the
// recording is a reference only while it comes from that release.
//
// Two repositories are built from the same portfolio. In the first every leaf
// is completed, so each has a recorded fingerprint. The second is the
// scenario: its plan `states` holds one task for each evidence state, reached
// through the commands of the release and through two edits of the ledger
// that no command makes, the removal of the execution facts of 1.5 and of the
// record of 1.6.
func TestRecordPreLifecycle(t *testing.T) {
	binary := os.Getenv(preLifecycleBinaryEnv)
	if binary == "" {
		t.Skipf("set %s to a walden %s binary to regenerate the recording", preLifecycleBinaryEnv, preLifecycleVersion)
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatalf("resolve the binary: %v", err)
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("the recording needs git: %v", err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	// run executes one command in dir and returns its standard output. The
	// exit status must be the expected one: a failure is part of the scenario
	// only where the scenario says so.
	run := func(dir string, wantExit int, name string, args ...string) string {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		exit := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			exit = exitErr.ExitCode()
		} else if err != nil {
			t.Fatalf("%s %v: %v", name, args, err)
		}
		if exit != wantExit {
			t.Fatalf("%s %v exited %d, want %d\n%s\n%s", name, args, exit, wantExit, stdout.String(), stderr.String())
		}
		return stdout.String()
	}
	walden := func(dir string, wantExit int, args ...string) string {
		t.Helper()
		return run(dir, wantExit, binary, args...)
	}
	git := func(dir string, args ...string) {
		t.Helper()
		run(dir, 0, "git", args...)
	}
	commit := func(dir, message string) {
		t.Helper()
		git(dir, "add", "-A")
		git(dir, "commit", "-qm", message)
	}

	version := walden(".", 0, "version")
	if !strings.Contains(version, "walden "+preLifecycleVersion+" ") {
		t.Fatalf("%s is %q: the recording must come from walden %s", binary, strings.TrimSpace(version), preLifecycleVersion)
	}

	// approvedPortfolio builds a repository from the portfolio and approves
	// the three documents of each feature through the review gates.
	approvedPortfolio := func(name string) string {
		t.Helper()
		root := filepath.Join(t.TempDir(), name)
		for source, target := range map[string]string{"code": "code", "walden": ".walden"} {
			if err := os.CopyFS(filepath.Join(root, target), os.DirFS(preLifecyclePath("portfolio", source))); err != nil {
				t.Fatalf("copy the portfolio: %v", err)
			}
		}
		git(root, "init", "-q", "-b", "main")
		git(root, "config", "user.email", "fixture@walden.test")
		git(root, "config", "user.name", "Fixture")
		commit(root, "portfolio")
		for _, feature := range preLifecycleFeatures {
			for _, phase := range []string{"requirements", "design", "tasks"} {
				walden(root, 0, "review", "open", feature, "--phase", phase)
				walden(root, 0, "review", "approve", feature, "--phase", phase)
			}
		}
		commit(root, "approved")
		return root
	}

	// Every leaf completed: the ledgers hold the fingerprint of each one.
	complete := approvedPortfolio("complete")
	for _, feature := range preLifecycleFeatures {
		walden(complete, 0, "task", "complete-all", feature)
	}

	// The scenario. Six leaves of `states` are completed on the first tree,
	// then the code changes.
	scenario := approvedPortfolio("scenario")
	for _, id := range []string{"1.1", "1.2", "1.3", "1.4", "1.5", "1.6"} {
		walden(scenario, 0, "task", "complete", "states", id)
	}
	commit(scenario, "six leaves completed")
	appendLine(t, filepath.Join(scenario, "code", "app.txt"), "second line\n")
	commit(scenario, "the code changes")

	// 1.1 and 1.5 are proven again on the second tree, and `steady` is
	// completed there: they are bound to the code as it is at the end. 1.2
	// and 1.3 stay bound to the first tree.
	walden(scenario, 0, "verify", "states", "--task", "1.1")
	walden(scenario, 0, "verify", "states", "--task", "1.5")
	walden(scenario, 0, "task", "complete-all", "steady")

	// 1.4 fails once the flag exists. The flag is under .walden/, outside the
	// code identity.
	if err := os.WriteFile(filepath.Join(scenario, ".walden", "fail-flag"), nil, 0o644); err != nil {
		t.Fatalf("write the flag: %v", err)
	}
	walden(scenario, 1, "verify", "states", "--task", "1.4")

	// The proof of 1.3 changes and the plan is approved again.
	plan := filepath.Join(scenario, ".walden", "specs", "states", "tasks.md")
	replaceOnce(t, plan, "test 3 -eq 3", "test 3 -ge 3")
	walden(scenario, 0, "reconcile", "states")
	walden(scenario, 0, "review", "open", "states", "--phase", "tasks")
	walden(scenario, 0, "review", "approve", "states", "--phase", "tasks")

	// No command of the release removes the execution facts of a record or a
	// record of a completed task: both are edits of the ledger, made here.
	ledgerPath := filepath.Join(scenario, ".walden", "evidence", "states.json")
	var ledger map[string]any
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	if err := json.Unmarshal(data, &ledger); err != nil {
		t.Fatalf("decode the ledger: %v", err)
	}
	tasks := ledger["tasks"].(map[string]any)
	delete(tasks["1.5"].(map[string]any), "execution")
	delete(tasks, "1.6")
	edited, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		t.Fatalf("encode the ledger: %v", err)
	}
	if err := os.WriteFile(ledgerPath, append(edited, '\n'), 0o644); err != nil {
		t.Fatalf("write the ledger: %v", err)
	}
	commit(scenario, "scenario")

	// The read commands run on the committed scenario. A blocked verdict
	// exits 1; every other command exits 0.
	results := map[string]string{}
	for _, command := range preLifecycleCommands {
		wantExit := 0
		if command.name == "release-check" || command.name == "release-check-strict" {
			wantExit = 1
		}
		results[command.name] = walden(scenario, wantExit, append(append([]string{}, command.args...), "--json")...)
	}
	if status := run(scenario, 0, "git", "status", "--porcelain"); status != "" {
		t.Fatalf("a read command changed the scenario:\n%s", status)
	}

	// Replace the recording.
	recorded := preLifecyclePath("recorded")
	if err := os.RemoveAll(recorded); err != nil {
		t.Fatalf("remove the earlier recording: %v", err)
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	copyTree := func(source, target string) {
		t.Helper()
		if err := os.CopyFS(target, os.DirFS(source)); err != nil {
			t.Fatalf("copy %s: %v", source, err)
		}
	}
	write(filepath.Join(recorded, "version.txt"), version)
	for _, part := range []string{"specs", "evidence"} {
		copyTree(filepath.Join(complete, ".walden", part), filepath.Join(recorded, "complete", "walden", part))
	}
	copyTree(filepath.Join(scenario, "code"), filepath.Join(recorded, "scenario", "code"))
	copyTree(filepath.Join(scenario, ".walden"), filepath.Join(recorded, "scenario", "walden"))
	for name, result := range results {
		write(filepath.Join(recorded, "results", name+".json"), result)
	}
}

func appendLine(t *testing.T, path, line string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()
	if _, err := file.WriteString(line); err != nil {
		t.Fatalf("append to %s: %v", path, err)
	}
}

func replaceOnce(t *testing.T, path, old, replacement string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Count(string(data), old) != 1 {
		t.Fatalf("%s holds %q %d time(s), want one", path, old, strings.Count(string(data), old))
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), old, replacement, 1)), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
