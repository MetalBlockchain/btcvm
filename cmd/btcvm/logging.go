// Copyright (C) 2024-2025, Metallicus, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	log "github.com/inconshreveable/log15"
)

// initLogging initializes the logging system with proper handlers.
// It sets up file logging if logDir is provided, otherwise uses stderr only.
func initLogging(logLevel string, logDir string) error {
	// Parse log level
	level, err := log.LvlFromString(logLevel)
	if err != nil {
		level = log.LvlInfo
		log.Warn("Invalid log level, defaulting to info", "requested", logLevel)
	}

	// Create handlers
	var handler log.Handler

	if logDir != "" {
		// Create log directory
		if err := os.MkdirAll(logDir, 0700); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		// Create log file path
		logFile := filepath.Join(logDir, "btcvm.log")

		// Try to open log file
		fileHandler, err := log.FileHandler(logFile, log.LogfmtFormat())
		if err != nil {
			log.Warn("Failed to create file logger, falling back to stderr", "error", err)
			handler = log.LvlFilterHandler(level, log.StderrHandler)
		} else {
			// Use both file and stderr
			handler = log.MultiHandler(
				log.LvlFilterHandler(level, log.StderrHandler),
				log.LvlFilterHandler(level, fileHandler),
			)
			log.Info("Logging to file", "path", logFile)
		}
	} else {
		// Just use stderr
		handler = log.LvlFilterHandler(level, log.StderrHandler)
	}

	// Set the handler
	log.Root().SetHandler(handler)

	log.Info("Logging initialized", "level", level.String())
	return nil
}

// setLogLevel changes the log level at runtime
func setLogLevel(level log.Lvl) {
	currentHandler := log.Root().GetHandler()
	log.Root().SetHandler(log.LvlFilterHandler(level, currentHandler))
	log.Info("Log level changed", "level", level.String())
}
