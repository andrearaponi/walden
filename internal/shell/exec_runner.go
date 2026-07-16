package shell

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

type execRunner struct{}

// NewExecRunner creates the default local process runner for proof commands.
func NewExecRunner() Runner {
	return execRunner{}
}

// waitDelay bounds how long Run waits for the output pipes after the process
// exits or the context is canceled: a proof that leaves an orphan holding
// stdout must surface as an explicit error, never as a hang.
var waitDelay = 5 * time.Second

func (execRunner) Run(ctx context.Context, name string, args ...string) (Response, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	configureProcessContainment(cmd)
	cmd.WaitDelay = waitDelay

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	response := Response{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err == nil {
		response.ExitCode = 0
		return response, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		response.ExitCode = exitErr.ExitCode()
		return response, nil
	}

	response.ExitCode = 1
	response.Err = err
	return response, err
}
