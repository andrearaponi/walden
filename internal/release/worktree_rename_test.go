package release

import (
	"reflect"
	"testing"
)

// A rename carries its source as a field of its own, in whichever column git
// reports it. Read as an entry, the source would name a path that is not
// uncommitted and may not exist.
func TestPorcelainPathListSkipsTheSourceOfARename(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   []string
	}{
		{"plain entries", " M a.go\x00?? b.go\x00", []string{"a.go", "b.go"}},
		{"staged rename", "R  new.txt\x00old.txt\x00 M keep.txt\x00", []string{"new.txt", "keep.txt"}},
		{"staged rename, then modified", "RM new.txt\x00old.txt\x00", []string{"new.txt"}},
		{"rename in the working tree", " R new.txt\x00old.txt\x00 M keep.txt\x00", []string{"new.txt", "keep.txt"}},
		{"copy", "C  copy.txt\x00old.txt\x00", []string{"copy.txt"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := porcelainPathList(tc.status); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("paths = %q, want %q", got, tc.want)
			}
		})
	}
}
