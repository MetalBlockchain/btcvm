// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/inconshreveable/log15"
)

// config defines the configuration options for btcvm
type config struct {
	// Logging
	LogLevel string
	LogDir   string

	// Profiling
	CPUProfile  string
	MemProfile  string
	HTTPProfile string

	// Paths
	DataDir string

	// Version
	ShowVersion bool
}

// defaultConfig returns a config with default values
func defaultConfig() *config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	defaultDataDir := filepath.Join(homeDir, ".btcvm")
	defaultLogDir := filepath.Join(defaultDataDir, "logs")

	return &config{
		LogLevel:    "info",
		LogDir:      defaultLogDir,
		DataDir:     defaultDataDir,
		CPUProfile:  "",
		MemProfile:  "",
		HTTPProfile: "",
		ShowVersion: false,
	}
}

// loadConfig loads the configuration from command line flags
func loadConfig() (*config, error) {
	cfg := defaultConfig()

	// Define flags
	flag.StringVar(&cfg.LogLevel, "loglevel", cfg.LogLevel, "Log level (trace, debug, info, warn, error, crit)")
	flag.StringVar(&cfg.LogDir, "logdir", cfg.LogDir, "Directory for log files")
	flag.StringVar(&cfg.DataDir, "datadir", cfg.DataDir, "Directory for data files")
	flag.StringVar(&cfg.CPUProfile, "cpuprofile", cfg.CPUProfile, "Write CPU profile to file")
	flag.StringVar(&cfg.MemProfile, "memprofile", cfg.MemProfile, "Write memory profile to file")
	flag.StringVar(&cfg.HTTPProfile, "httpprofile", cfg.HTTPProfile, "Enable HTTP profiling on port (e.g., 6060)")
	flag.BoolVar(&cfg.ShowVersion, "version", cfg.ShowVersion, "Show version and exit")

	// Parse flags
	flag.Parse()

	// Show version and exit if requested
	if cfg.ShowVersion {
		fmt.Printf("btcvm version %s\n", version())
		os.Exit(0)
	}

	// Validate configuration
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate checks that the configuration is valid
func (c *config) validate() error {
	// Validate log level
	validLevels := []string{"trace", "debug", "info", "warn", "error", "crit"}
	validLevel := false
	for _, level := range validLevels {
		if c.LogLevel == level {
			validLevel = true
			break
		}
	}
	if !validLevel {
		return fmt.Errorf("invalid log level: %s (valid: %v)", c.LogLevel, validLevels)
	}

	// Ensure directories exist
	dirs := []string{c.DataDir}
	if c.LogDir != "" {
		dirs = append(dirs, c.LogDir)
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// version returns the version string
func version() string {
	return "0.1.0"
}

// showConfig logs the current configuration
func (c *config) show() {
	log.Info("Configuration",
		"logLevel", c.LogLevel,
		"logDir", c.LogDir,
		"dataDir", c.DataDir,
		"cpuProfile", c.CPUProfile,
		"memProfile", c.MemProfile,
		"httpProfile", c.HTTPProfile,
	)
}
