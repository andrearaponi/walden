package app

import "fmt"

// Step is a single fail-fast unit of work in a command flow.
type Step struct {
	Name string
	Run  func() error
}

// ExecuteSteps runs each step in order and stops at the first failure.
func ExecuteSteps(steps ...Step) error {
	for _, step := range steps {
		if step.Run == nil {
			return fmt.Errorf("step %q failed: missing runner", step.Name)
		}

		if err := step.Run(); err != nil {
			return fmt.Errorf("step %q failed: %w", step.Name, err)
		}
	}

	return nil
}
