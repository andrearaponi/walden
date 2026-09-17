package release

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/spec"
)

var readInputFile = os.ReadFile

type inputSnapshot struct {
	Path     string
	Present  bool
	Required bool
	Data     []byte
	Err      error
}

type featureSnapshot struct {
	Feature   spec.Feature
	Ledger    evidence.Document
	LoadErr   error
	LedgerErr error
	Inputs    []inputSnapshot
}

// InputBinding names the exact consumed path and its relationship to the
// captured commit. It is an observation, not a stored certification state.
type InputBinding struct {
	Path   string `json:"path"`
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

func captureInput(root, relative string, required, strict bool) inputSnapshot {
	input := inputSnapshot{Path: filepath.ToSlash(relative), Required: required}
	if !filepath.IsLocal(relative) {
		input.Err = fmt.Errorf("input path escapes the repository")
		return input
	}
	path := filepath.Join(root, relative)
	if strict {
		cursor := root
		parts := strings.Split(filepath.ToSlash(relative), "/")
		for i, part := range parts {
			cursor = filepath.Join(cursor, part)
			info, err := os.Lstat(cursor)
			if errors.Is(err, os.ErrNotExist) {
				return input
			}
			if err != nil {
				input.Err = err
				return input
			}
			if info.Mode()&os.ModeSymlink != 0 {
				input.Err = fmt.Errorf("symbolic link at %s is not a committed regular-file input", cursor)
				return input
			}
			if i == len(parts)-1 && !info.Mode().IsRegular() {
				input.Err = fmt.Errorf("input is not a regular file")
				return input
			}
			if i < len(parts)-1 && !info.IsDir() {
				input.Err = fmt.Errorf("non-directory input ancestor %s", cursor)
				return input
			}
		}
	}
	data, err := readInputFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return input
	}
	input.Present, input.Data, input.Err = err == nil, data, err
	return input
}

func captureFeature(root, name string, strict bool) featureSnapshot {
	featureRoot := filepath.Join(root, ".walden", "specs", name)
	snapshot := featureSnapshot{Feature: spec.Feature{Name: name, Root: featureRoot}}
	for _, entry := range []struct {
		name   string
		target *spec.Document
	}{
		{"requirements.md", &snapshot.Feature.Requirements},
		{"design.md", &snapshot.Feature.Design},
		{"tasks.md", &snapshot.Feature.Tasks},
	} {
		input := captureInput(root, filepath.Join(".walden", "specs", name, entry.name), true, strict)
		snapshot.Inputs = append(snapshot.Inputs, input)
		path := filepath.Join(featureRoot, entry.name)
		*entry.target = spec.Document{Path: path}
		if input.Err != nil {
			snapshot.LoadErr = errors.Join(snapshot.LoadErr, fmt.Errorf("%s: %w", input.Path, input.Err))
			continue
		}
		if !input.Present {
			continue
		}
		document, err := spec.ParseDocument(path, input.Data)
		if err != nil {
			snapshot.LoadErr = errors.Join(snapshot.LoadErr, fmt.Errorf("%s: %w", input.Path, err))
			continue
		}
		*entry.target = document
	}
	ledgerRequired := false
	if tree, err := spec.ParseTaskTree(snapshot.Feature.Tasks); err == nil {
		for _, task := range tree.LeafTasks() {
			ledgerRequired = ledgerRequired || task.Completed
		}
	}
	input := captureInput(root, filepath.Join(".walden", "evidence", name+".json"), ledgerRequired, strict)
	snapshot.Inputs = append(snapshot.Inputs, input)
	snapshot.Ledger = evidence.Document{SchemaVersion: evidence.SchemaVersion, Feature: name, Tasks: map[string]evidence.Record{}}
	if input.Err != nil {
		snapshot.LedgerErr = fmt.Errorf("%s: %w", input.Path, input.Err)
	} else if input.Present {
		snapshot.Ledger, snapshot.LedgerErr = evidence.Decode(input.Data, name)
		if snapshot.LedgerErr != nil {
			snapshot.LedgerErr = fmt.Errorf("%s: %w", input.Path, snapshot.LedgerErr)
		}
	}
	return snapshot
}

func bindInput(ctx context.Context, root, commit string, input inputSnapshot) InputBinding {
	binding := InputBinding{Path: input.Path, State: "matched"}
	fail := func(state, detail string) InputBinding { binding.State, binding.Detail = state, detail; return binding }
	if input.Err != nil {
		return fail("unreadable", input.Err.Error())
	}
	if !input.Present && input.Required {
		return fail("missing", "required certification input is absent from the judged working set")
	}
	if commit == "" {
		return fail("unavailable", "no existing captured commit")
	}
	listing, err := evidence.Git(ctx, gitRunner, root, "ls-tree", "-z", commit, "--", input.Path)
	if err != nil || listing.ExitCode != 0 {
		return fail("unreadable", fmt.Sprintf("committed path unavailable: %v; %s", err, strings.TrimSpace(listing.Stderr)))
	}
	entries, err := evidence.ParseTree(listing.Stdout)
	if err != nil {
		return fail("unreadable", err.Error())
	}
	if len(entries) == 0 {
		if !input.Present {
			binding.State = "matched-absent"
			return binding
		}
		return fail("missing", "input is absent from the captured commit")
	}
	if len(entries) != 1 || entries[0].Path != input.Path {
		return fail("unreadable", "committed path lookup is ambiguous")
	}
	entry := entries[0]
	if entry.Kind != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
		return fail("unsupported-kind", "committed input is not a regular-file blob")
	}
	if !input.Present {
		return fail("different", "input exists in the captured commit but not in the judged working set")
	}
	blob, err := evidence.Git(ctx, gitRunner, root, "cat-file", "blob", entry.OID)
	if err != nil || blob.ExitCode != 0 {
		return fail("unreadable", fmt.Sprintf("committed blob unavailable: %v; %s", err, strings.TrimSpace(blob.Stderr)))
	}
	if !bytes.Equal(input.Data, []byte(blob.Stdout)) {
		return fail("different", "judged bytes differ from the captured commit (including metadata or checkout conversions)")
	}
	return binding
}

func bindPortfolio(ctx context.Context, root, commit string, selected []string) InputBinding {
	binding := InputBinding{Path: ".walden/specs", State: "matched"}
	if commit == "" {
		binding.State, binding.Detail = "unavailable", "no existing captured commit"
		return binding
	}
	listing, err := evidence.Git(ctx, gitRunner, root, "ls-tree", "-z", "-d", commit+":.walden/specs")
	if err != nil || listing.ExitCode != 0 {
		binding.State, binding.Detail = "unavailable", "committed portfolio directory is absent or unreadable"
		return binding
	}
	entries, err := evidence.ParseTree(listing.Stdout)
	if err != nil {
		binding.State, binding.Detail = "unreadable", err.Error()
		return binding
	}
	names := []string{}
	for _, entry := range entries {
		if entry.Kind == "tree" {
			names = append(names, entry.Path)
		}
	}
	sort.Strings(names)
	if !reflect.DeepEqual(names, selected) {
		binding.State, binding.Detail = "different", fmt.Sprintf("judged portfolio %v differs from committed feature inventory %v", selected, names)
	}
	return binding
}

func inputBlocker(binding InputBinding) string {
	return fmt.Sprintf("certification input %s: %s — %s; align and commit the selected inputs before certifying (--strict)", binding.Path, binding.State, binding.Detail)
}
