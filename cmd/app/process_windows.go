package main

import (
	"os/exec"
)

// configureChildProcess is a no-op on Windows; process groups and signals
// work differently there.
func configureChildProcess(cmd *exec.Cmd) {}

// terminateChild kills the child. Windows console processes cannot be sent
// SIGTERM, so there is no graceful variant.
func terminateChild(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}

// killChild force-kills the child.
func killChild(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
