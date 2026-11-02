package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

// proofDiagnostic runs a task's proof against one scripted response and
// returns the failure text.
func proofDiagnostic(t *testing.T, task ExecutableTask, response testutil.Response) string {
	t.Helper()
	_, _, err := executeProof(context.Background(), testutil.NewFakeRunner(response), task)
	if err == nil {
		t.Fatal("expected the proof to fail")
	}
	return err.Error()
}

// timeoutDiagnostic runs a task's proof against a step that outlives its
// timeout after printing the given output.
func timeoutDiagnostic(t *testing.T, stdout string) string {
	t.Helper()
	runner := runnerFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		<-ctx.Done()
		return shell.Response{Stdout: stdout, ExitCode: -1}, nil
	})
	_, _, err := executeProof(context.Background(), runner, timeoutTask("10ms"))
	if err == nil {
		t.Fatal("expected a timeout failure")
	}
	return err.Error()
}

func TestProofFailureShowsOutputTail(t *testing.T) {
	const (
		command = `verification failed for task "1.1": command "command [\"go\",\"test\",\"-run\",\"TestX\",\"./pkg/\"]"`
		missing = command + ` output does not contain expected content "wanted"`
		exited  = command + ` exited with code 1 (expected 0)`
	)
	cases := []struct {
		name     string
		expect   string
		response testutil.Response
		want     string
	}{
		{
			name:     "output mismatch shows what the step printed",
			expect:   "wanted",
			response: testutil.Response{Stdout: "setup\nFATAL actual cause\n"},
			want:     missing + ": setup\nFATAL actual cause",
		},
		{
			name:     "output mismatch reads both streams",
			expect:   "wanted",
			response: testutil.Response{Stdout: "from stdout\n", Stderr: "from stderr\n"},
			want:     missing + ": from stdout\nfrom stderr",
		},
		{
			name:     "exit mismatch keeps a failing test behind a linker warning",
			response: testutil.Response{Stdout: "--- FAIL: TestX\n", Stderr: "ld: warning: duplicate libraries\n", ExitCode: 1},
			want:     exited + ": --- FAIL: TestX\nld: warning: duplicate libraries",
		},
		{
			name:     "exit mismatch on standard error keeps its text",
			response: testutil.Response{Stderr: "proof failed\n", ExitCode: 1},
			want:     exited + ": proof failed",
		},
		{
			name:     "exit mismatch on standard output keeps its text",
			response: testutil.Response{Stdout: "proof failed\n", ExitCode: 1},
			want:     exited + ": proof failed",
		},
		{
			name:     "silent exit mismatch gains nothing",
			response: testutil.Response{ExitCode: 1},
			want:     exited,
		},
		{
			name:     "silent output mismatch gains nothing",
			expect:   "wanted",
			response: testutil.Response{},
			want:     missing,
		},
		{
			name:     "whitespace counts as silence",
			expect:   "wanted",
			response: testutil.Response{Stdout: " \n\t\n"},
			want:     missing,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := proofDiagnostic(t, expectOutputTask(tc.expect), tc.response); got != tc.want {
				t.Fatalf("diagnostic mismatch\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}

	t.Run("silent timeout gains nothing", func(t *testing.T) {
		got := timeoutDiagnostic(t, "")
		if !strings.HasSuffix(got, "exceeded timeout 10ms") {
			t.Fatalf("diagnostic does not end with the timeout: %q", got)
		}
	})
}

func TestProofFailureShowsOutputBeforeTimeout(t *testing.T) {
	timeout := "200ms"
	// The printed text is assembled at run time: the diagnostic also names the
	// command, which must not satisfy the assertion on its own.
	proof := spec.VerificationSpec{Steps: []spec.VerificationStep{{
		Argv:    []string{"sh", "-c", "echo reached-the-$(echo slow)-step; sleep 30"},
		Timeout: &timeout,
	}}}
	task := ExecutableTask{ID: "1.1", Title: "Step", Verification: proof.Display(), Proof: proof}

	_, _, err := executeProof(context.Background(), shell.NewExecRunner(), task)
	if err == nil {
		t.Fatal("expected a timeout failure")
	}
	for _, want := range []string{"exceeded timeout 200ms", "reached-the-slow-step"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("diagnostic lacks %q: %q", want, err.Error())
		}
	}
}

func TestProofFailureExcerptIsBounded(t *testing.T) {
	const end = "END-OF-OUTPUT"
	huge := strings.Repeat("x", 2_000_000) + end

	failures := []struct {
		name string
		run  func(t *testing.T) string
	}{
		{"exit mismatch", func(t *testing.T) string {
			return proofDiagnostic(t, expectOutputTask(""), testutil.Response{Stderr: huge, ExitCode: 1})
		}},
		{"output mismatch", func(t *testing.T) string {
			return proofDiagnostic(t, expectOutputTask("wanted"), testutil.Response{Stdout: huge})
		}},
		{"timeout", func(t *testing.T) string {
			return timeoutDiagnostic(t, huge)
		}},
	}
	for _, failure := range failures {
		t.Run("2 MB under 5 KB on "+failure.name, func(t *testing.T) {
			got := failure.run(t)
			if len(got) >= 5*1024 {
				t.Fatalf("diagnostic holds %d bytes", len(got))
			}
			if !strings.HasSuffix(got, end) {
				t.Fatalf("diagnostic lost the output's final bytes: …%q", got[max(0, len(got)-40):])
			}
		})
	}

	excerptOf := func(t *testing.T, output string) string {
		t.Helper()
		got := proofDiagnostic(t, expectOutputTask("wanted"), testutil.Response{Stdout: output})
		_, excerpt, found := strings.Cut(got, `expected content "wanted": `)
		if !found {
			t.Fatalf("diagnostic carries no excerpt: %q", got)
		}
		return excerpt
	}

	t.Run("a longer output states what it omits", func(t *testing.T) {
		want := "[904 bytes omitted] " + strings.Repeat("x", 4096)
		if got := excerptOf(t, strings.Repeat("x", 5000)); got != want {
			t.Fatalf("excerpt holds %d bytes and starts with %q", len(got), got[:30])
		}
	})
	t.Run("an output at the limit omits nothing", func(t *testing.T) {
		want := strings.Repeat("x", 4096)
		if got := excerptOf(t, want); got != want {
			t.Fatalf("excerpt holds %d bytes and starts with %q", len(got), got[:30])
		}
	})
	t.Run("a cut inside a character starts at the next one", func(t *testing.T) {
		// 6000 bytes of three-byte characters: the limit falls at byte 1904,
		// inside the character that starts at 1902.
		got := excerptOf(t, strings.Repeat("€", 2000))
		if want := "[1905 bytes omitted] " + strings.Repeat("€", 1365); got != want {
			t.Fatalf("excerpt holds %d bytes and starts with %q", len(got), got[:30])
		}
	})
	t.Run("the longest character is skipped whole", func(t *testing.T) {
		// 6001 bytes: the limit falls at byte 1905, on the first of the three
		// continuation bytes of the four-byte character that starts at 1904.
		got := excerptOf(t, strings.Repeat("😀", 1500)+"x")
		want := "[1908 bytes omitted] " + strings.Repeat("😀", 1023) + "x"
		if got != want {
			t.Fatalf("excerpt holds %d bytes and starts with %q", len(got), got[:30])
		}
		if !utf8.ValidString(got) {
			t.Fatal("excerpt is not valid UTF-8")
		}
	})
	t.Run("output with no character boundary keeps an excerpt", func(t *testing.T) {
		got := excerptOf(t, strings.Repeat("\x80", 5000))
		if want := "[907 bytes omitted] " + strings.Repeat("\x80", 4093); got != want {
			t.Fatalf("excerpt holds %d bytes and starts with %q", len(got), got[:30])
		}
	})
	t.Run("the same output yields the same text", func(t *testing.T) {
		output := strings.Repeat("deterministic ", 1000)
		if first, second := excerptOf(t, output), excerptOf(t, output); first != second {
			t.Fatal("two runs over one output produced different diagnostics")
		}
	})
}

func TestProofFailureExcerptLeavesAssessmentUntouched(t *testing.T) {
	filler := strings.Repeat("x", 8192)

	t.Run("expected content before the excerpt still passes", func(t *testing.T) {
		output := "wanted\n" + filler
		runner := testutil.NewFakeRunner(testutil.Response{Stdout: output})
		results, _, err := executeProof(context.Background(), runner, expectOutputTask("wanted"))
		if err != nil {
			t.Fatalf("proof failed: %v", err)
		}
		if results[0].OutputDigest != evidence.DigestOutput(output) {
			t.Fatal("digest does not cover the complete output")
		}
	})

	t.Run("a failed step digests its complete output", func(t *testing.T) {
		output := filler + "\ntail-of-output\n"
		runner := testutil.NewFakeRunner(testutil.Response{Stdout: output})
		results, _, err := executeProof(context.Background(), runner, expectOutputTask("wanted"))
		if err == nil || !strings.Contains(err.Error(), "tail-of-output") {
			t.Fatalf("expected a failure showing the tail, got %v", err)
		}
		if results[0].OutputDigest != evidence.DigestOutput(output) {
			t.Fatal("digest does not cover the complete output")
		}
	})
}

func TestVerifyFailureExcerptStaysOutOfLedger(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	completeBoth(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob bbb\tmain.go\n"))

	const sentinel = "EXCERPT-SENTINEL-ONLY-IN-OUTPUT"
	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stdout: sentinel + "\n", Stderr: "noise\n", ExitCode: 1},
	)
	result, err := Verify(context.Background(), root, "todo-app-demo", false, false, runner)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if len(result.Outcomes) != 2 || !strings.Contains(result.Outcomes[1].Failure, sentinel) {
		t.Fatalf("diagnostic does not report the output: %+v", result.Outcomes)
	}

	data, err := os.ReadFile(filepath.Join(root, ".walden", "evidence", "todo-app-demo.json"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if strings.Contains(string(data), sentinel) {
		t.Fatal("the excerpt reached the ledger")
	}
}
