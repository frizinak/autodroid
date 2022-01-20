//go:build !windows && !linux

package adb

import "syscall"

func parentlessSysProc() *syscall.SysProcAttr { return nil }
