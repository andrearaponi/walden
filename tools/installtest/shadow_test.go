//go:build darwin || linux

package installtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A walden earlier on PATH than the one just installed is reported with its
// path, its version and the two remedies. It is executed once to ask for its
// version and never moved, removed or written.
func TestBootstrapInstallerShadowWarning(t *testing.T) {
	const warning = "walden on your PATH is "
	shadow := func(f *fixture, body string) (dir, path string) {
		dir = filepath.Join(f.root, "shadow-bin")
		path = filepath.Join(dir, "walden")
		put(f.t, path, body, 0o755)
		return dir, path
	}
	untouched := func(t *testing.T, path, body string) {
		t.Helper()
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if read(t, path) != body || info.Mode().Perm() != 0o755 || info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("shadow binary changed: mode %v", info.Mode())
		}
	}

	t.Run("old-shadow", func(t *testing.T) {
		f := newFixture(t, "curl")
		const body = "#!/bin/sh\nprintf 'walden v0.9.9 (shadow)\\n'\n"
		dir, path := shadow(f, body)
		f.env["PATH"] = dir + string(os.PathListSeparator) + f.bin
		output, code := f.run("/bin/sh", "--version", "v0.10.2")
		if code != 0 {
			t.Fatalf("install failed with a shadow on PATH: exit %d\n%s", code, output)
		}
		for _, want := range []string{
			warning + path + " (walden v0.9.9), not " + f.binary() + ".",
			"Remove it, or update it with the tool that installed it",
			"go install github.com/andrearaponi/walden/cmd/walden@v0.10.2",
		} {
			if !strings.Contains(output, want) {
				t.Fatalf("missing %q in:\n%s", want, output)
			}
		}
		untouched(t, path, body)
	})

	t.Run("shadow-no-version", func(t *testing.T) {
		f := newFixture(t, "curl")
		const body = "#!/bin/sh\nexit 3\n"
		dir, path := shadow(f, body)
		f.env["PATH"] = dir + string(os.PathListSeparator) + f.bin
		output, code := f.run("/bin/sh", "--version", "v0.10.2")
		if code != 0 {
			t.Fatalf("install failed with an unanswering shadow: exit %d\n%s", code, output)
		}
		if !strings.Contains(output, warning+path+", not "+f.binary()+".") {
			t.Fatalf("warning does not name the path alone:\n%s", output)
		}
		for _, line := range strings.Split(output, "\n") {
			if strings.Contains(line, warning) && strings.Contains(line, "(") {
				t.Fatalf("invented version text: %q", line)
			}
		}
		untouched(t, path, body)
	})

	t.Run("no-false-alarm", func(t *testing.T) {
		for _, layout := range []string{"local-bin-first", "link-first", "no-other-walden"} {
			t.Run(layout, func(t *testing.T) {
				f := newFixture(t, "curl")
				switch layout {
				case "local-bin-first":
					f.env["PATH"] = filepath.Dir(f.binary()) + string(os.PathListSeparator) + f.bin
				case "link-first":
					dir := filepath.Join(f.root, "link-bin")
					if err := os.MkdirAll(dir, 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(f.binary(), filepath.Join(dir, "walden")); err != nil {
						t.Fatal(err)
					}
					f.env["PATH"] = dir + string(os.PathListSeparator) + f.bin
				}
				output, code := f.run("/bin/sh", "--version", "v0.10.2")
				if code != 0 {
					t.Fatalf("install failed: exit %d\n%s", code, output)
				}
				if strings.Contains(output, warning) {
					t.Fatalf("false shadowing warning for %s:\n%s", layout, output)
				}
			})
		}
	})
}
