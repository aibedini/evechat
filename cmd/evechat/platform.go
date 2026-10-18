package main

import (
	"os/exec"
	"runtime"
)

func runtimeGOOS() string   { return runtime.GOOS }
func runtimeGOARCH() string { return runtime.GOARCH }

// restartService restarts the systemd unit when one is installed.
func restartService() {
	if runtime.GOOS != "linux" {
		return
	}
	path, err := exec.LookPath("systemctl")
	if err != nil {
		return
	}
	if err := exec.Command(path, "restart", "evechat").Run(); err != nil {
		// Not fatal: the operator may run the binary manually.
		return
	}
}
