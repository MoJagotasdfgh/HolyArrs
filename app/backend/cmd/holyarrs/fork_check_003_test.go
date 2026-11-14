package main

// HolyArrs fork, check for patch 003 (Windows service mode): the command is
// there, refuses to run by hand, needs -config and defaults to 127.0.0.1.

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestForkCheck003Args(t *testing.T) {
	t.Setenv(EnvAddr, "")
	t.Setenv(EnvPort, "")
	if _, _, err := forkServiceArgs([]string{"-name", "X"}); err == nil {
		t.Fatal("a service without -config was accepted")
	}
	name, rest, err := forkServiceArgs([]string{"-name", "HolyArrsMod", "-config", `D:\Media\config`})
	if err != nil || name != "HolyArrsMod" || !slices.Equal(rest, []string{"-addr", "127.0.0.1:" + defaultPort, "-config", `D:\Media\config`}) {
		t.Fatalf("%q %q %v", name, rest, err)
	}
	if c, err := parseFlags(rest, os.Stderr); err != nil || c.Addr != "127.0.0.1:"+defaultPort || c.ConfigDir != `D:\Media\config` {
		t.Fatalf("parsed %+v %v", c, err)
	}
	name, rest, _ = forkServiceArgs([]string{"-name=Other", "-config=C:\\c", "-addr", "127.0.0.1:7000"})
	if name != "Other" || slices.Contains(rest, "127.0.0.1:"+defaultPort) {
		t.Fatalf("%q %q", name, rest)
	}
	if name, _, _ := forkServiceArgs([]string{"-config", "c"}); name != forkServiceDefaultName {
		t.Fatalf("default name %q", name)
	}
}

func TestForkCheck003RefusesByHand(t *testing.T) {
	if _, ok := forkCommands["service"]; !ok {
		t.Fatal("service command missing")
	}
	if code, ok := forkCommand([]string{"service", "-config", t.TempDir()}); !ok || code != 2 {
		t.Fatalf("by hand: code %d ok %v, want 2", code, ok)
	}
	if code := forkExitCode(errRestart); code != exitRestart {
		t.Fatalf("restart code %d", code)
	}
}

func TestForkCheck003WindowsSide(t *testing.T) {
	src, err := os.ReadFile("fork_service_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"svc.Run(name, h)", "svc.IsWindowsService()", "svc.AcceptStop | svc.AcceptShutdown"} {
		if !strings.Contains(string(src), want) {
			t.Fatalf("service mode lost %q", want)
		}
	}
}
