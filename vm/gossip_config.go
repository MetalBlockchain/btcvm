// Copyright (C) 2024-2025, Metallicus, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"fmt"
	"time"
)

// GossipConfig contains all configuration parameters for the gossip system
type GossipConfig struct {
	// Push Gossip Parameters
	//
	// PushGossipPercentStake is the percentage of validator stake to push gossip to [0-1]
	// Default: 0.9 (90% of validator stake)
	PushGossipPercentStake float64

	// PushGossipNumValidators is the maximum number of validators to push gossip to
	// Default: 100
	PushGossipNumValidators int

	// PushGossipNumPeers is the maximum number of non-validator peers to push gossip to
	// Default: 0
	PushGossipNumPeers int

	// PushGossipFrequency is how often to push gossip
	// Default: 100ms
	PushGossipFrequency time.Duration

	// Pull Gossip Parameters
	//
	// PullGossipFrequency is how often to pull gossip from peers
	// Default: 1s
	PullGossipFrequency time.Duration

	// Regossip Parameters
	//
	// PushRegossipNumValidators is the number of validators to regossip to
	// Default: 10
	PushRegossipNumValidators int

	// PushRegossipNumPeers is the number of non-validator peers to regossip to
	// Default: 0
	PushRegossipNumPeers int

	// RegossipFrequency is how often to regossip known items
	// Default: 30s
	RegossipFrequency time.Duration

	// Bloom Filter Parameters
	//
	// BloomFilterSize is the target number of elements in the bloom filter
	// Default: 8192
	BloomFilterSize int

	// BloomFalsePositiveRate is the target false positive rate for the bloom filter
	// Default: 0.01 (1%)
	BloomFalsePositiveRate float64

	// BloomResetThreshold is the false positive rate that triggers a bloom filter reset
	// Default: 0.05 (5%)
	BloomResetThreshold float64
}

// DefaultGossipConfig returns production-ready defaults matching subnet-evm/coreth
func DefaultGossipConfig() GossipConfig {
	return GossipConfig{
		// Push Gossip - Fast propagation
		PushGossipPercentStake:  0.9,  // 90% of validator stake
		PushGossipNumValidators: 100,  // Up to 100 validators
		PushGossipNumPeers:      0,    // No non-validator peers by default
		PushGossipFrequency:     100 * time.Millisecond,

		// Pull Gossip - Reliability and gap-filling
		PullGossipFrequency: 1 * time.Second,

		// Regossip - Ensure network-wide propagation
		PushRegossipNumValidators: 10,
		PushRegossipNumPeers:      0,
		RegossipFrequency:         30 * time.Second,

		// Bloom Filter - Efficient duplicate detection
		BloomFilterSize:        8192, // 8K elements
		BloomFalsePositiveRate: 0.01, // 1% FP rate
		BloomResetThreshold:    0.05, // Reset at 5% FP
	}
}

// Validate checks if the gossip configuration is valid
func (c *GossipConfig) Validate() error {
	if c.PushGossipPercentStake < 0 || c.PushGossipPercentStake > 1 {
		return fmt.Errorf("push gossip percent stake must be between 0 and 1, got %f", c.PushGossipPercentStake)
	}

	if c.PushGossipNumValidators < 0 {
		return fmt.Errorf("push gossip num validators must be non-negative, got %d", c.PushGossipNumValidators)
	}

	if c.PushGossipNumPeers < 0 {
		return fmt.Errorf("push gossip num peers must be non-negative, got %d", c.PushGossipNumPeers)
	}

	if c.PushGossipFrequency <= 0 {
		return fmt.Errorf("push gossip frequency must be positive, got %s", c.PushGossipFrequency)
	}

	if c.PullGossipFrequency <= 0 {
		return fmt.Errorf("pull gossip frequency must be positive, got %s", c.PullGossipFrequency)
	}

	if c.PushRegossipNumValidators < 0 {
		return fmt.Errorf("push regossip num validators must be non-negative, got %d", c.PushRegossipNumValidators)
	}

	if c.PushRegossipNumPeers < 0 {
		return fmt.Errorf("push regossip num peers must be non-negative, got %d", c.PushRegossipNumPeers)
	}

	if c.RegossipFrequency <= 0 {
		return fmt.Errorf("regossip frequency must be positive, got %s", c.RegossipFrequency)
	}

	if c.BloomFilterSize <= 0 {
		return fmt.Errorf("bloom filter size must be positive, got %d", c.BloomFilterSize)
	}

	if c.BloomFalsePositiveRate <= 0 || c.BloomFalsePositiveRate >= 1 {
		return fmt.Errorf("bloom false positive rate must be between 0 and 1, got %f", c.BloomFalsePositiveRate)
	}

	if c.BloomResetThreshold <= 0 || c.BloomResetThreshold >= 1 {
		return fmt.Errorf("bloom reset threshold must be between 0 and 1, got %f", c.BloomResetThreshold)
	}

	return nil
}
