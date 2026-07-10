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

// Identity derives a deterministic identity of the working tree as one
// uniform path→blob map: the .walden-filtered HEAD listing seeds committed
// blob ids, and every dirty or untracked path overrides its entry with the
// blob id of its current content (git hash-object). Uniform blob ids on both
// layers mean committing an unchanged file never moves the identity.
// Identical content yields identical identities regardless of branch names,
// timestamps, or .walden contents. The second return is false when git is
// unavailable — callers record the absence and continue.
func Identity(ctx context.Context, runner shell.Runner, root string) (string, bool) {
	status, err := runner.Run(ctx, "git", "-C", root, "status", "--porcelain", "--untracked-files=all", "-z", "--", ".", ":(exclude)"+waldenDir)
	if err != nil || status.ExitCode != 0 {
		return "", false
	}

	// Unborn HEAD (no commits yet) degrades to the overlay alone; the
	// repository itself is proven present by the successful status call.
	blobs := map[string]string{}
	if lsTree, err := runner.Run(ctx, "git", "-C", root, "ls-tree", "-r", "HEAD"); err == nil && lsTree.ExitCode == 0 {
		seedListing(blobs, lsTree.Stdout)
	}

	if !applyOverlay(ctx, runner, root, status.Stdout, blobs) {
		return "", false
	}

	entries := make([]string, 0, len(blobs))
	for path, blob := range blobs {
		entries = append(entries, path+":"+blob)
	}
	sort.Strings(entries)

	sum := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return "sha256:" + hex.EncodeToString(sum[:]), true
}

// seedListing fills the blob map from a `git ls-tree -r HEAD` listing
// (format: "<mode> <type> <hash>\t<path>"), dropping .walden/ entries.
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
		blobs[path] = fields[2]
	}
}

// applyOverlay overrides the blob map with the current content of every
// dirty or untracked path from NUL-separated porcelain output, using git
// hash-object so the representation matches committed entries. Deleted
// paths leave the map.
func applyOverlay(ctx context.Context, runner shell.Runner, root, porcelain string, blobs map[string]string) bool {
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

		if _, err := os.Stat(filepath.Join(root, path)); os.IsNotExist(err) {
			delete(blobs, path)
			continue
		}

		hashed, err := runner.Run(ctx, "git", "-C", root, "hash-object", "--", path)
		if err != nil || hashed.ExitCode != 0 {
			return false
		}
		blob := strings.TrimSpace(hashed.Stdout)
		if blob == "" {
			return false
		}
		blobs[path] = blob
	}
	return true
}

func isWaldenPath(path string) bool {
	return path == waldenDir || strings.HasPrefix(path, waldenDir+"/")
}
