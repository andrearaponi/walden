package testutil

import (
	"context"
	"fmt"

	"github.com/andrearaponi/walden/internal/shell"
)

type Response = shell.Response

// Call records a single invocation against the fake runner.
type Call struct {
	Name string
	Args []string
}

// FakeRunner returns canned responses and records invocations for assertions.
type FakeRunner struct {
	responses []Response
	calls     []Call
}

// NewFakeRunner builds a fake runner with a deterministic response sequence.
func NewFakeRunner(responses ...Response) *FakeRunner {
	cloned := make([]Response, len(responses))
	copy(cloned, responses)

	return &FakeRunner{responses: cloned}
}

// Run records the invocation and returns the next configured response.
func (r *FakeRunner) Run(_ context.Context, name string, args ...string) (Response, error) {
	callArgs := make([]string, len(args))
	copy(callArgs, args)
	r.calls = append(r.calls, Call{Name: name, Args: callArgs})

	if len(r.responses) == 0 {
		err := fmt.Errorf("unexpected command: %s", name)
		return Response{ExitCode: 1, Err: err}, err
	}

	response := r.responses[0]
	r.responses = r.responses[1:]

	if response.Err != nil {
		return response, response.Err
	}

	return response, nil
}

// Calls returns a copy of the recorded invocation list.
func (r *FakeRunner) Calls() []Call {
	calls := make([]Call, len(r.calls))
	copy(calls, r.calls)
	return calls
}
