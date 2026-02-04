// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"

	log "github.com/inconshreveable/log15"
)

// startProfiler starts CPU and memory profiling if requested.
// Returns cleanup function that should be called on shutdown.
func startProfiler(cpuProfile string, memProfile string, httpProfile string) (func(), error) {
	cleanupFuncs := make([]func(), 0)

	// Start CPU profiling
	if cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			return nil, fmt.Errorf("failed to create CPU profile: %w", err)
		}

		if err := pprof.StartCPUProfile(f); err != nil {
			f.Close()
			return nil, fmt.Errorf("failed to start CPU profile: %w", err)
		}

		log.Info("CPU profiling enabled", "file", cpuProfile)

		cleanupFuncs = append(cleanupFuncs, func() {
			pprof.StopCPUProfile()
			f.Close()
			log.Info("CPU profile written", "file", cpuProfile)
		})
	}

	// Setup memory profiling
	if memProfile != "" {
		cleanupFuncs = append(cleanupFuncs, func() {
			f, err := os.Create(memProfile)
			if err != nil {
				log.Error("Failed to create memory profile", "error", err)
				return
			}
			defer f.Close()

			runtime.GC() // Force GC before writing profile
			if err := pprof.WriteHeapProfile(f); err != nil {
				log.Error("Failed to write memory profile", "error", err)
				return
			}

			log.Info("Memory profile written", "file", memProfile)
		})
	}

	// Start HTTP profiling server
	if httpProfile != "" {
		go func() {
			listenAddr := net.JoinHostPort("", httpProfile)
			log.Info("HTTP profiling server listening", "addr", listenAddr)

			profileRedirect := http.RedirectHandler("/debug/pprof", http.StatusSeeOther)
			http.Handle("/", profileRedirect)

			if err := http.ListenAndServe(listenAddr, nil); err != nil {
				log.Error("HTTP profiling server error", "error", err)
			}
		}()
	}

	// Return cleanup function
	return func() {
		for _, cleanup := range cleanupFuncs {
			cleanup()
		}
	}, nil
}
