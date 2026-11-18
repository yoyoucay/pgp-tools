//go:build !windows

package main

import "os"

func attachConsole() {}

func isRoot() bool { return os.Geteuid() == 0 }
