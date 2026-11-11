package workflow

import (
	"reflect"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
)

// TestSelectProofs has one entry for each rule of the selection, in the
// order in which the rules apply.
func TestSelectProofs(t *testing.T) {
	attested := &evidence.AttestedCommit{Commit: "0123456789abcdef0123456789abcdef01234567"}
	entries := []evidence.TaskEvidence{
		{TaskID: "pending", State: evidence.StatePending},
		{TaskID: "pending-delivery", State: evidence.StatePending, Delivery: true},
		{TaskID: "delivered", State: evidence.StateDelivered, Delivery: true, Attestation: attested},
		{TaskID: "awaiting", State: evidence.StateStaleSpec, Delivery: true, AwaitsAdoption: true},
		{TaskID: "unattested", State: evidence.StateVerified, Delivery: true},
		{TaskID: "stale-delivery", State: evidence.StateStaleCode, Delivery: true},
		{TaskID: "verified", State: evidence.StateVerified},
		{TaskID: "stale", State: evidence.StateStaleCode},
		{TaskID: "failed", State: evidence.StateFailed},
		{TaskID: "unrecorded", State: evidence.StateUnrecorded},
	}

	cases := []struct {
		name   string
		all    bool
		task   string
		attest bool
		want   ProofSelection
	}{
		{
			name: "a default run on a tree with uncommitted paths",
			want: ProofSelection{
				Run:     []string{"stale-delivery", "stale", "failed", "unrecorded"},
				Skipped: []string{"unattested", "verified"},
				Kept:    []string{"delivered"},
				Left:    []string{"awaiting"},
			},
		},
		{
			// Rule 5: on a committed tree a delivery proof that holds no
			// attestation is proven although it reads verified.
			name: "a default run on a committed tree", attest: true,
			want: ProofSelection{
				Run:     []string{"unattested", "stale-delivery", "stale", "failed", "unrecorded"},
				Skipped: []string{"verified"},
				Kept:    []string{"delivered"},
				Left:    []string{"awaiting"},
			},
		},
		{
			name: "every task", all: true,
			want: ProofSelection{
				Run:  []string{"unattested", "stale-delivery", "verified", "stale", "failed", "unrecorded"},
				Kept: []string{"delivered"},
				Left: []string{"awaiting"},
			},
		},
		{
			name: "every task on a committed tree", all: true, attest: true,
			want: ProofSelection{
				Run:  []string{"unattested", "stale-delivery", "verified", "stale", "failed", "unrecorded"},
				Kept: []string{"delivered"},
				Left: []string{"awaiting"},
			},
		},
		{
			// A run that names a task executes that proof and nothing else,
			// also when its pass awaits adoption.
			name: "a named task whose pass awaits adoption", task: "awaiting", attest: true,
			want: ProofSelection{Run: []string{"awaiting"}},
		},
		{
			name: "a named verified task", task: "verified",
			want: ProofSelection{Run: []string{"verified"}},
		},
		{
			name: "a named pending task", task: "pending",
			want: ProofSelection{},
		},
	}
	for _, tc := range cases {
		got := SelectProofs(entries, tc.all, tc.task, tc.attest)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", tc.name, got, tc.want)
		}
	}

	// Without delivery tasks the selection is the one of today: verified
	// tasks skipped by default, everything else run.
	plain := []evidence.TaskEvidence{
		{TaskID: "1", State: evidence.StateVerified}, {TaskID: "2", State: evidence.StateStaleSpec},
		{TaskID: "3", State: evidence.StatePending}, {TaskID: "4", State: evidence.StateUnattested},
	}
	for _, attest := range []bool{false, true} {
		if got, want := SelectProofs(plain, false, "", attest), (ProofSelection{Run: []string{"2", "4"}, Skipped: []string{"1"}}); !reflect.DeepEqual(got, want) {
			t.Errorf("no delivery task, attest=%t: got %+v, want %+v", attest, got, want)
		}
	}
}
