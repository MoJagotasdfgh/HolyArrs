package main

// HolyArrs fork (MoJagotasdfgh/HolyArrs), patch 003: Windows service mode.
//
//	holyarrs.exe service -name <service name> -config <folder> [server flags]
//
// The Service Control Manager starts the program with these words (the
// service is created with them in its command line); started by hand it
// refuses. -config is required (the /config default means nothing on
// Windows), and with no -addr and no HOLYARRS_ADDR or HOLYARRS_PORT the
// server listens on 127.0.0.1 only. Stop and shutdown end the server
// cleanly; a restart or exit asked for from the controls ends the service
// with that code, so the service's recovery setting starts it again.

import (
	"errors"
	"os"
	"strings"
)

// forkServiceDefaultName is the service name when -name is not given.
const forkServiceDefaultName = "HolyArrs"

func init() {
	forkPatches = append(forkPatches, "003-windows-service")
	forkCommands["service"] = forkService
}

// forkServiceArgs takes -name out of args and returns the service name and
// the server flags, with -addr 127.0.0.1:<port> added when nothing sets the
// address.
func forkServiceArgs(args []string) (string, []string, error) {
	name := forkServiceDefaultName
	var rest []string
	hasConfig, hasAddr := false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-name" || a == "--name":
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" {
				return "", nil, errors.New("-name needs a service name")
			}
			name = args[i+1]
			i++
			continue
		case strings.HasPrefix(a, "-name=") || strings.HasPrefix(a, "--name="):
			name = a[strings.Index(a, "=")+1:]
			if strings.TrimSpace(name) == "" {
				return "", nil, errors.New("-name needs a service name")
			}
			continue
		}
		f := strings.TrimLeft(a, "-")
		hasConfig = hasConfig || f == "config" || strings.HasPrefix(f, "config=")
		hasAddr = hasAddr || f == "addr" || strings.HasPrefix(f, "addr=")
		rest = append(rest, a)
	}
	if !hasConfig {
		return "", nil, errors.New("-config <folder> is required for a service")
	}
	if !hasAddr && strings.TrimSpace(os.Getenv(EnvAddr)) == "" && strings.TrimSpace(os.Getenv(EnvPort)) == "" {
		rest = append([]string{"-addr", "127.0.0.1:" + defaultPort}, rest...)
	}
	return name, rest, nil
}

// forkExitCode turns serve's end into the program's exit code, as main does.
func forkExitCode(err error) int {
	var ex errAppExit
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errRestart):
		return exitRestart
	case errors.As(err, &ex):
		return ex.code
	}
	return 1
}
