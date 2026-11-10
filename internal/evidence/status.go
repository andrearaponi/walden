package evidence

import "strings"

// StatusEntry is one entry of `git status --porcelain -z`.
type StatusEntry struct {
	// Code holds the two status letters: the index column, then the
	// working-tree column.
	Code string
	Path string
	// Source is the path a rename or a copy came from, empty otherwise.
	Source string
}

// Renamed reports that the entry moved Source to Path, so Source is no
// longer in the working tree. A copy leaves its source where it was.
func (e StatusEntry) Renamed() bool {
	return e.Source != "" && strings.Contains(e.Code, "R")
}

// ParseStatus reads NUL-separated porcelain output. A rename or a copy is
// followed by a field naming its source, in whichever column git reports it:
// the index column, or the working-tree column for a path added with intent
// to add. That field belongs to its entry and is never an entry of its own.
func ParseStatus(porcelain string) []StatusEntry {
	fields := strings.Split(porcelain, "\x00")
	entries := make([]StatusEntry, 0, len(fields))
	for index := 0; index < len(fields); index++ {
		field := fields[index]
		if len(field) < 4 {
			continue
		}
		entry := StatusEntry{Code: field[:2], Path: field[3:]}
		if strings.ContainsAny(entry.Code, "RC") && index+1 < len(fields) {
			index++
			entry.Source = fields[index]
		}
		entries = append(entries, entry)
	}
	return entries
}
