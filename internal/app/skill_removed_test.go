package app

import (
	"bytes"
	"strings"
	"testing"
)

// The binary carries no knowledge of the skill: `walden skill ...` is an
// unrecognized word like any other, and no usage or registry entry names it.
func TestSkillRemoved(t *testing.T) {
	cases := [][]string{
		{"skill"},
		{"skill", "install", "claude", "--project"},
		{"skill", "uninstall", "codex"},
		{"skill", "status", "--json"},
		{"skill", "show"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(args, &stdout, &stderr)

			control := append([]string{"no-such-command"}, args[1:]...)
			var ctrlOut, ctrlErr bytes.Buffer
			ctrlCode := Run(control, &ctrlOut, &ctrlErr)

			if code != ctrlCode || code == 0 {
				t.Fatalf("exit %d, control exit %d; want equal non-zero", code, ctrlCode)
			}
			if stdout.Len() != 0 || ctrlOut.Len() != 0 {
				t.Fatalf("stdout must be empty: %q / %q", stdout.String(), ctrlOut.String())
			}
			if !strings.HasPrefix(stderr.String(), "unknown command: "+strings.Join(args, " ")+"\n") {
				t.Fatalf("stderr does not start with the unknown-command line: %q", stderr.String())
			}
			// Same shape after the first line: the shared usage text.
			tail := func(s string) string {
				_, rest, _ := strings.Cut(s, "\n")
				return rest
			}
			if tail(stderr.String()) != tail(ctrlErr.String()) {
				t.Fatalf("usage tail differs from control:\n%s\n---\n%s", stderr.String(), ctrlErr.String())
			}
		})
	}

	t.Run("usage", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"--help"}, &stdout, &stderr); code != 0 {
			t.Fatalf("--help exited %d: %s", code, stderr.String())
		}
		for _, line := range strings.Split(stdout.String(), "\n") {
			fields := strings.Fields(line)
			if len(fields) > 0 && fields[0] == "skill" {
				t.Fatalf("--help still lists a skill command: %q", line)
			}
		}
		for _, spec := range commandRegistry {
			if spec.Path == "skill" || strings.HasPrefix(spec.Path, "skill ") {
				t.Fatalf("command registry still carries %q", spec.Path)
			}
		}
	})
}
