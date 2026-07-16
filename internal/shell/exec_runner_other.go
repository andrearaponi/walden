//go:build !unix

package shell

import "os/exec"

// configureProcessContainment is a no-op outside unix: context cancellation
// falls back to killing the direct process only. Released binaries target
// linux and darwin, both unix.
func configureProcessContainment(cmd *exec.Cmd) {}
