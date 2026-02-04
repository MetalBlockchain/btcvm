// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

// GossipItemType represents the type of item being gossiped
type GossipItemType byte

const (
	// GossipItemTypeTx represents a transaction gossip item
	GossipItemTypeTx GossipItemType = 0x01

	// GossipItemTypeBlock represents a block gossip item
	GossipItemTypeBlock GossipItemType = 0x02
)
