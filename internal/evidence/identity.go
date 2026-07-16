package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/andrearaponi/walden/internal/shell"
)

// waldenDir is excluded from the code identity at every layer: spec changes
// invalidate through fingerprints, and committing the evidence document must
// never invalidate the evidence it carries.
const waldenDir = ".walden"

// Manifest is the working tree's uniform path→"mode:blob" map — the raw
// material of the code identity, exported so re-verification can attribute a
// working-tree change to the proof that ran between two captures and name
// the paths involved.
type Manifest map[string]string

// CaptureManifest derives the deterministic manifest of the working tree:
// the .walden-filtered HEAD listing seeds committed blob ids, and every
// dirty or untracked path overrides its entry with the blob id of its
// current content (git hash-object). Uniform blob ids on both layers mean
// committing an unchanged file never moves the manifest. The second return
// is false when git is unavailable — callers record the absence and continue.
func CaptureManifest(ctx context.Context, runner shell.Runner, root string) (Manifest, bool) {
	status, err := runner.Run(ctx, "git", "-C", root, "status", "--porcelain", "--untracked-files=all", "-z", "--", ".", ":(exclude)"+waldenDir)
	if err != nil || status.ExitCode != 0 {
		return nil, false
	}

	// Unborn HEAD (no commits yet) degrades to the overlay alone; the
	// repository itself is proven present by the successful status call.
	blobs := Manifest{}
	if lsTree, err := runner.Run(ctx, "git", "-C", root, "ls-tree", "-r", "HEAD"); err == nil && lsTree.ExitCode == 0 {
		seedListing(blobs, lsTree.Stdout)
	}

	if !applyOverlay(ctx, runner, root, status.Stdout, blobs) {
		return nil, false
	}
	return blobs, true
}

// Digest folds the manifest into the identity string. Identical content
// yields identical digests regardless of branch names, timestamps, or
// .walden contents.
func (m Manifest) Digest() string {
	entries := make([]string, 0, len(m))
	for path, blob := range m {
		entries = append(entries, path+":"+blob)
	}
	sort.Strings(entries)

	sum := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// DiffPaths returns the sorted union of paths whose entries differ between
// two manifests: added, removed, or content-changed.
func DiffPaths(before, after Manifest) []string {
	changed := []string{}
	for path, blob := range before {
		if other, exists := after[path]; !exists || other != blob {
			changed = append(changed, path)
		}
	}
	for path := range after {
		if _, exists := before[path]; !exists {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed
}

// Identity derives the deterministic identity of the working tree — the
// digest of its manifest. The second return is false when git is
// unavailable.
func Identity(ctx context.Context, runner shell.Runner, root string) (string, bool) {
	manifest, ok := CaptureManifest(ctx, runner, root)
	if !ok {
		return "", false
	}
	return manifest.Digest(), true
}

// seedListing fills the map from a `git ls-tree -r HEAD` listing (format:
// "<mode> <type> <hash>\t<path>"), dropping .walden/ entries. Entries carry
// "<mode>:<blob>": the executable bit is code, so mode changes must move
// the identity.
func seedListing(blobs map[string]string, listing string) {
	for _, line := range strings.Split(listing, "\n") {
		meta, path, found := strings.Cut(line, "\t")
		if !found || isWaldenPath(path) {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) < 3 {
			continue
		}
		blobs[path] = fields[0] + ":" + fields[2]
	}
}

// applyOverlay overrides the blob map with the current content of every
// dirty or untracked path from NUL-separated porcelain output, using git
// hash-object so the representation matches committed entries. Deleted
// paths leave the map. Regular files are hashed in chunked batches — one
// git spawn per file made a 400-file dirty tree cost seconds per status.
func applyOverlay(ctx context.Context, runner shell.Runner, root, porcelain string, blobs map[string]string) bool {
	pending := []string{}  // repo-relative paths, hashed in batches
	hashArgs := []string{} // the argv actually passed for each pending path
	modes := []string{}    // canonical git mode per pending path
	cleanups := []string{}

	defer func() {
		for _, name := range cleanups {
			_ = os.Remove(name)
		}
	}()

	fields := strings.Split(porcelain, "\x00")
	for index := 0; index < len(fields); index++ {
		entry := fields[index]
		if len(entry) < 4 {
			continue
		}
		statusCode, path := entry[:2], entry[3:]
		if strings.HasPrefix(statusCode, "R") || strings.HasPrefix(statusCode, "C") {
			// Renames and copies carry the source path as the next field.
			index++
		}
		if isWaldenPath(path) {
			continue
		}

		info, err := os.Lstat(filepath.Join(root, path))
		if os.IsNotExist(err) {
			delete(blobs, path)
			continue
		}
		if err != nil {
			return false
		}

		hashArg := path
		mode := "100644"
		if info.Mode().Perm()&0o111 != 0 {
			mode = "100755"
		}
		if info.Mode()&os.ModeSymlink != 0 {
			mode = "120000"
			// Git stores a symlink as a blob of its target string, while
			// hash-object on the link path would follow it and hash the
			// target's content — a representation split that would make the
			// untracked→committed transition read as a code change. Hash a
			// staged copy of the target string instead.
			target, err := os.Readlink(filepath.Join(root, path))
			if err != nil {
				return false
			}
			staged, err := os.CreateTemp("", ".walden-linkblob-*")
			if err != nil {
				return false
			}
			if _, err := staged.WriteString(target); err != nil {
				_ = staged.Close()
				_ = os.Remove(staged.Name())
				return false
			}
			_ = staged.Close()
			cleanups = append(cleanups, staged.Name())
			hashArg = staged.Name()
		}
		pending = append(pending, path)
		hashArgs = append(hashArgs, hashArg)
		modes = append(modes, mode)
	}

	const chunkSize = 500
	for start := 0; start < len(pending); start += chunkSize {
		end := start + chunkSize
		if end > len(pending) {
			end = len(pending)
		}

		args := append([]string{"-C", root, "hash-object", "--"}, hashArgs[start:end]...)
		hashed, err := runner.Run(ctx, "git", args...)
		if err != nil || hashed.ExitCode != 0 {
			return false
		}
		lines := strings.Fields(hashed.Stdout)
		if len(lines) != end-start {
			return false
		}
		for offset, blob := range lines {
			blobs[pending[start+offset]] = modes[start+offset] + ":" + blob
		}
	}
	return true
}

func isWaldenPath(path string) bool {
	return path == waldenDir || strings.HasPrefix(path, waldenDir+"/")
}
