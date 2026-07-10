package selfupdate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
)

func seedSwapAftermath(t *testing.T) (executable, backup string) {
	t.Helper()
	dir := t.TempDir()
	executable = filepath.Join(dir, "walden")
	backup = filepath.Join(dir, ".walden-backup-test")
	if err := os.WriteFile(executable, []byte("NEW-BINARY"), 0o755); err != nil {
		t.Fatalf("seed executable: %v", err)
	}
	if err := os.WriteFile(backup, []byte("OLD-BINARY"), 0o755); err != nil {
		t.Fatalf("seed backup: %v", err)
	}
	return executable, backup
}

func TestSmokePassDeletesBackup(t *testing.T) {
	executable, backup := seedSwapAftermath(t)
	runner := &fakeRunner{respond: func(string, []string) (shell.Response, error) {
		return shell.Response{Stdout: "walden v0.7.0 (schema v0beta1)", ExitCode: 0}, nil
	}}

	if err := smokeTestAndFinalize(context.Background(), runner, executable, backup, "v0.7.0"); err != nil {
		t.Fatalf("smokeTestAndFinalize returned error: %v", err)
	}

	if len(runner.calls) != 1 || runner.calls[0][0] != executable || runner.calls[0][1] != "version" {
		t.Fatalf("smoke test invoked %v, want [%s version]", runner.calls, executable)
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatal("backup survived a passing smoke test")
	}
	content, _ := os.ReadFile(executable)
	if string(content) != "NEW-BINARY" {
		t.Fatalf("executable content = %q, want NEW-BINARY", content)
	}
}

func TestSmokeFailureRestoresPreviousBinary(t *testing.T) {
	cases := []struct {
		name    string
		respond func(string, []string) (shell.Response, error)
	}{
		{
			name: "wrong version reported",
			respond: func(string, []string) (shell.Response, error) {
				return shell.Response{Stdout: "walden v0.5.0 (schema v0beta1)", ExitCode: 0}, nil
			},
		},
		{
			name: "non-zero exit",
			respond: func(string, []string) (shell.Response, error) {
				return shell.Response{Stderr: "segfault", ExitCode: 1}, nil
			},
		},
		{
			name: "launch failure",
			respond: func(string, []string) (shell.Response, error) {
				return shell.Response{}, errors.New("exec format error")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			executable, backup := seedSwapAftermath(t)
			runner := &fakeRunner{respond: tc.respond}

			err := smokeTestAndFinalize(context.Background(), runner, executable, backup, "v0.7.0")
			if err == nil {
				t.Fatal("smokeTestAndFinalize accepted a failing smoke test")
			}
			if !strings.Contains(err.Error(), "restored") {
				t.Fatalf("error %q does not state the previous binary was restored", err)
			}

			content, readErr := os.ReadFile(executable)
			if readErr != nil {
				t.Fatalf("read executable: %v", readErr)
			}
			if string(content) != "OLD-BINARY" {
				t.Fatalf("executable content = %q, want restored OLD-BINARY", content)
			}
			if _, statErr := os.Stat(backup); !os.IsNotExist(statErr) {
				t.Fatal("backup still present after restore")
			}
		})
	}
}
