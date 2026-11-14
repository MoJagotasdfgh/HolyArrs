package main

// HolyArrs fork, patch 000: runs a fork command (fork-info, service) before
// main(). Go runs a package's init functions in file-name order, so this
// file's name keeps it last: every other init of this package has run.
// Server flags and upstream's own commands never match, so they fall
// through to main() unchanged.

import "os"

func init() {
	if code, ok := forkCommand(os.Args[1:]); ok {
		os.Exit(code)
	}
}
