// Copyright (C) 2024-2025, Metallicus, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"sync"
	"time"

	"github.com/MetalBlockchain/btcvm/btcd/btcutil"
	"github.com/MetalBlockchain/btcvm/btcd/chaincfg/chainhash"
	"github.com/MetalBlockchain/metalgo/snow/engine/common"
	"go.uber.org/zap"
)

const (
	// txSubmitChannelSize is the size of the channel for transaction submission events
	txSubmitChannelSize = 1024

	// TargetBlockTime is the desired interval between blocks when transactions are pending
	TargetBlockTime = 2 * time.Second

	// RetryDelay is the minimum delay before retrying block building after a failed attempt
	RetryDelay = 100 * time.Millisecond
)

// blockBuilder manages the event-driven block building process.
// It monitors the mempool for pending transactions and signals
// when a block should be built.
type blockBuilder struct {
	// vm is the parent VM instance
	vm *VM

	// Synchronization
	lock          sync.Mutex
	pendingSignal *sync.Cond // Signal when transactions are pending

	// Transaction event channel
	txSubmitChan chan struct{}

	// State tracking
	hasPendingTxs bool // Whether mempool has pending txs
	shutdownChan  <-chan struct{}

	// Track last build time and parent for delay calculation
	buildBlockLock      sync.Mutex
	lastBuildTime       time.Time
	lastBuildParentHash chainhash.Hash
}

// newBlockBuilder creates a new block builder instance
func newBlockBuilder(vm *VM) *blockBuilder {
	b := &blockBuilder{
		vm:           vm,
		txSubmitChan: make(chan struct{}, txSubmitChannelSize),
		shutdownChan: vm.shutdownChan,
	}
	b.pendingSignal = sync.NewCond(&b.lock)
	return b
}

// start begins the block builder's goroutines
func (b *blockBuilder) start() {
	go b.awaitTxSubmissions()
}

// awaitTxSubmissions listens for transaction submission events
// from the mempool and signals when blocks should be built.
func (b *blockBuilder) awaitTxSubmissions() {
	for {
		select {
		case <-b.txSubmitChan:
			b.signalCanBuild()
		case <-b.shutdownChan:
			return
		}
	}
}

// onTxAccepted is called when a transaction is accepted into the mempool
func (b *blockBuilder) onTxAccepted(tx *btcutil.Tx) {
	b.vm.ctx.Log.Info("onTxAccepted called", zap.String("txHash", tx.Hash().String()))
	select {
	case b.txSubmitChan <- struct{}{}:
		b.vm.ctx.Log.Info("onTxAccepted sent signal to txSubmitChan")
	default:
		b.vm.ctx.Log.Info("onTxAccepted txSubmitChan full, signal dropped")
	}
}

// signalCanBuild marks that transactions are available and schedules block building
// It starts a goroutine that waits for the appropriate delay before notifying the engine
func (b *blockBuilder) signalCanBuild() {
	b.vm.ctx.Log.Info("signalCanBuild called - transactions are available")

	b.lock.Lock()
	alreadyPending := b.hasPendingTxs
	b.hasPendingTxs = true
	b.lock.Unlock()

	// If we already have a pending build scheduled, don't start another one
	if alreadyPending {
		b.vm.ctx.Log.Info("signalCanBuild: build already scheduled, skipping")
		return
	}

	b.pendingSignal.Broadcast()
	b.vm.ctx.Log.Info("signalCanBuild broadcasted to condition variable")

	// Start a goroutine to handle the delay and notify the engine
	go b.scheduleBlockBuild()
}

// scheduleBlockBuild waits for the appropriate delay and then notifies the engine to build a block
func (b *blockBuilder) scheduleBlockBuild() {
	b.vm.ctx.Log.Info("scheduleBlockBuild started")

	// Get current block to calculate delay
	currentBlock, err := b.vm.getCurrentBlock()
	if err != nil {
		b.vm.ctx.Log.Error("scheduleBlockBuild failed to get current block", zap.Error(err))
		b.lock.Lock()
		b.hasPendingTxs = false
		b.lock.Unlock()
		return
	}

	// Calculate delay based on last build time
	delay := b.calculateBuildingDelay(*currentBlock.Hash())
	b.vm.ctx.Log.Info("scheduleBlockBuild calculated delay", zap.Duration("delay", delay))

	// If delay is needed, wait for it
	if delay > 0 {
		b.vm.ctx.Log.Info("scheduleBlockBuild waiting for delay", zap.Duration("delay", delay))
		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			b.vm.ctx.Log.Info("scheduleBlockBuild delay elapsed")
		case <-b.shutdownChan:
			b.vm.ctx.Log.Info("scheduleBlockBuild cancelled due to shutdown")
			return
		}
	} else {
		b.vm.ctx.Log.Info("scheduleBlockBuild no delay needed")
	}

	// Check if we still need to build (transactions might have been included in another block)
	if !b.needToBuild() {
		b.vm.ctx.Log.Info("scheduleBlockBuild no transactions to build")
		b.lock.Lock()
		b.hasPendingTxs = false
		b.lock.Unlock()
		return
	}

	// Notify the engine to build a block
	b.vm.ctx.Log.Info("scheduleBlockBuild notifying engine")
	select {
	case b.vm.toEngine <- common.PendingTxs:
		b.vm.ctx.Log.Info("scheduleBlockBuild successfully notified engine")
	default:
		b.vm.ctx.Log.Warn("scheduleBlockBuild failed to notify engine (channel full)")
	}
}

// needToBuild returns true if there are pending transactions
func (b *blockBuilder) needToBuild() bool {
	mempool := b.vm.btcdAdapter.TxMemPool()
	if mempool == nil {
		return false
	}
	return len(mempool.MiningDescs()) > 0
}

// calculateBuildingDelay determines how long to wait before building the next block
func (b *blockBuilder) calculateBuildingDelay(currentBlockHash chainhash.Hash) time.Duration {
	b.buildBlockLock.Lock()
	defer b.buildBlockLock.Unlock()

	// If we've never built a block, no delay needed
	if b.lastBuildTime.IsZero() {
		b.vm.ctx.Log.Debug("first block build, no delay")
		return 0
	}

	// Check if this is a retry (same parent as last attempt)
	isRetry := b.lastBuildParentHash.IsEqual(&currentBlockHash)

	var nextBuildTime time.Time
	if isRetry {
		// Retry scenario: short delay
		nextBuildTime = b.lastBuildTime.Add(RetryDelay)
		b.vm.ctx.Log.Debug("retry detected, using retry delay")
	} else {
		// Normal scenario: target block time
		nextBuildTime = b.lastBuildTime.Add(TargetBlockTime)
		b.vm.ctx.Log.Debug("normal build, using target block time")
	}

	// Calculate remaining delay
	now := time.Now()
	remainingDelay := nextBuildTime.Sub(now)

	if remainingDelay < 0 {
		remainingDelay = 0
	}

	b.vm.ctx.Log.Debug("calculated building delay")

	return remainingDelay
}

// handleBuildAttempt records that we attempted to build a block
// Should be called by BuildBlock regardless of success/failure
func (b *blockBuilder) handleBuildAttempt(parentHash chainhash.Hash) {
	b.buildBlockLock.Lock()
	b.lastBuildTime = time.Now()
	b.lastBuildParentHash = parentHash
	b.buildBlockLock.Unlock()

	b.vm.ctx.Log.Debug("recorded build attempt")

	// Clear the pending flag and check if we need to schedule another build
	b.lock.Lock()
	b.hasPendingTxs = false
	b.lock.Unlock()

	// If there are still transactions in mempool, schedule another build
	if b.needToBuild() {
		b.vm.ctx.Log.Info("handleBuildAttempt: more transactions pending, scheduling next build")
		b.signalCanBuild()
	}
}

// waitForNeedToBuild blocks until transactions are pending
func (b *blockBuilder) waitForNeedToBuild(ctx context.Context) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	for !b.hasPendingTxs && !b.needToBuild() {
		b.vm.ctx.Log.Debug("no transactions in mempool, waiting for signal")

		// Create a channel that will be closed when the condition is signaled
		signaled := make(chan struct{})

		// Start a goroutine to wait on the condition variable
		go func() {
			b.lock.Lock()
			defer b.lock.Unlock()
			b.pendingSignal.Wait()
			close(signaled)
		}()

		// Temporarily release the lock so the goroutine can acquire it
		b.lock.Unlock()

		// Wait for either the signal or context cancellation
		select {
		case <-signaled:
			// Condition was signaled, re-acquire lock and check condition
			b.lock.Lock()
			// Continue loop to re-check the condition

		case <-ctx.Done():
			// Context was cancelled, broadcast to wake up the waiting goroutine
			b.pendingSignal.Broadcast()
			// Wait for the goroutine to finish to prevent leak
			<-signaled
			// Re-acquire lock before returning
			b.lock.Lock()
			return ctx.Err()

		case <-b.shutdownChan:
			// Shutdown requested, broadcast to wake up the waiting goroutine
			b.pendingSignal.Broadcast()
			// Wait for the goroutine to finish to prevent leak
			<-signaled
			// Re-acquire lock before returning
			b.lock.Lock()
			return context.Canceled
		}
	}

	b.vm.ctx.Log.Debug("transactions available in mempool")
	b.hasPendingTxs = false
	return nil
}

// waitForEvent waits for an event that requires block building
// and returns the appropriate message to the Snowman engine
func (b *blockBuilder) waitForEvent(ctx context.Context) (common.Message, error) {
	b.vm.ctx.Log.Info("waitForEvent starting - waiting for transactions")

	// STEP 1: Wait until transactions are available in mempool
	if err := b.waitForNeedToBuild(ctx); err != nil {
		b.vm.ctx.Log.Info("waitForEvent waitForNeedToBuild returned error", zap.Error(err))
		return 0, err
	}

	b.vm.ctx.Log.Info("waitForEvent transactions available, calculating delay")

	// STEP 2: Calculate delay based on last build time
	currentBlock, err := b.vm.getCurrentBlock()
	if err != nil {
		b.vm.ctx.Log.Error("failed to get current block", zap.Error(err))
		return 0, err
	}

	delay := b.calculateBuildingDelay(*currentBlock.Hash())
	b.vm.ctx.Log.Info("waitForEvent calculated delay", zap.Duration("delay", delay), zap.String("currentBlockHash", currentBlock.Hash().String()))

	// STEP 3: If no delay needed, return immediately
	if delay <= 0 {
		b.vm.ctx.Log.Info("waitForEvent no delay needed, returning PendingTxs immediately")
		return common.PendingTxs, nil
	}

	// STEP 4: Wait for delay period
	b.vm.ctx.Log.Info("waitForEvent waiting for delay period", zap.Duration("delay", delay))

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		b.vm.ctx.Log.Info("waitForEvent context cancelled", zap.Error(ctx.Err()))
		return 0, ctx.Err()
	case <-timer.C:
		b.vm.ctx.Log.Info("waitForEvent delay elapsed, returning PendingTxs")
		return common.PendingTxs, nil
	case <-b.shutdownChan:
		b.vm.ctx.Log.Info("waitForEvent shutdown signal received")
		return 0, context.Canceled
	}
}

// clearPendingSignal resets the pending transaction flag
// Called after a block is successfully built
func (b *blockBuilder) clearPendingSignal() {
	b.lock.Lock()
	b.hasPendingTxs = false
	b.lock.Unlock()
	b.vm.ctx.Log.Debug("cleared pending transaction signal")
}

