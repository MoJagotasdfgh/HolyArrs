package main

// HolyArrs fork, check for patch 000 (fork base): the hook runs last and the
// fork's commands answer.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zz_fork_init.go runs the fork's commands and sorts after every other Go
// file here, so its init runs last.
func TestForkCheck000Hook(t *testing.T) {
	src, err := os.ReadFile("zz_fork_init.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "forkCommand(os.Args[1:])") {
		t.Fatal("zz_fork_init.go no longer calls forkCommand")
	}
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if !strings.HasSuffix(n, "_test.go") && n > "zz_fork_init.go" {
			t.Fatalf("%s sorts after zz_fork_init.go: its init would run after the fork command", n)
		}
	}
}

func TestForkCheck000Commands(t *testing.T) {
	if _, ok := forkCommand(nil); ok {
		t.Fatal("no words ran a fork command")
	}
	if _, ok := forkCommand([]string{"-config", "x"}); ok {
		t.Fatal("server flags ran a fork command")
	}
	if _, ok := forkCommands["fork-info"]; !ok {
		t.Fatal("fork-info missing")
	}
}
