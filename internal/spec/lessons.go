package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var lessonPhases = []string{"requirements", "design", "tasks", "execute", "release"}

const lessonsHeader = `# Walden Lessons

Review this file before non-trivial work when the current request matches past mistakes, rejections, or validation failures.

## Lessons

<!-- Append entries with scripts/log_walden_lesson.py -->
`

// LessonEntry is the canonical structured payload for one Walden lesson.
type LessonEntry struct {
	Feature   string
	Phase     string
	Trigger   string
	Lesson    string
	Guardrail string
	LoggedAt  string
}

// AppendLesson validates, bootstraps, and appends one canonical lesson entry.
func AppendLesson(root string, entry LessonEntry) (LessonEntry, string, error) {
	normalized, err := normalizeLessonEntry(entry)
	if err != nil {
		return LessonEntry{}, "", err
	}

	lessonsPath := filepath.Join(root, ".walden", "lessons.md")
	if err := ensureLessonsFile(lessonsPath); err != nil {
		return LessonEntry{}, "", err
	}

	handle, err := os.OpenFile(lessonsPath, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return LessonEntry{}, "", fmt.Errorf("open lessons file for append: %w", err)
	}
	defer handle.Close()

	if _, err := handle.WriteString(formatLessonEntry(normalized)); err != nil {
		return LessonEntry{}, "", fmt.Errorf("append lesson entry: %w", err)
	}

	return normalized, lessonsPath, nil
}

func normalizeLessonEntry(entry LessonEntry) (LessonEntry, error) {
	feature, err := NormalizeFeatureName(entry.Feature)
	if err != nil {
		return LessonEntry{}, err
	}

	phase := strings.TrimSpace(strings.ToLower(entry.Phase))
	if phase == "" {
		return LessonEntry{}, fmt.Errorf("phase is required")
	}
	if !isAllowedLessonPhase(phase) {
		return LessonEntry{}, fmt.Errorf(`phase must be one of "requirements", "design", "tasks", "execute", "release"`)
	}

	trigger := strings.TrimSpace(entry.Trigger)
	if trigger == "" {
		return LessonEntry{}, fmt.Errorf("trigger is required")
	}

	lesson := strings.TrimSpace(entry.Lesson)
	if lesson == "" {
		return LessonEntry{}, fmt.Errorf("lesson is required")
	}

	guardrail := strings.TrimSpace(entry.Guardrail)
	if guardrail == "" {
		return LessonEntry{}, fmt.Errorf("guardrail is required")
	}

	loggedAt := strings.TrimSpace(entry.LoggedAt)
	if loggedAt == "" {
		loggedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	}

	return LessonEntry{
		Feature:   feature,
		Phase:     phase,
		Trigger:   trigger,
		Lesson:    lesson,
		Guardrail: guardrail,
		LoggedAt:  loggedAt,
	}, nil
}

func ensureLessonsFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect lessons file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create lessons directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(lessonsHeader), 0o644); err != nil {
		return fmt.Errorf("bootstrap lessons file: %w", err)
	}

	return nil
}

func formatLessonEntry(entry LessonEntry) string {
	return fmt.Sprintf(
		"### %s | %s | %s\n- Trigger: %s\n- Lesson: %s\n- Guardrail: %s\n\n",
		entry.LoggedAt,
		entry.Feature,
		entry.Phase,
		entry.Trigger,
		entry.Lesson,
		entry.Guardrail,
	)
}

func isAllowedLessonPhase(phase string) bool {
	for _, allowed := range lessonPhases {
		if phase == allowed {
			return true
		}
	}

	return false
}
