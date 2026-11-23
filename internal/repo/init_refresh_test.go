package repo

import "testing"

// TestRefreshPin holds the pure function to its two promises: the version of
// every install target becomes the pin, and no other byte changes. The
// expected workflow of each case is written out, never computed.
func TestRefreshPin(t *testing.T) {
	const target = "github.com/andrearaponi/walden/cmd/walden@"

	cases := []struct {
		name      string
		workflow  string
		pin       string
		want      string
		wantFound bool
	}{
		{
			name:      "release version to a newer one",
			workflow:  "        run: go install " + target + "v0.7.0\n",
			pin:       "v0.7.1",
			want:      "        run: go install " + target + "v0.7.1\n",
			wantFound: true,
		},
		{
			name:      "latest to a release version",
			workflow:  "run: go install " + target + "latest\n",
			pin:       "v0.7.1",
			want:      "run: go install " + target + "v0.7.1\n",
			wantFound: true,
		},
		{
			name:      "release version to latest",
			workflow:  "run: go install " + target + "v0.7.1\n",
			pin:       "latest",
			want:      "run: go install " + target + "latest\n",
			wantFound: true,
		},
		{
			name:      "git-describe string",
			workflow:  "run: go install " + target + "v0.7.1-2-gc360063\n",
			pin:       "v0.7.2",
			want:      "run: go install " + target + "v0.7.2\n",
			wantFound: true,
		},
		{
			name:      "pseudo-version",
			workflow:  "run: go install " + target + "v0.7.2-0.20260711060210-abcdef123456\n",
			pin:       "v0.8.0",
			want:      "run: go install " + target + "v0.8.0\n",
			wantFound: true,
		},
		{
			name:      "branch name with a slash",
			workflow:  "run: go install " + target + "release/next\n",
			pin:       "v0.8.0",
			want:      "run: go install " + target + "v0.8.0\n",
			wantFound: true,
		},
		{
			name:      "followed by a space and another command",
			workflow:  "run: go install " + target + "v0.7.0 && walden version\n",
			pin:       "v0.7.1",
			want:      "run: go install " + target + "v0.7.1 && walden version\n",
			wantFound: true,
		},
		{
			name:      "followed by a semicolon",
			workflow:  "run: go install " + target + "v0.7.0; walden version\n",
			pin:       "v0.7.1",
			want:      "run: go install " + target + "v0.7.1; walden version\n",
			wantFound: true,
		},
		{
			name:      "inside double quotes",
			workflow:  "run: \"go install " + target + "v0.7.0\"\n",
			pin:       "v0.7.1",
			want:      "run: \"go install " + target + "v0.7.1\"\n",
			wantFound: true,
		},
		{
			name:      "inside single quotes",
			workflow:  "run: 'go install " + target + "v0.7.0'\n",
			pin:       "v0.7.1",
			want:      "run: 'go install " + target + "v0.7.1'\n",
			wantFound: true,
		},
		{
			name:      "inside parentheses",
			workflow:  "run: (go install " + target + "v0.7.0)\n",
			pin:       "v0.7.1",
			want:      "run: (go install " + target + "v0.7.1)\n",
			wantFound: true,
		},
		{
			name:      "followed by a carriage return",
			workflow:  "run: go install " + target + "v0.7.0\r\n      - name: Next\r\n",
			pin:       "v0.7.1",
			want:      "run: go install " + target + "v0.7.1\r\n      - name: Next\r\n",
			wantFound: true,
		},
		{
			name:      "followed by the full stop of a sentence",
			workflow:  "# This job installs " + target + "v0.7.0.\n",
			pin:       "v0.7.1",
			want:      "# This job installs " + target + "v0.7.1.\n",
			wantFound: true,
		},
		{
			name:      "at the end of the workflow, with no newline",
			workflow:  "run: go install " + target + "v0.7.0",
			pin:       "v0.7.1",
			want:      "run: go install " + target + "v0.7.1",
			wantFound: true,
		},
		{
			name:      "two targets",
			workflow:  "a: go install " + target + "v0.6.0\nb: go install " + target + "latest\n",
			pin:       "v0.7.1",
			want:      "a: go install " + target + "v0.7.1\nb: go install " + target + "v0.7.1\n",
			wantFound: true,
		},
		{
			name:      "already the pin",
			workflow:  "run: go install " + target + "v0.7.1\n",
			pin:       "v0.7.1",
			want:      "run: go install " + target + "v0.7.1\n",
			wantFound: true,
		},
		{
			name:      "no install target",
			workflow:  "run: python3 scripts/validate_walden_spec.py \"$feature\"\n",
			pin:       "v0.7.1",
			want:      "run: python3 scripts/validate_walden_spec.py \"$feature\"\n",
			wantFound: false,
		},
		{
			name:      "a version query is not an install target",
			workflow:  "run: go install " + target + ">=v1.2\n",
			pin:       "v0.7.1",
			want:      "run: go install " + target + ">=v1.2\n",
			wantFound: false,
		},
		{
			name:      "the module path with no version",
			workflow:  "run: go install " + target + "\n",
			pin:       "v0.7.1",
			want:      "run: go install " + target + "\n",
			wantFound: false,
		},
		{
			name:      "empty workflow",
			workflow:  "",
			pin:       "v0.7.1",
			want:      "",
			wantFound: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, found := refreshPin([]byte(tc.workflow), tc.pin)
			if string(got) != tc.want {
				t.Errorf("refreshed workflow:\n got %q\nwant %q", got, tc.want)
			}
			if found != tc.wantFound {
				t.Errorf("found = %v, want %v", found, tc.wantFound)
			}
		})
	}
}
