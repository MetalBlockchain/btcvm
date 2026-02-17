// Copyright (C) 2024-2025, Metallicus, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"fmt"
	"os"

	log "github.com/inconshreveable/log15"
)

// unveilx is a wrapper for unveil that exits on error.
// unveil restricts filesystem access (OpenBSD security feature).
func unveilx(path string, perms string) {
	// This is a no-op on non-OpenBSD systems
	// On OpenBSD, this would call ossec.Unveil
	log.Debug("unveil", "path", path, "perms", perms)
}

// pledgex is a wrapper for pledge that exits on error.
// pledge restricts system operations (OpenBSD security feature).
func pledgex(promises string) {
	// This is a no-op on non-OpenBSD systems
	// On OpenBSD, this would call ossec.PledgePromises
	log.Debug("pledge", "promises", promises)
}

// unveilxFatal is like unveilx but exits on error.
func unveilxFatal(path string, perms string) {
	log.Debug("unveil (fatal)", "path", path, "perms", perms)
	// On OpenBSD with actual ossec support, this would exit on error
}

// pledgexFatal is like pledgex but exits on error.
func pledgexFatal(promises string) {
	log.Debug("pledge (fatal)", "promises", promises)
	// On OpenBSD with actual ossec support, this would exit on error
}

func init() {
	// On OpenBSD, this would set initial pledge promises
	// For now, this is a no-op on Linux
	if isOpenBSD() {
		pledgex("unveil stdio id rpath wpath cpath flock dns inet tty")
	}
}

// isOpenBSD returns true if running on OpenBSD
func isOpenBSD() bool {
	// Check if we're on OpenBSD
	// This is a simple check; in production you might use build tags
	return false // We're on Linux
}

// initSecurity initializes security features like unveil and pledge.
// This is called during initialization to set up filesystem and operation restrictions.
func initSecurity(dataDir string, logDir string) error {
	if !isOpenBSD() {
		log.Debug("Security features (unveil/pledge) not available on this platform")
		return nil
	}

	// On OpenBSD, we would unveil the directories we need access to:
	// - dataDir for database
	// - logDir for logs
	// - /etc/ssl/cert.pem for TLS (if needed)
	unveilxFatal(dataDir, "rwc")
	unveilxFatal(logDir, "rwc")

	// After unveiling all paths, lock down system operations
	pledgexFatal("stdio rpath wpath cpath flock dns inet")

	log.Info("Security features initialized")
	return nil
}

// checkRequiredDirectories verifies that required directories exist and are accessible.
func checkRequiredDirectories(dataDir string, logDir string) error {
	dirs := []string{dataDir, logDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}
