package evidence

import (
	"context"
	"strings"

	"github.com/andrearaponi/walden/internal/shell"
)

// RootPrefix asks git where root lies in its working tree: the path from the
// top level down to root, with its trailing slash, or the empty string when
// root is the top level. The second return is false when git does not answer
// with such a path.
//
// Git reports a status from the top level and everything else from the
// directory it runs in. This is the one fact that joins the two.
func RootPrefix(ctx context.Context, runner shell.Runner, root string) (string, bool) {
	response, err := Git(ctx, runner, root, "rev-parse", "--show-prefix")
	if err != nil || response.ExitCode != 0 {
		return "", false
	}
	prefix := strings.TrimSuffix(strings.TrimSuffix(response.Stdout, "\n"), "\r")
	if prefix == "" {
		return "", true
	}
	if !strings.HasSuffix(prefix, "/") || strings.ContainsAny(prefix, "\x00\n") {
		return "", false
	}
	return prefix, true
}

// placeStatus rewrites the paths of status entries, which git reports from
// the top level, as paths relative to the root that lies at prefix. The
// second return is false when an entry names a path, or the source of a
// rename or a copy, that is not under the root: such a path cannot be read
// from the root, and passing over it would leave it out of the code identity
// without a word.
func placeStatus(entries []StatusEntry, prefix string) ([]StatusEntry, bool) {
	placed := make([]StatusEntry, 0, len(entries))
	for _, entry := range entries {
		path, under := strings.CutPrefix(entry.Path, prefix)
		if !under || path == "" {
			return nil, false
		}
		entry.Path = path
		if entry.Source != "" {
			source, under := strings.CutPrefix(entry.Source, prefix)
			if !under || source == "" {
				return nil, false
			}
			entry.Source = source
		}
		placed = append(placed, entry)
	}
	return placed, true
}
