// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/MetalBlockchain/metalgo/vms/rpcchainvm"
	"github.com/spf13/cobra"

	"github.com/MetalBlockchain/btcvm/vm"

	log "github.com/inconshreveable/log15"
)

var (
	cfg *config
)

// btcvmMain is the real main function for btcvm. It is necessary to work around
// the fact that deferred functions do not run when os.Exit() is called.
func btcvmMain() error {
	// Load configuration and parse command line
	tcfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg = tcfg

	// Initialize logging
	if err := initLogging(cfg.LogLevel, cfg.LogDir); err != nil {
		return fmt.Errorf("failed to initialize logging: %w", err)
	}
	defer log.Info("Shutdown complete")

	// Show version at startup
	log.Info("Starting Bitcoin VM", "version", version())

	// Show configuration
	cfg.show()

	// Get interrupt channel for graceful shutdown
	interrupt := interruptListener()

	// Start profiling if requested
	stopProfiler, err := startProfiler(cfg.CPUProfile, cfg.MemProfile, cfg.HTTPProfile)
	if err != nil {
		log.Error("Failed to start profiler", "error", err)
		return err
	}
	defer stopProfiler()

	// Initialize security features (unveil/pledge on OpenBSD)
	if err := initSecurity(cfg.DataDir, cfg.LogDir); err != nil {
		log.Error("Failed to initialize security features", "error", err)
		return err
	}

	// Return now if an interrupt signal was triggered
	if interruptRequested(interrupt) {
		return nil
	}

	// Create context for VM
	ctx := context.Background()

	// Setup VM serving in a goroutine
	errChan := make(chan error, 1)
	go func() {
		log.Info("Starting btcvm RPC chain VM server")
		errChan <- rpcchainvm.Serve(ctx, &vm.VM{})
	}()

	// Wait for either interrupt or error
	select {
	case <-interrupt:
		log.Info("Received interrupt signal, shutting down gracefully")
		return nil
	case err := <-errChan:
		if err != nil {
			log.Error("RPC chain VM server error", "error", err)
			return err
		}
		return nil
	}
}

func main() {
	// Override GC percent if not explicitly set
	if os.Getenv("GOGC") == "" {
		// Set GC to run more frequently to avoid memory spikes
		// This value is optimized for blockchain processing
		debug.SetGCPercent(10)
	}

	// Use cobra for CLI but with enhanced initialization
	rootCmd := &cobra.Command{
		Use:   "btcvm",
		Short: "Bitcoin VM for Metal",
		Long:  "A Bitcoin Virtual Machine implementation running on Metal consensus",
		RunE:  runFunc,
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runFunc(*cobra.Command, []string) error {
	// Work around defer not working after os.Exit()
	if err := btcvmMain(); err != nil {
		return err
	}
	return nil
}

