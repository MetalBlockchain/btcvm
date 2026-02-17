// Copyright (C) 2024-2025, Metallicus, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package main

import (
	"os"
	"syscall"
)

func init() {
	interruptSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
}
