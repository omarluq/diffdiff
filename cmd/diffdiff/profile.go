package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "net/http/pprof" //nolint:gosec // diagnostic only: pprof is reachable solely when DIFFDIFF_PPROF is set, on a local server
)

// startProfiling launches an HTTP pprof server when the DIFFDIFF_PPROF
// environment variable holds a bind address (e.g. "localhost:6060"). It is a
// no-op otherwise, so normal runs carry no profiling cost. It exists to diagnose
// UI responsiveness: capture a CPU profile while reproducing the slowness, e.g.
//
//	DIFFDIFF_PPROF=localhost:6060 ./bin/diffdiff
//	# then, while clicking/resizing:
//	go tool pprof -top "http://localhost:6060/debug/pprof/profile?seconds=20"
//	go tool pprof -top "http://localhost:6060/debug/pprof/allocs"
func startProfiling() {
	addr := os.Getenv("DIFFDIFF_PPROF")
	if addr == "" {
		return
	}

	server := &http.Server{Addr: addr, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			_, _ = fmt.Fprintln(os.Stderr, err)
		}
	}()
	_, _ = fmt.Fprintf(os.Stderr, "pprof: listening on http://%s/debug/pprof/\n", addr)
}
