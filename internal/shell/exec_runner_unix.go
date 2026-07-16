//go:build unix

package shell

import (
	"os"
	"os/exec"
	"syscall"
)

// configureProcessContainment places the command in its own process group and
// makes context cancellation kill the whole group: proof steps spawn children
// (sh -c pipelines, test binaries), and killing only the direct child would
// leak them past a timeout.
func configureProcessContainment(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
}
