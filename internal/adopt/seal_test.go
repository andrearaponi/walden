package adopt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

func TestAdoptSealShape(t *testing.T) {
	t.Run("seals absent fingerprints and repairs empty links", func(t *testing.T) {
		root := t.TempDir()
		preFingerprintDocs(t, root, "old-era")

		before, err := spec.LoadFeature(root, "old-era")
		if err != nil {
			t.Fatalf("load before: %v", err)
		}

		sealed, err := sealFeature(root, "old-era")
		if err != nil {
			t.Fatalf("sealFeature: %v", err)
		}
		if strings.Join(sealed, ",") != "requirements.md,design.md,tasks.md" {
			t.Fatalf("sealed = %v", sealed)
		}

		after, err := spec.LoadFeature(root, "old-era")
		if err != nil {
			t.Fatalf("load after: %v", err)
		}

		// Fingerprints stamped from the current bodies, path-aware.
		if !spec.BodyMatchesFingerprint(after.Requirements.Path, after.Requirements.Body, after.Requirements.ApprovedFingerprint) {
			t.Fatal("requirements seal does not match its body")
		}
		// Chain links joined to the upstream seals.
		if after.Design.SourceRequirementsFingerprint != after.Requirements.ApprovedFingerprint {
			t.Fatal("design source link not repaired to the requirements seal")
		}
		if after.Tasks.SourceDesignFingerprint != after.Design.ApprovedFingerprint {
			t.Fatal("tasks source link not repaired to the design seal")
		}
		// Bodies, statuses, and every timestamp untouched.
		for _, pair := range []struct{ before, after spec.Document }{
			{before.Requirements, after.Requirements},
			{before.Design, after.Design},
			{before.Tasks, after.Tasks},
		} {
			if pair.after.Body != pair.before.Body {
				t.Fatalf("seal changed a body: %s", pair.after.Path)
			}
			if pair.after.Status != pair.before.Status || pair.after.ApprovedAt != pair.before.ApprovedAt {
				t.Fatalf("seal changed status or approval timestamp: %s", pair.after.Path)
			}
			if pair.after.SourceRequirementsApprovedAt != pair.before.SourceRequirementsApprovedAt ||
				pair.after.SourceDesignApprovedAt != pair.before.SourceDesignApprovedAt {
				t.Fatalf("seal changed a source timestamp: %s", pair.after.Path)
			}
		}

		// Idempotence: nothing left to seal.
		again, err := sealFeature(root, "old-era")
		if err != nil {
			t.Fatalf("second sealFeature: %v", err)
		}
		if len(again) != 0 {
			t.Fatalf("second seal wrote %v, want nothing", again)
		}
	})

	t.Run("drafts and in-review documents stay byte-identical", func(t *testing.T) {
		root := t.TempDir()
		preFingerprintDocs(t, root, "mid-flight")
		draftPath := filepath.Join(root, ".walden", "specs", "mid-flight", "design.md")
		draft := "---\nstatus: draft\napproved_at: \nlast_modified: 2026-06-09T14:10:00Z\napproved_fingerprint: \nsource_requirements_approved_at: \nsource_requirements_fingerprint: \n---\n\n# Feature Design (draft)\n"
		if err := os.WriteFile(draftPath, []byte(draft), 0o644); err != nil {
			t.Fatalf("write draft: %v", err)
		}

		if _, err := sealFeature(root, "mid-flight"); err != nil {
			t.Fatalf("sealFeature: %v", err)
		}
		content, err := os.ReadFile(draftPath)
		if err != nil {
			t.Fatalf("read draft after: %v", err)
		}
		if string(content) != draft {
			t.Fatal("seal touched a draft document")
		}
	})

	t.Run("present fingerprints are never rewritten, wrong ones included", func(t *testing.T) {
		root := t.TempDir()
		sealedDocs(t, root, "drifted", true)
		path := filepath.Join(root, ".walden", "specs", "drifted", "requirements.md")
		content, _ := os.ReadFile(path)
		edited := strings.Replace(string(content), "# Requirements Document", "# Requirements Document\n\nEdited after approval.", 1)
		if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
			t.Fatalf("drift the doc: %v", err)
		}

		sealed, err := sealFeature(root, "drifted")
		if err != nil {
			t.Fatalf("sealFeature: %v", err)
		}
		if len(sealed) != 0 {
			t.Fatalf("seal wrote to a drifted feature: %v", sealed)
		}
		after, _ := os.ReadFile(path)
		if string(after) != edited {
			t.Fatal("seal modified a document with a present fingerprint")
		}
	})
}
