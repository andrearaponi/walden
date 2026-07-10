package selfupdate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedExecutable(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "walden")
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("seed executable: %v", err)
	}
	return path
}

func TestSwapReplacesContentAndMode(t *testing.T) {
	dir := t.TempDir()
	executable := seedExecutable(t, dir, "OLD-BINARY")

	staged := filepath.Join(dir, ".walden-update-test")
	if err := os.WriteFile(staged, []byte("NEW-BINARY"), 0o600); err != nil {
		t.Fatalf("seed staged file: %v", err)
	}

	backup, err := swapExecutable(staged, executable)
	if err != nil {
		t.Fatalf("swapExecutable returned error: %v", err)
	}

	content, err := os.ReadFile(executable)
	if err != nil {
		t.Fatalf("read swapped executable: %v", err)
	}
	if string(content) != "NEW-BINARY" {
		t.Fatalf("executable content = %q, want NEW-BINARY", content)
	}

	info, err := os.Stat(executable)
	if err != nil {
		t.Fatalf("stat swapped executable: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("executable mode = %v, want 0755", info.Mode().Perm())
	}

	backupContent, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backupContent) != "OLD-BINARY" {
		t.Fatalf("backup content = %q, want OLD-BINARY", backupContent)
	}
}

func TestSwapResolvesSymlinkedExecutable(t *testing.T) {
	realDir := t.TempDir()
	linkDir := t.TempDir()
	real := seedExecutable(t, realDir, "OLD-BINARY")

	link := filepath.Join(linkDir, "walden")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	resolved, err := resolveExecutable(link)
	if err != nil {
		t.Fatalf("resolveExecutable returned error: %v", err)
	}
	if evalReal, _ := filepath.EvalSymlinks(real); resolved != evalReal {
		t.Fatalf("resolveExecutable = %q, want %q", resolved, evalReal)
	}

	staged := filepath.Join(filepath.Dir(resolved), ".walden-update-test")
	if err := os.WriteFile(staged, []byte("NEW-BINARY"), 0o600); err != nil {
		t.Fatalf("seed staged file: %v", err)
	}
	if _, err := swapExecutable(staged, resolved); err != nil {
		t.Fatalf("swapExecutable returned error: %v", err)
	}

	throughLink, err := os.ReadFile(link)
	if err != nil {
		t.Fatalf("read through symlink: %v", err)
	}
	if string(throughLink) != "NEW-BINARY" {
		t.Fatalf("content through symlink = %q, want NEW-BINARY", throughLink)
	}
}

func TestSwapUnwritableDirectoryAbortsUntouched(t *testing.T) {
	dir := t.TempDir()
	executable := seedExecutable(t, dir, "OLD-BINARY")

	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("make directory read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err := probeStaging(executable)
	if err == nil {
		t.Fatal("probeStaging succeeded in a read-only directory")
	}
	if !strings.Contains(err.Error(), dir) {
		t.Fatalf("error %q does not name the directory", err)
	}

	content, readErr := os.ReadFile(executable)
	if readErr != nil {
		t.Fatalf("read executable: %v", readErr)
	}
	if string(content) != "OLD-BINARY" {
		t.Fatalf("executable modified by a failed probe: %q", content)
	}
}

func TestSwapProbeStagesNextToExecutable(t *testing.T) {
	dir := t.TempDir()
	executable := seedExecutable(t, dir, "OLD-BINARY")

	staged, err := probeStaging(executable)
	if err != nil {
		t.Fatalf("probeStaging returned error: %v", err)
	}
	if filepath.Dir(staged) != dir {
		t.Fatalf("staging dir = %q, want %q", filepath.Dir(staged), dir)
	}
	if !strings.HasPrefix(filepath.Base(staged), ".walden-update-") {
		t.Fatalf("staging name = %q, want .walden-update-* prefix", filepath.Base(staged))
	}
	if _, err := os.Stat(staged); err != nil {
		t.Fatalf("staging file not created: %v", err)
	}
}
