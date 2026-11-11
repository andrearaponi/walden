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
	return captureManifest(ctx, runner, root, "HEAD", true)
}

// RecordingLimit is the largest number of differing paths a record keeps.
// Over it the record keeps their number instead: a ledger is committed and
// read, and a tree with hundreds of uncommitted paths is not what it is for.
const RecordingLimit = 100

// Capture is a working-tree capture together with what its manifest was built
// from. The facts explain a later difference; they never judge a state.
type Capture struct {
	Manifest Manifest
	// Commit is the object id HEAD named, empty when it does not resolve.
	Commit string
	// Differing holds every path whose manifest entry differs from the
	// commit's listing: its "mode:blob" entry, or "" when the commit holds
	// the path and the working tree does not. Nil without a commit.
	Differing map[string]string
	// Untracked holds the paths git reports as untracked.
	Untracked map[string]bool

	// listed is the commit's own listing, kept to tell a path the commit
	// holds from one it does not.
	listed Manifest
}

// CaptureTree captures the working tree as CaptureManifest does and also
// reports the commit and the paths behind the manifest. It adds one git
// invocation, the commit: the listing is then read from that same id, so the
// two cannot disagree. When the answer is not an object id — a repository
// without a commit, or a runner that does not script the subcommand — the
// capture lists HEAD as CaptureManifest does and yields no facts.
//
// It is for the places that write or explain a record. A command that only
// needs the identity keeps calling CaptureManifest, whose invocations do not
// change.
func CaptureTree(ctx context.Context, runner shell.Runner, root string) (Capture, bool) {
	commit := ""
	if response, err := Git(ctx, runner, root, "rev-parse", "--verify", "HEAD^{commit}"); err == nil && response.ExitCode == 0 {
		if id := strings.TrimSpace(response.Stdout); objectID(id) {
			commit = id
		}
	}
	revision := "HEAD"
	if commit != "" {
		revision = commit
	}

	parts, ok := captureParts(ctx, runner, root, revision, true, true)
	if !ok {
		return Capture{}, false
	}
	capture := Capture{Manifest: parts.manifest, Commit: commit, Untracked: parts.untracked, listed: parts.listed}
	if commit != "" {
		capture.Differing = differingPaths(parts.listed, parts.manifest)
	}
	return capture, true
}

// Committed reports that the working tree holds what its commit holds: the
// capture names a commit and no path outside .walden/ differs from it. It is
// what "no uncommitted path" means for an attestation, in the terms of the
// code identity: a path differs exactly when it moves the digest, so the
// identity of a committed capture is the identity of the tree of its commit.
func (c Capture) Committed() bool {
	return c.Commit != "" && len(c.Differing) == 0
}

// Current describes what the captured identity is made of: the commit it
// rests on and each path that differs from it, in path order, as changed,
// added or removed, with the untracked ones marked. In a repository without
// a commit it names none and every path is added.
func (c Capture) Current() *CurrentTree {
	current := &CurrentTree{Commit: c.Commit}
	differing := c.Differing
	if c.Commit == "" {
		differing = c.Manifest
	}
	for path, entry := range differing {
		kind := KindAdded
		if entry == "" {
			kind = KindRemoved
		} else if _, held := c.listed[path]; held && c.Commit != "" {
			kind = KindChanged
		}
		current.Paths = append(current.Paths, TreePath{Path: path, Kind: kind, Untracked: c.Untracked[path]})
	}
	sort.Slice(current.Paths, func(i, j int) bool { return current.Paths[i].Path < current.Paths[j].Path })
	return current
}

// differingPaths returns every path whose entry differs between a commit's
// listing and a manifest. Both sides use the manifest's own representation,
// so a path counts as differing exactly when it moves the digest.
func differingPaths(listed, manifest Manifest) map[string]string {
	paths := map[string]string{}
	for path, entry := range manifest {
		if listed[path] != entry {
			paths[path] = entry
		}
	}
	for path := range listed {
		if _, exists := manifest[path]; !exists {
			paths[path] = ""
		}
	}
	return paths
}

// Facts returns what a record keeps of the capture: nothing without a
// commit, the commit with its differing paths up to the recording limit, and
// the commit with their number over it.
func (c Capture) Facts() *TreeFacts {
	if c.Commit == "" {
		return nil
	}
	facts := &TreeFacts{Commit: c.Commit}
	if len(c.Differing) > RecordingLimit {
		facts.PathsOmitted = len(c.Differing)
		return facts
	}
	if len(c.Differing) > 0 {
		facts.Paths = make(map[string]string, len(c.Differing))
		for path, entry := range c.Differing {
			facts.Paths[path] = entry
		}
	}
	return facts
}

// RebuildManifest applies a record's differing paths to the listing of its
// commit (`git ls-tree -r`). The result is the manifest the record's code
// identity was computed from, which its digest confirms or refutes.
func RebuildManifest(listing string, paths map[string]string) Manifest {
	manifest := Manifest{}
	seedListing(manifest, listing)
	for path, entry := range paths {
		if entry == "" {
			delete(manifest, path)
			continue
		}
		manifest[path] = entry
	}
	return manifest
}

// CaptureManifestAtCommit uses the existing identity representation anchored
// to a captured revision. An unreadable captured commit is never unborn.
func CaptureManifestAtCommit(ctx context.Context, runner shell.Runner, root, commit string) (Manifest, bool) {
	if commit == "" {
		return nil, false
	}
	return captureManifest(ctx, runner, root, commit, false)
}

func captureManifest(ctx context.Context, runner shell.Runner, root, revision string, allowUnborn bool) (Manifest, bool) {
	parts, ok := captureParts(ctx, runner, root, revision, allowUnborn, false)
	if !ok {
		return nil, false
	}
	return parts.manifest, true
}

// captureParts is one capture of the working tree. withFacts also keeps the
// revision's listing and the untracked paths; without it the capture runs
// the same git invocations and keeps the manifest alone.
type captureResult struct {
	manifest  Manifest
	listed    Manifest
	untracked map[string]bool
}

func captureParts(ctx context.Context, runner shell.Runner, root, revision string, allowUnborn, withFacts bool) (captureResult, bool) {
	status, err := Git(ctx, runner, root, "status", "--porcelain", "--untracked-files=all", "-z", "--", ".", ":(exclude)"+waldenDir)
	if err != nil || status.ExitCode != 0 {
		return captureResult{}, false
	}

	blobs := Manifest{}
	if lsTree, err := Git(ctx, runner, root, "ls-tree", "-r", revision); err == nil && lsTree.ExitCode == 0 {
		seedListing(blobs, lsTree.Stdout)
	} else if !allowUnborn || !unbornRepository(ctx, runner, root) {
		// Failure to read an existing tree is not evidence of an empty tree.
		return captureResult{}, false
	}

	result := captureResult{}
	if withFacts {
		result.listed = make(Manifest, len(blobs))
		for path, entry := range blobs {
			result.listed[path] = entry
		}
		result.untracked = untrackedPaths(status.Stdout)
	}

	if !applyOverlay(ctx, runner, root, status.Stdout, blobs) {
		return captureResult{}, false
	}
	result.manifest = blobs
	return result, true
}

// untrackedPaths reads the paths git reports as untracked from NUL-separated
// porcelain output, through the parser the overlay uses: the source field of
// a rename or a copy is never an entry of its own.
func untrackedPaths(porcelain string) map[string]bool {
	untracked := map[string]bool{}
	for _, entry := range ParseStatus(porcelain) {
		if entry.Code == "??" && !isWaldenPath(entry.Path) {
			untracked[entry.Path] = true
		}
	}
	return untracked
}

// unbornRepository positively establishes a symbolic HEAD whose branch ref
// is absent. Corrupt refs, detached HEAD and plumbing errors do not qualify.
func unbornRepository(ctx context.Context, runner shell.Runner, root string) bool {
	head, err := Git(ctx, runner, root, "symbolic-ref", "-q", "HEAD")
	if err != nil || head.ExitCode != 0 || !strings.HasPrefix(strings.TrimSpace(head.Stdout), "refs/heads/") {
		return false
	}
	ref, err := Git(ctx, runner, root, "show-ref", "--verify", "--quiet", strings.TrimSpace(head.Stdout))
	return err == nil && ref.ExitCode == 1
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
// paths leave the map, and so does the source of a rename. Regular files are
// hashed in chunked batches — one git spawn per file made a 400-file dirty
// tree cost seconds per status.
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

	for _, entry := range ParseStatus(porcelain) {
		if entry.Renamed() {
			// A rename took its source out of the working tree, so the entry
			// the commit's listing seeded for it leaves the manifest. Left
			// there, it would move the identity when the rename is committed
			// and hide a file created again at that path. Such a file is
			// reported on its own and hashed after this loop, whatever the
			// order of the entries. A copy's source is still in the tree.
			delete(blobs, entry.Source)
		}
		path := entry.Path
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

		args := append([]string{"hash-object", "--"}, hashArgs[start:end]...)
		hashed, err := Git(ctx, runner, root, args...)
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
