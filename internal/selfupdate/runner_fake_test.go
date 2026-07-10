package selfupdate

import (
	"context"

	"github.com/andrearaponi/walden/internal/shell"
)

// fakeRunner records every invocation and answers through a configurable
// respond function, standing in for the real exec-backed runner.
type fakeRunner struct {
	calls   [][]string
	respond func(name string, args []string) (shell.Response, error)
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (shell.Response, error) {
	call := append([]string{name}, args...)
	f.calls = append(f.calls, call)
	if f.respond == nil {
		return shell.Response{ExitCode: 0}, nil
	}
	return f.respond(name, args)
}
