// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package btcvm

import (
	"github.com/MetalBlockchain/metalgo/snow"
	"github.com/MetalBlockchain/metalgo/vms"

	"github.com/MetalBlockchain/btcvm/vm"
)

var _ vms.Factory = &Factory{}

// Factory implements the vms.Factory interface
type Factory struct{}

// New returns a new Bitcoin VM instance
func (f *Factory) New(*snow.Context) (interface{}, error) {
	return &vm.VM{}, nil
}