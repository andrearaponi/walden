package spec

import "testing"

func TestFingerprintGoldenVectors(t *testing.T) {
	// Known SHA-256 vectors pin the normalization contract: any future change
	// to normalization breaks these values and must be treated as breaking.
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "empty body",
			body: "",
			want: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name: "whitespace-only body normalizes to empty",
			body: "\n\n  \n",
			want: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name: "abc NIST vector",
			body: "abc",
			want: "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
		{
			name: "abc with surrounding whitespace",
			body: "\nabc\n",
			want: "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Fingerprint(test.body); got != test.want {
				t.Fatalf("Fingerprint(%q) = %q, want %q", test.body, got, test.want)
			}
		})
	}
}

func TestFingerprintLineEndingNormalization(t *testing.T) {
	reference := Fingerprint("# Title\n\nBody line one\nBody line two\n")

	tests := []struct {
		name string
		body string
	}{
		{"CRLF endings", "# Title\r\n\r\nBody line one\r\nBody line two\r\n"},
		{"CR endings", "# Title\r\rBody line one\rBody line two\r"},
		{"mixed endings", "# Title\r\n\nBody line one\rBody line two\r\n"},
		{"extra leading and trailing whitespace", "\n\n# Title\n\nBody line one\nBody line two\n\n\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Fingerprint(test.body); got != reference {
				t.Fatalf("Fingerprint(%q) = %q, want reference %q", test.body, got, reference)
			}
		})
	}
}

func TestFingerprintCheckboxStateInsensitivity(t *testing.T) {
	unchecked := "# Implementation Plan\n\n- [ ] 1. Objective\n  - [ ] 1.1 Step one\n  - [ ] 1.2 Step two\n"
	partiallyChecked := "# Implementation Plan\n\n- [ ] 1. Objective\n  - [x] 1.1 Step one\n  - [ ] 1.2 Step two\n"
	fullyChecked := "# Implementation Plan\n\n- [x] 1. Objective\n  - [x] 1.1 Step one\n  - [X] 1.2 Step two\n"

	reference := Fingerprint(unchecked)
	if got := Fingerprint(partiallyChecked); got != reference {
		t.Fatalf("partially checked plan fingerprint diverged: %q != %q", got, reference)
	}
	if got := Fingerprint(fullyChecked); got != reference {
		t.Fatalf("fully checked plan fingerprint diverged: %q != %q", got, reference)
	}
}

func TestFingerprintCheckboxNormalizationScope(t *testing.T) {
	// Only a line's leading task marker is normalized: inline bracket text
	// and non-list lines remain distinguishing content.
	if Fingerprint("verify the [x] flag") == Fingerprint("verify the [ ] flag") {
		t.Fatal("inline bracket text must remain distinguishing content")
	}
	if Fingerprint("- [x] done") != Fingerprint("- [ ] done") {
		t.Fatal("leading task marker must be normalized")
	}
	if Fingerprint("note - [x] not a marker") == Fingerprint("note - [ ] not a marker") {
		t.Fatal("mid-line dashes must not be treated as task markers")
	}
}

func TestFingerprintIsDeterministic(t *testing.T) {
	body := "# Requirements\n\n1. `R1.AC1` WHEN x, the system SHALL y.\n"
	first := Fingerprint(body)
	for i := 0; i < 100; i++ {
		if got := Fingerprint(body); got != first {
			t.Fatalf("Fingerprint is not deterministic: %q != %q", got, first)
		}
	}
}

func TestFingerprintDistinguishesContent(t *testing.T) {
	if Fingerprint("alpha") == Fingerprint("beta") {
		t.Fatal("different bodies produced the same fingerprint")
	}
}

func TestFingerprintValidation(t *testing.T) {
	valid := Fingerprint("any content")

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"computed fingerprint", valid, true},
		{"empty string", "", false},
		{"missing prefix", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", false},
		{"wrong prefix", "sha512:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", false},
		{"short digest", "sha256:e3b0c442", false},
		{"uppercase hex", "sha256:E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855", false},
		{"trailing garbage", "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855x", false},
		{"non-hex characters", "sha256:z3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85z", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ValidFingerprint(test.value); got != test.want {
				t.Fatalf("ValidFingerprint(%q) = %t, want %t", test.value, got, test.want)
			}
		})
	}
}

func TestFingerprintBodyMatch(t *testing.T) {
	body := "# Design\n\nContent under approval.\n"
	recorded := Fingerprint(body)

	tests := []struct {
		name        string
		body        string
		fingerprint string
		want        bool
	}{
		{"matching body", body, recorded, true},
		{"matching body with CRLF endings", "# Design\r\n\r\nContent under approval.\r\n", recorded, true},
		{"edited body", body + "\nInjected line.\n", recorded, false},
		{"malformed fingerprint fails closed", body, "sha256:not-a-digest", false},
		{"empty fingerprint fails closed", body, "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := BodyMatchesFingerprint(test.body, test.fingerprint); got != test.want {
				t.Fatalf("BodyMatchesFingerprint(%q, %q) = %t, want %t", test.body, test.fingerprint, got, test.want)
			}
		})
	}
}
