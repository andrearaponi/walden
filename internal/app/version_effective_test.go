package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestEffectiveVersionResolution(t *testing.T) {
	cases := []struct {
		name         string
		ldflags      string
		buildVersion string
		want         string
	}{
		{
			name:         "ldflags version wins over build info",
			ldflags:      "v0.9.9",
			buildVersion: "v0.1.0",
			want:         "v0.9.9",
		},
		{
			name:         "go-install module version adopted when ldflags unset",
			ldflags:      "dev",
			buildVersion: "v0.6.0",
			want:         "v0.6.0",
		},
		{
			name:         "source build reports dev",
			ldflags:      "dev",
			buildVersion: "(devel)",
			want:         "dev",
		},
		{
			name:         "missing build info reports dev",
			ldflags:      "dev",
			buildVersion: "",
			want:         "dev",
		},
		{
			name:         "pseudo-version is not a release tag",
			ldflags:      "dev",
			buildVersion: "v0.6.1-0.20260709123456-abcdef123456",
			want:         "dev",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restoreVersion := Version
			restoreSeam := buildInfoVersion
			t.Cleanup(func() {
				Version = restoreVersion
				buildInfoVersion = restoreSeam
			})

			Version = tc.ldflags
			buildInfoVersion = func() string { return tc.buildVersion }

			if got := effectiveVersion(); got != tc.want {
				t.Fatalf("effectiveVersion() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEffectiveVersionBacksVersionCommand(t *testing.T) {
	restoreVersion := Version
	restoreSeam := buildInfoVersion
	t.Cleanup(func() {
		Version = restoreVersion
		buildInfoVersion = restoreSeam
	})

	Version = "dev"
	buildInfoVersion = func() string { return "v0.6.0" }

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"version"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "v0.6.0") {
		t.Fatalf("expected version output to adopt build info version, got %q", stdout.String())
	}
}
