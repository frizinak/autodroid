//go:build linux

package adb

import "syscall"

func parentlessSysProc() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}
}
