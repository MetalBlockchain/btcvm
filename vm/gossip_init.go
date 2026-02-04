// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"fmt"

	"github.com/MetalBlockchain/metalgo/network/p2p/gossip"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// initializeGossip initializes the unified gossip system with both push and pull mechanisms
func (vm *VM) initializeGossip() error {
	vm.ctx.Log.Info("Initializing unified gossip system")

	// Create prometheus registry for gossip metrics
	reg := prometheus.NewRegistry()

	// Create bloom filter for tracking gossiped items
	bloom, err := gossip.NewBloomFilter(
		reg,
		"btc_gossip_bloom",
		vm.gossipConfig.BloomFilterSize,
		vm.gossipConfig.BloomFalsePositiveRate,
		vm.gossipConfig.BloomResetThreshold,
	)
	if err != nil {
		return fmt.Errorf("failed to create bloom filter: %w", err)
	}
	vm.ctx.Log.Debug("Created bloom filter for gossip",
		zap.Int("size", vm.gossipConfig.BloomFilterSize),
		zap.Float64("fpRate", vm.gossipConfig.BloomFalsePositiveRate),
	)

	// Create unified BTC set (handles both transactions and blocks)
	// Blocks are stored in btcd's database, not cached in memory
	btcSet := NewUnifiedBTCSet(vm, bloom)
	vm.btcSet = btcSet
	vm.ctx.Log.Debug("Created unified BTC set")

	// Create gossip metrics
	metrics, err := gossip.NewMetrics(reg, "btc_gossip")
	if err != nil {
		return fmt.Errorf("failed to create gossip metrics: %w", err)
	}
	vm.ctx.Log.Debug("Created gossip metrics")

	// Create the gossip handler that handles protobuf wrapping/unwrapping
	handler := gossip.NewHandler[*BTCGossip](
		vm.ctx.Log,
		&BTCGossipMarshaller{},
		btcSet,
		metrics,
		4*1024*1024, // 4MB target response size (accommodate both txs and blocks)
	)
	vm.ctx.Log.Debug("Created gossip handler")

	// Initialize validators for stake-weighted gossip
	if vm.p2pValidators == nil {
		vm.p2pValidators, err = vm.InitializeValidators()
		if err != nil {
			return fmt.Errorf("failed to initialize validators: %w", err)
		}
		vm.ctx.Log.Info("Initialized validator set for gossip")
	}

	// Create p2p client for gossip
	client := vm.p2pNetwork.NewClient(BTCGossipHandlerID)
	vm.ctx.Log.Debug("Created p2p client", zap.Uint64("handlerID", BTCGossipHandlerID))

	// Configure gossip parameters
	pushGossipParams := gossip.BranchingFactor{
		StakePercentage: vm.gossipConfig.PushGossipPercentStake,
		Validators:      vm.gossipConfig.PushGossipNumValidators,
		Peers:           vm.gossipConfig.PushGossipNumPeers,
	}
	pushRegossipParams := gossip.BranchingFactor{
		Validators: vm.gossipConfig.PushRegossipNumValidators,
		Peers:      vm.gossipConfig.PushRegossipNumPeers,
	}

	vm.ctx.Log.Info("Gossip parameters configured",
		zap.String("pushParams", fmt.Sprintf("%+v", pushGossipParams)),
		zap.String("regossipParams", fmt.Sprintf("%+v", pushRegossipParams)),
		zap.Duration("pushFreq", vm.gossipConfig.PushGossipFrequency),
		zap.Duration("pullFreq", vm.gossipConfig.PullGossipFrequency),
		zap.Duration("regossipFreq", vm.gossipConfig.RegossipFrequency),
	)

	// Create push gossiper
	pushGossiper, err := gossip.NewPushGossiper[*BTCGossip](
		&BTCGossipMarshaller{},
		btcSet,
		vm.p2pValidators,
		client,
		metrics,
		pushGossipParams,
		pushRegossipParams,
		1000,                                // discardedSize
		10,                                  // targetGossipSize
		vm.gossipConfig.RegossipFrequency,   // maxRegossipFrequency
	)
	if err != nil {
		return fmt.Errorf("failed to create push gossiper: %w", err)
	}
	vm.pushGossiper = pushGossiper
	vm.ctx.Log.Info("Created push gossiper successfully")

	// Create pull gossiper
	pullGossiper := gossip.NewPullGossiper[*BTCGossip](
		vm.ctx.Log,
		&BTCGossipMarshaller{},
		btcSet,
		client,
		metrics,
		10, // targetGossipSize
	)
	vm.pullGossiper = pullGossiper
	vm.ctx.Log.Info("Created pull gossiper successfully")

	// Register the gossip handler with the p2p network
	if err := vm.p2pNetwork.AddHandler(BTCGossipHandlerID, handler); err != nil {
		return fmt.Errorf("failed to register gossip handler: %w", err)
	}
	vm.ctx.Log.Info("Registered unified gossip handler",
		zap.Uint64("handlerID", BTCGossipHandlerID))

	return nil
}

// startGossipLoops starts the push and pull gossip goroutines
func (vm *VM) startGossipLoops() {
	vm.ctx.Log.Info("Starting gossip loops")

	// Start push gossip loop
	vm.shutdownWg.Add(1)
	go func() {
		defer vm.shutdownWg.Done()
		vm.ctx.Log.Info("Push gossip loop started",
			zap.Duration("frequency", vm.gossipConfig.PushGossipFrequency))
		gossip.Every(
			vm.gossipCtx,
			vm.ctx.Log,
			vm.pushGossiper,
			vm.gossipConfig.PushGossipFrequency,
		)
		vm.ctx.Log.Info("Push gossip loop stopped")
	}()

	// Start pull gossip loop
	vm.shutdownWg.Add(1)
	go func() {
		defer vm.shutdownWg.Done()
		vm.ctx.Log.Info("Pull gossip loop started",
			zap.Duration("frequency", vm.gossipConfig.PullGossipFrequency))
		gossip.Every(
			vm.gossipCtx,
			vm.ctx.Log,
			vm.pullGossiper,
			vm.gossipConfig.PullGossipFrequency,
		)
		vm.ctx.Log.Info("Pull gossip loop stopped")
	}()

	vm.ctx.Log.Info("Gossip loops started successfully",
		zap.Duration("pushFreq", vm.gossipConfig.PushGossipFrequency),
		zap.Duration("pullFreq", vm.gossipConfig.PullGossipFrequency),
	)
}
