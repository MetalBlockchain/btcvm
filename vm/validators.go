// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"fmt"
	"time"

	"github.com/MetalBlockchain/metalgo/network/p2p"
)

const (
	// maxValidatorSetStaleness is the maximum age of the validator set
	// before it's considered stale and needs to be refreshed
	maxValidatorSetStaleness = 5 * time.Minute
)

// InitializeValidators creates a validator set for gossip targeting.
// This wraps the Avalanche validator state and provides stake-weighted
// peer selection for gossip operations.
func (vm *VM) InitializeValidators() (*p2p.Validators, error) {
	if vm.ctx == nil {
		return nil, fmt.Errorf("vm context not initialized")
	}

	if vm.ctx.ValidatorState == nil {
		return nil, fmt.Errorf("validator state not initialized")
	}

	// Get peers from the p2p network
	peers := vm.p2pNetwork.Peers

	p2pValidators := p2p.NewValidators(
		peers,
		vm.ctx.Log,
		vm.ctx.SubnetID,
		vm.ctx.ValidatorState,
		maxValidatorSetStaleness,
	)

	return p2pValidators, nil
}
