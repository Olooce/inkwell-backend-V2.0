//go:build unix && !linux

package main

import (
	"os/exec"
	"syscall"
)

// configureChildProcess puts the child in its own process group so the whole
// tree can be signalled at once. Pdeathsig is Linux-only, so on other Unix
// systems children are only cleaned up via the normal shutdown path.
func configureChildProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateChild asks the child's process group to exit.
func terminateChild(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
}

// killChild force-kills the child's process group.
func killChild(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
