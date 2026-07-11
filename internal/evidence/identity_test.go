package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
)

// fakeGit answers git subcommands from a canned map keyed by the second
// argument after -C <root> (e.g. "status", "ls-tree").
type fakeGit struct {
	responses map[string]shell.Response
	errors    map[string]error
}

func (f *fakeGit) Run(_ context.Context, name string, args ...string) (shell.Response, error) {
	if name != "git" || len(args) < 3 {
		return shell.Response{}, errors.New("unexpected invocation")
	}
	sub := args[2]
	if err, exists := f.errors[sub]; exists {
		return shell.Response{}, err
	}
	if sub == "hash-object" {
		// Content-derived fake blob id: sha256 of the file, mirroring how
		// real blob ids are stable functions of content. Absolute paths
		// (the staged symlink-target copies) are honored as git does.
		root, path := args[1], args[len(args)-1]
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return shell.Response{ExitCode: 128, Stderr: err.Error()}, nil
		}
		sum := sha256.Sum256(content)
		return shell.Response{ExitCode: 0, Stdout: hex.EncodeToString(sum[:]) + "\n"}, nil
	}
	if resp, exists := f.responses[sub]; exists {
		return resp, nil
	}
	return shell.Response{ExitCode: 0}, nil
}

func lsTreeListing(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

func TestIdentityCleanTreeExcludesWalden(t *testing.T) {
	root := t.TempDir()

	withEvidence := &fakeGit{responses: map[string]shell.Response{
		"status": {ExitCode: 0},
		"ls-tree": {ExitCode: 0, Stdout: lsTreeListing(
			"100644 blob aaa\tmain.go",
			"100644 blob bbb\t.walden/evidence/f.json",
		)},
	}}
	withoutEvidence := &fakeGit{responses: map[string]shell.Response{
		"status":  {ExitCode: 0},
		"ls-tree": {ExitCode: 0, Stdout: lsTreeListing("100644 blob aaa\tmain.go")},
	}}
	differentCode := &fakeGit{responses: map[string]shell.Response{
		"status":  {ExitCode: 0},
		"ls-tree": {ExitCode: 0, Stdout: lsTreeListing("100644 blob ccc\tmain.go")},
	}}

	first, ok := Identity(context.Background(), withEvidence, root)
	if !ok {
		t.Fatal("identity not computed")
	}
	second, ok := Identity(context.Background(), withoutEvidence, root)
	if !ok {
		t.Fatal("identity not computed")
	}
	third, ok := Identity(context.Background(), differentCode, root)
	if !ok {
		t.Fatal("identity not computed")
	}

	if first != second {
		t.Fatalf("committing .walden content changed the identity: %s vs %s", first, second)
	}
	if first == third {
		t.Fatal("a code change did not change the identity")
	}
	if !strings.HasPrefix(first, "sha256:") {
		t.Fatalf("identity %q lacks the sha256 prefix", first)
	}
}

func TestIdentityDirtyOverlayIsOrderIndependent(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{"a.go": "alpha", "b.go": "beta"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	orderAB := &fakeGit{responses: map[string]shell.Response{
		"status":  {ExitCode: 0, Stdout: " M a.go\x00?? b.go\x00"},
		"ls-tree": {ExitCode: 0, Stdout: lsTreeListing("100644 blob aaa\tmain.go")},
	}}
	orderBA := &fakeGit{responses: map[string]shell.Response{
		"status":  {ExitCode: 0, Stdout: "?? b.go\x00 M a.go\x00"},
		"ls-tree": {ExitCode: 0, Stdout: lsTreeListing("100644 blob aaa\tmain.go")},
	}}

	first, _ := Identity(context.Background(), orderAB, root)
	second, _ := Identity(context.Background(), orderBA, root)
	if first != second {
		t.Fatalf("status ordering changed the identity: %s vs %s", first, second)
	}
}

func TestIdentityTracksUntrackedContent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "new.go")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	git := &fakeGit{responses: map[string]shell.Response{
		"status":  {ExitCode: 0, Stdout: "?? new.go\x00"},
		"ls-tree": {ExitCode: 0, Stdout: ""},
	}}

	before, _ := Identity(context.Background(), git, root)
	if err := os.WriteFile(path, []byte("v2"), 0o644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}
	after, _ := Identity(context.Background(), git, root)

	if before == after {
		t.Fatal("untracked content change did not change the identity")
	}
}

func TestIdentityHandlesDeletedFiles(t *testing.T) {
	root := t.TempDir()
	git := &fakeGit{responses: map[string]shell.Response{
		"status":  {ExitCode: 0, Stdout: " D gone.go\x00"},
		"ls-tree": {ExitCode: 0, Stdout: lsTreeListing("100644 blob aaa\tgone.go")},
	}}

	identity, ok := Identity(context.Background(), git, root)
	if !ok || identity == "" {
		t.Fatal("deleted file broke identity computation")
	}
}

func TestIdentityUnbornHeadUsesOverlayAlone(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "first.go"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	git := &fakeGit{responses: map[string]shell.Response{
		"status":  {ExitCode: 0, Stdout: "?? first.go\x00"},
		"ls-tree": {ExitCode: 128, Stderr: "fatal: not a valid object name HEAD"},
	}}

	identity, ok := Identity(context.Background(), git, root)
	if !ok || identity == "" {
		t.Fatal("unborn HEAD did not degrade to the overlay")
	}
}

func TestIdentityGitFailureReturnsAbsent(t *testing.T) {
	root := t.TempDir()

	cases := []*fakeGit{
		{responses: map[string]shell.Response{"status": {ExitCode: 128, Stderr: "fatal: not a git repository"}}},
		{errors: map[string]error{"status": errors.New("executable file not found")}},
	}
	for _, git := range cases {
		if identity, ok := Identity(context.Background(), git, root); ok || identity != "" {
			t.Fatalf("git failure yielded identity %q ok=%t, want absent", identity, ok)
		}
	}
}

func TestIdentitySurvivesTheCommitTransition(t *testing.T) {
	// The serpent's little sibling: an untracked file and the same file
	// committed must produce the same identity — layer transitions are not
	// content changes.
	root := t.TempDir()
	content := []byte("#!/bin/sh\necho 5\n")
	if err := os.WriteFile(filepath.Join(root, "calc.sh"), content, 0o755); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	blob := sha256.Sum256(content)
	blobID := hex.EncodeToString(blob[:])

	untracked := &fakeGit{responses: map[string]shell.Response{
		"status":  {ExitCode: 0, Stdout: "?? calc.sh\x00"},
		"ls-tree": {ExitCode: 128, Stderr: "fatal: not a valid object name HEAD"},
	}}
	committed := &fakeGit{responses: map[string]shell.Response{
		"status":  {ExitCode: 0},
		"ls-tree": {ExitCode: 0, Stdout: "100644 blob " + blobID + "\tcalc.sh\n"},
	}}

	before, ok := Identity(context.Background(), untracked, root)
	if !ok {
		t.Fatal("untracked identity not computed")
	}
	after, ok := Identity(context.Background(), committed, root)
	if !ok {
		t.Fatal("committed identity not computed")
	}
	if before != after {
		t.Fatalf("committing unchanged content moved the identity: %s vs %s", before, after)
	}
}

func TestIdentitySurvivesTheSymlinkCommitTransition(t *testing.T) {
	// Git stores a symlink blob as its target string; following the link
	// would hash the target's content instead. Both layers must agree.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte("build:\n\ttrue\n"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	if err := os.Symlink("Makefile", filepath.Join(root, "link.mk")); err != nil {
		t.Fatalf("seed symlink: %v", err)
	}
	targetBlob := sha256.Sum256([]byte("Makefile"))
	linkBlobID := hex.EncodeToString(targetBlob[:])
	fileBlob := sha256.Sum256([]byte("build:\n\ttrue\n"))
	fileBlobID := hex.EncodeToString(fileBlob[:])

	untracked := &fakeGit{responses: map[string]shell.Response{
		"status":  {ExitCode: 0, Stdout: "?? Makefile\x00?? link.mk\x00"},
		"ls-tree": {ExitCode: 128, Stderr: "fatal: not a valid object name HEAD"},
	}}
	committed := &fakeGit{responses: map[string]shell.Response{
		"status": {ExitCode: 0},
		"ls-tree": {ExitCode: 0, Stdout: "100644 blob " + fileBlobID + "\tMakefile\n" +
			"120000 blob " + linkBlobID + "\tlink.mk\n"},
	}}

	before, ok := Identity(context.Background(), untracked, root)
	if !ok {
		t.Fatal("untracked identity not computed")
	}
	after, ok := Identity(context.Background(), committed, root)
	if !ok {
		t.Fatal("committed identity not computed")
	}
	if before != after {
		t.Fatalf("committing an unchanged symlink moved the identity: %s vs %s", before, after)
	}
}
