package main

// HolyArrs fork (MoJagotasdfgh/HolyArrs), patch 000: the fork's own first
// words on the command line and its build stamp. zz_fork_init.go runs a
// fork command before main() (no edit to main.go: upstream caps its size);
// every later patch registers itself from init().

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/HolyArrs/holyarrs/app/backend/internal/sources"
)

// forkBuild is stamped by fork/apply.ps1: -X main.forkBuild=mod+r<recipe>.
var forkBuild = "unstamped"

// forkCommands are the fork's commands (fork-info, service, ...); each
// returns the exit code.
var forkCommands = map[string]func(args []string) int{}

// forkPatches names the patches compiled into this program.
var forkPatches = []string{"000-fork-base"}

func init() { forkCommands["fork-info"] = forkInfo }

// forkCommand runs a fork command when args start with one.
func forkCommand(args []string) (int, bool) {
	if len(args) == 0 {
		return 0, false
	}
	fn, ok := forkCommands[args[0]]
	if !ok {
		return 0, false
	}
	return fn(args[1:]), true
}

// forkInfo prints what this program is, for the recipe's checks and for the
// hub: version, build stamp, patches, commands and plugged-in site readers.
func forkInfo([]string) int {
	cmds := make([]string, 0, len(forkCommands))
	for k := range forkCommands {
		cmds = append(cmds, k)
	}
	sort.Strings(cmds)
	patches := append([]string(nil), forkPatches...)
	sort.Strings(patches)
	sites := make([]string, 0, len(sources.Sites))
	for _, s := range sources.Sites {
		sites = append(sites, s.ID)
	}
	out, err := json.Marshal(map[string]any{"version": version, "build": forkBuild, "patches": patches, "commands": cmds, "site_readers": sites})
	if err != nil {
		return 1
	}
	fmt.Fprintln(os.Stdout, string(out))
	return 0
}
