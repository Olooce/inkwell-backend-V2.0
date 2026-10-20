package main

import (
	"os/exec"
	"syscall"
)

// configureChildProcess puts the child in its own process group so the whole
// tree can be signalled at once, and asks the kernel to SIGTERM it if this
// process dies without running its shutdown path (panic, SIGKILL, IDE stop).
func configureChildProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGTERM,
	}
}

// terminateChild asks the child's process group to exit.
func terminateChild(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
}

// killChild force-kills the child's process group.
func killChild(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
