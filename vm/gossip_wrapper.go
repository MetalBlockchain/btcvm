// Copyright (C) 2024-2025, Metallicus, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/MetalBlockchain/btcvm/btcd/btcutil"
	"github.com/MetalBlockchain/btcvm/btcd/chaincfg/chainhash"
	"github.com/MetalBlockchain/metalgo/ids"
)

// BTCGossip wraps any Bitcoin data structure for gossip.
// This unified type allows a single gossip system to handle
// both transactions and blocks.
type BTCGossip struct {
	ItemType GossipItemType
	Tx       *btcutil.Tx     // non-nil if ItemType == GossipItemTypeTx
	Block    *btcutil.Block  // non-nil if ItemType == GossipItemTypeBlock
}

// GossipID returns the unique identifier for this gossip item.
// For transactions, this is the transaction hash.
// For blocks, this is the block hash.
func (g *BTCGossip) GossipID() ids.ID {
	switch g.ItemType {
	case GossipItemTypeTx:
		if g.Tx != nil {
			return hashToID(g.Tx.Hash())
		}
	case GossipItemTypeBlock:
		if g.Block != nil {
			return hashToID(g.Block.Hash())
		}
	}
	return ids.Empty
}

// Type returns the type of this gossip item
func (g *BTCGossip) Type() GossipItemType {
	return g.ItemType
}

// hashToID converts a Bitcoin chainhash.Hash to an Avalanche ids.ID
func hashToID(hash *chainhash.Hash) ids.ID {
	if hash == nil {
		return ids.Empty
	}
	return ids.ID(*hash)
}

// NewTxGossip creates a new BTCGossip wrapper for a transaction
func NewTxGossip(tx *btcutil.Tx) *BTCGossip {
	return &BTCGossip{
		ItemType: GossipItemTypeTx,
		Tx:       tx,
	}
}

// NewBlockGossip creates a new BTCGossip wrapper for a block
func NewBlockGossip(block *btcutil.Block) *BTCGossip {
	return &BTCGossip{
		ItemType: GossipItemTypeBlock,
		Block:    block,
	}
}
