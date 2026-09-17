package evidence

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/andrearaponi/walden/internal/shell"
)

// Git is read-only plumbing: disable status's optional index refresh locks.
// Callers supply literal argv and never request a checkout, fetch or write.
func Git(ctx context.Context, runner shell.Runner, root string, args ...string) (shell.Response, error) {
	return runner.Run(ctx, "git", append([]string{"--no-optional-locks", "-C", root}, args...)...)
}

// Commit resolves one existing commit, never an unborn or non-commit object.
func Commit(ctx context.Context, runner shell.Runner, root, ref string) (string, error) {
	response, err := Git(ctx, runner, root, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil || response.ExitCode != 0 || !objectID(strings.TrimSpace(response.Stdout)) {
		return "", fmt.Errorf("commit %s unavailable (%v; %s)", ref, err, strings.TrimSpace(response.Stderr))
	}
	return strings.TrimSpace(response.Stdout), nil
}

type TreeEntry struct{ Mode, Kind, OID, Path string }

// ParseTree reads ls-tree -z output without quoting/path ambiguities.
func ParseTree(text string) ([]TreeEntry, error) {
	var entries []TreeEntry
	for _, item := range strings.Split(text, "\x00") {
		if item == "" {
			continue
		}
		meta, path, ok := strings.Cut(item, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 || !objectID(fields[2]) {
			return nil, fmt.Errorf("invalid Git tree entry %q", item)
		}
		entries = append(entries, TreeEntry{Mode: fields[0], Kind: fields[1], OID: fields[2], Path: path})
	}
	return entries, nil
}

func objectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
