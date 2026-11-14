//go:build windows

package main

// HolyArrs fork, patch 003: the Windows side of service mode (fork_service.go).

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/sys/windows/svc"

	"github.com/HolyArrs/holyarrs/app/backend/internal/core/startup"
)

// forkStopWait is how long a stop waits for the server to end cleanly.
const forkStopWait = 25 * time.Second

func forkService(args []string) int {
	name, rest, err := forkServiceArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "service:", err)
		return 2
	}
	isService, err := svc.IsWindowsService()
	if err != nil || !isService {
		fmt.Fprintln(os.Stderr, "service: this is started by Windows services (sc start "+name+"), not by hand; leave out 'service' to run it in a window")
		return 2
	}
	if err := startup.StartRunAs(configDirArg(rest)); err != nil {
		log.Printf("holyarrs: %v", err)
		return 1
	}
	c, err := parseFlags(rest, os.Stderr)
	if err != nil {
		return 2
	}
	h := &forkServiceHandler{c: c}
	if err := svc.Run(name, h); err != nil {
		log.Printf("holyarrs: service %s: %v", name, err)
		return 1
	}
	return h.code
}

type forkServiceHandler struct {
	c    config
	code int
}

// Execute runs the server until Windows asks it to stop or it ends itself.
func (h *forkServiceHandler) Execute(_ []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	startup.UseMemoryMode()
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, h.c, runOpts{}) }()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case err := <-done:
			if err != nil {
				log.Printf("holyarrs: %v", err)
			}
			h.code = forkExitCode(err)
			status <- svc.Status{State: svc.StopPending}
			return h.code != 0, uint32(h.code)
		case r := <-req:
			switch r.Cmd {
			case svc.Interrogate:
				status <- r.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending, WaitHint: uint32(forkStopWait / time.Millisecond)}
				stop()
				select {
				case err := <-done:
					h.code = forkExitCode(err)
				case <-time.After(forkStopWait):
					log.Printf("holyarrs: did not stop within %s", forkStopWait)
					h.code = 1
				}
				return false, 0
			}
		}
	}
}
