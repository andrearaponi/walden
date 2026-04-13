package shell

import "context"

// Response captures the result of a shell command execution.
type Response struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

// Runner executes a command and returns a structured response.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (Response, error)
}
