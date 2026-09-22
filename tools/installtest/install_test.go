//go:build darwin || linux

package installtest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

type fixture struct {
	t                       *testing.T
	root, home, bin, script string
	events, payload, sums   string
	env                     map[string]string
}

func put(t *testing.T, path, text string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), mode); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func newFixture(t *testing.T, downloader string) *fixture {
	t.Helper()
	_, current, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(current), "../.."))
	root := t.TempDir()
	f := &fixture{t: t, root: root, home: filepath.Join(root, "home"), bin: filepath.Join(root, "commands"), script: filepath.Join(root, "install.sh"), events: filepath.Join(root, "events"), payload: filepath.Join(root, "payload"), sums: filepath.Join(root, "checksums")}
	put(t, f.script, read(t, filepath.Join(repo, "install.sh")), 0o700)
	for _, dir := range []string{f.home, f.bin, filepath.Join(root, "tmp")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	// Only allow known local utilities onto the installer PATH. No real
	// downloader can be reached when the selected fake downloader fails.
	for _, command := range []string{"tr", "mktemp", "cp", "chmod", "mv", "rm", "mkdir", "awk", "sed", "head", "cat", "sha256sum", "shasum"} {
		path, err := exec.LookPath(command)
		if err != nil {
			continue
		}
		if err := os.Symlink(path, filepath.Join(f.bin, command)); err != nil {
			t.Fatal(err)
		}
	}
	put(t, filepath.Join(f.bin, "uname"), `#!/bin/sh
printf 'uname %s\n' "$*" >> "$EVENTS"
case "$1" in -s) printf '%s\n' "${TEST_OS:-Darwin}" ;; -m) printf '%s\n' "${TEST_ARCH:-arm64}" ;; *) exit 9 ;; esac
`, 0o700)
	put(t, f.payload, `#!/bin/sh
printf 'payload %s\n' "$*" >> "$EVENTS"
case "$1" in
 version) test "${TEST_VERSION_FAIL:-0}" = 0 || exit 19; printf 'walden v0.10.2 (fixture)\n' ;;
 *) printf 'unknown command: %s\n' "$*" >&2; exit 1 ;;
esac
`, 0o700)
	digest := sha256.Sum256([]byte(read(t, f.payload)))
	put(t, f.sums, fmt.Sprintf("%x  walden-v0.10.2-darwin-arm64\n%x  walden-v0.10.2-linux-amd64\n", digest, digest), 0o600)
	put(t, filepath.Join(f.bin, downloader), `#!/bin/sh
printf 'fetch %s\n' "$*" >> "$EVENTS"
case "$1" in
 -fsSLI|--max-redirect=0)
  test "${TEST_REDIRECT_FAIL:-0}" = 0 || exit 22
  if test "$1" = -fsSLI; then printf 'https://github.com/andrearaponi/walden/releases/tag/v0.10.2'
  else printf '  Location: https://github.com/andrearaponi/walden/releases/tag/v0.10.2\n' >&2; fi
  exit 0 ;;
esac
dest="$3"; url="$4"
case "$url" in
 https://github.com/andrearaponi/walden/releases/download/*/checksums.txt)
  test "${TEST_MISSING_SUMS:-0}" = 0 || exit 22
  cp "$SUMS" "$dest" ;;
 https://github.com/andrearaponi/walden/releases/download/*/walden-*)
  test "${TEST_DOWNLOAD_FAIL:-0}" = 0 || exit 22
  cp "$PAYLOAD" "$dest" ;;
 *) printf 'unexpected URL: %s\n' "$url" >&2; exit 24 ;;
esac
`, 0o700)
	f.env = map[string]string{"HOME": f.home, "PATH": f.bin, "TMPDIR": filepath.Join(root, "tmp"), "EVENTS": f.events, "PAYLOAD": f.payload, "SUMS": f.sums, "LC_ALL": "C"}
	for _, name := range []string{".claude/skills/walden/SKILL.md", ".agents/skills/walden/SKILL.md", ".codex/AGENTS.md", ".config/walden/fixture-registry.json", ".zshrc"} {
		put(t, filepath.Join(f.home, name), "UNRELATED-BOOTSTRAP-SENTINEL:"+name+"\n", 0o600)
	}
	return f
}

func (f *fixture) run(shell string, args ...string) (string, int) {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, append([]string{f.script}, args...)...)
	cmd.Dir = f.root
	for key, value := range f.env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	// Do not inherit the developer's controlling terminal (/dev/tty).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		f.t.Fatalf("installer timed out: %s", output)
	}
	if err == nil {
		return string(output), 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return string(output), exit.ExitCode()
	}
	f.t.Fatal(err)
	return "", -1
}

func (f *fixture) eventText() string {
	f.t.Helper()
	data, err := os.ReadFile(f.events)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

func (f *fixture) binary() string { return filepath.Join(f.home, ".local/bin/walden") }

func (f *fixture) homeSnapshot() map[string]string {
	f.t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(f.home, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || path == f.binary() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(f.home, path)
		result[rel] = fmt.Sprintf("%o:%x", info.Mode().Perm(), sha256.Sum256(data))
		return nil
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return result
}

func (f *fixture) assertHomeUnchanged(before map[string]string) {
	f.t.Helper()
	after := f.homeSnapshot()
	if fmt.Sprint(before) != fmt.Sprint(after) {
		f.t.Fatalf("skill/registry/shell files changed:\nbefore %v\nafter %v", before, after)
	}
}

func TestBootstrapInstallerBinaryOnly(t *testing.T) {
	for _, item := range []struct {
		name, downloader string
		replace, latest  bool
	}{
		{"first-install", "curl", false, false},
		{"replace", "curl", true, false},
		{"latest", "curl", false, true},
		{"wget", "wget", false, false},
	} {
		t.Run(item.name, func(t *testing.T) {
			f := newFixture(t, item.downloader)
			if item.replace {
				put(t, f.binary(), "OLD-EXECUTABLE-SENTINEL", 0o700)
			}
			before := f.homeSnapshot()
			args := []string{"--no-skill"}
			if !item.latest {
				args = append(args, "--version", "v0.10.2")
			}
			output, code := f.run("/bin/sh", args...)
			if code != 0 {
				t.Fatalf("exit=%d: %s", code, output)
			}
			if read(t, f.binary()) != read(t, f.payload) {
				t.Fatal("installed bytes differ from verified payload")
			}
			info, _ := os.Stat(f.binary())
			if info.Mode().Perm()&0o111 == 0 {
				t.Fatal("installed payload is not executable")
			}
			if strings.Contains(f.eventText(), "payload skill") {
				t.Fatalf("binary-only path reached skill handoff: %s", f.eventText())
			}
			if !strings.Contains(f.eventText(), "payload version") || !strings.Contains(output, "Checksum verified") {
				t.Fatalf("missing binary/integrity checks: %s\n%s", output, f.eventText())
			}
			if strings.Contains(output, "skill install <agent>") {
				t.Fatal("deliberate binary-only skip recommends native skill reinstallation")
			}
			f.assertHomeUnchanged(before)
		})
	}
	for _, failure := range []string{"download", "missing-checksums", "bad-checksum", "missing-checksum-entry", "unsupported-os", "unsupported-arch", "version"} {
		t.Run(failure, func(t *testing.T) {
			f := newFixture(t, "curl")
			put(t, f.binary(), "OLD-EXECUTABLE-SENTINEL", 0o700)
			before := f.homeSnapshot()
			switch failure {
			case "download":
				f.env["TEST_DOWNLOAD_FAIL"] = "1"
			case "missing-checksums":
				f.env["TEST_MISSING_SUMS"] = "1"
			case "bad-checksum":
				put(t, f.sums, strings.Repeat("0", 64)+"  walden-v0.10.2-darwin-arm64\n", 0o600)
			case "missing-checksum-entry":
				put(t, f.sums, strings.Repeat("0", 64)+"  another-asset\n", 0o600)
			case "unsupported-os":
				f.env["TEST_OS"] = "UnsupportedOS"
			case "unsupported-arch":
				f.env["TEST_ARCH"] = "unsupported-cpu"
			case "version":
				f.env["TEST_VERSION_FAIL"] = "1"
			}
			output, code := f.run("/bin/sh", "--version", "v0.10.2", "--no-skill")
			if code == 0 {
				t.Fatalf("failure was accepted: %s", output)
			}
			if strings.Contains(output, "Unknown flag") {
				t.Fatalf("new mode was not actually exercised: %s", output)
			}
			if failure != "version" && read(t, f.binary()) != "OLD-EXECUTABLE-SENTINEL" {
				t.Fatal("pre-install failure replaced existing executable")
			}
			// The existing installer has no rollback guarantee after its self-check.
			if strings.Contains(f.eventText(), "payload skill") {
				t.Fatal("failed install reached skill handoff")
			}
			f.assertHomeUnchanged(before)
		})
	}
	t.Run("dash", func(t *testing.T) {
		shell, err := exec.LookPath("dash")
		if err != nil {
			t.Skip("additional POSIX shell unavailable; primary sh matrix remains required")
		}
		f := newFixture(t, "curl")
		output, code := f.run(shell, "--version", "v0.10.2", "--no-skill")
		if code != 0 {
			t.Fatalf("dash: %s", output)
		}
	})
}

// The installer places the binary and points at the Skills CLI for the guide.
// It never prompts for, installs, or removes a skill.
func TestBootstrapInstallerBinaryOnlyContract(t *testing.T) {
	const pointer = "npx skills add andrearaponi/walden"

	t.Run("skill-flag-rejected", func(t *testing.T) {
		for _, args := range [][]string{{"--skill", "claude"}, {"--skill", "all"}, {"--skill", "unknown-agent"}, {"--version", "v0.10.2", "--skill", "codex"}} {
			t.Run(strings.Join(args, "_"), func(t *testing.T) {
				f := newFixture(t, "curl")
				before := f.homeSnapshot()
				output, code := f.run("/bin/sh", args...)
				if code == 0 || !strings.Contains(output, pointer) {
					t.Fatalf("--skill was not rejected with the Skills CLI pointer: exit %d, %s", code, output)
				}
				if f.eventText() != "" {
					t.Fatalf("rejected flag reached platform/network/payload execution: %s", f.eventText())
				}
				if _, err := os.Stat(f.binary()); !os.IsNotExist(err) {
					t.Fatal("rejected flag installed a binary")
				}
				f.assertHomeUnchanged(before)
			})
		}
	})

	t.Run("no-skill-compat-noop", func(t *testing.T) {
		f := newFixture(t, "curl")
		before := f.homeSnapshot()
		output, code := f.run("/bin/sh", "--version", "v0.10.2", "--no-skill")
		if code != 0 || !strings.Contains(f.eventText(), "payload version") {
			t.Fatalf("--no-skill must install the binary: exit %d, %s\n%s", code, output, f.eventText())
		}
		if read(t, f.binary()) != read(t, f.payload) {
			t.Fatal("installed bytes differ from verified payload")
		}
		f.assertHomeUnchanged(before)
	})

	t.Run("default-no-prompt", func(t *testing.T) {
		f := newFixture(t, "curl")
		before := f.homeSnapshot()
		output, code := f.run("/bin/sh", "--version", "v0.10.2")
		if code != 0 {
			t.Fatalf("default install: %s", output)
		}
		if strings.Contains(f.eventText(), "payload skill") || strings.Contains(output, "Install Walden skill for") || strings.Contains(output, "Choice [") {
			t.Fatalf("default install prompted for or installed a skill: %s\n%s", output, f.eventText())
		}
		if !strings.Contains(output, pointer) {
			t.Fatalf("success epilogue does not point at the Skills CLI: %s", output)
		}
		f.assertHomeUnchanged(before)
	})

	for _, present := range []bool{false, true} {
		t.Run(fmt.Sprintf("uninstall-binary-only-%v", present), func(t *testing.T) {
			f := newFixture(t, "curl")
			if present {
				put(t, f.binary(), read(t, f.payload), 0o700)
			}
			before := f.homeSnapshot()
			output, code := f.run("/bin/sh", "--uninstall")
			if code != 0 {
				t.Fatalf("uninstall: %s", output)
			}
			if _, err := os.Stat(f.binary()); !os.IsNotExist(err) {
				t.Fatal("binary remains after uninstall")
			}
			if strings.Contains(f.eventText(), "payload skill") || strings.Contains(f.eventText(), "fetch ") {
				t.Fatalf("uninstall touched skills or the network: %s", f.eventText())
			}
			if read(t, filepath.Join(f.home, ".claude/skills/walden/SKILL.md")) != "UNRELATED-BOOTSTRAP-SENTINEL:.claude/skills/walden/SKILL.md\n" {
				t.Fatal("uninstall altered an agent skill file")
			}
			f.assertHomeUnchanged(before)
		})
	}

	t.Run("no-skill-with-uninstall-accepted", func(t *testing.T) {
		f := newFixture(t, "curl")
		put(t, f.binary(), read(t, f.payload), 0o700)
		_, code := f.run("/bin/sh", "--uninstall", "--no-skill")
		if code != 0 {
			t.Fatal("--no-skill is a no-op and must not conflict with --uninstall")
		}
	})

	t.Run("help", func(t *testing.T) {
		f := newFixture(t, "curl")
		output, code := f.run("/bin/sh", "--help")
		if code != 0 || f.eventText() != "" {
			t.Fatalf("invalid help behavior: %s", output)
		}
		if strings.Contains(output, "--skill <agent>") || strings.Contains(output, "Remove the skill") {
			t.Fatalf("help still advertises skill operations: %s", output)
		}
		if !strings.Contains(output, pointer) {
			t.Fatalf("help does not point at the Skills CLI: %s", output)
		}
	})

	t.Run("explicit-legacy-checksum-bypass", func(t *testing.T) {
		f := newFixture(t, "curl")
		f.env["TEST_MISSING_SUMS"] = "1"
		output, code := f.run("/bin/sh", "--version", "v0.10.2", "--no-verify")
		if code != 0 || !strings.Contains(output, "Checksum verification skipped") {
			t.Fatalf("existing explicit flag changed: %s", output)
		}
	})
}

// A non-POSIX host must be refused before any network use, and the refusal
// must name the two Windows paths instead of a bare "Unsupported OS".
func TestBootstrapInstallerUnsupportedOSPointer(t *testing.T) {
	for _, uname := range []string{"MINGW64_NT-10.0-22631", "MSYS_NT-10.0", "CYGWIN_NT-10.0"} {
		t.Run(uname, func(t *testing.T) {
			f := newFixture(t, "curl")
			f.env["TEST_OS"] = uname
			before := f.homeSnapshot()
			output, code := f.run("/bin/sh", "--version", "v0.10.4", "--no-skill")
			if code == 0 {
				t.Fatalf("installer accepted %s: %s", uname, output)
			}
			for _, want := range []string{"go install github.com/andrearaponi/walden/cmd/walden@", ".exe", "releases"} {
				if !strings.Contains(output, want) {
					t.Errorf("refusal on %s must name %q, got:\n%s", uname, want, output)
				}
			}
			for _, line := range strings.Split(read(t, f.events), "\n") {
				if strings.HasPrefix(line, "download ") {
					t.Fatalf("network used before the OS refusal: %s", line)
				}
			}
			if after := f.homeSnapshot(); !reflect.DeepEqual(before, after) {
				t.Fatalf("refusal changed the home: %v → %v", before, after)
			}
		})
	}
}
