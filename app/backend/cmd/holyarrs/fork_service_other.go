//go:build !windows

package main

// HolyArrs fork, patch 003: service mode is for Windows (fork_service.go).

import (
	"fmt"
	"os"
)

func forkService([]string) int {
	fmt.Fprintln(os.Stderr, "service: Windows services only; leave out 'service' to run the server")
	return 2
}
