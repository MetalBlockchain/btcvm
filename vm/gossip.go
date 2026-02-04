// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/MetalBlockchain/btcvm/btcd/blockchain"
	"github.com/MetalBlockchain/btcvm/btcd/btcutil"
	"github.com/MetalBlockchain/btcvm/btcd/chaincfg/chainhash"
	"github.com/MetalBlockchain/btcvm/btcd/wire"
	"github.com/MetalBlockchain/metalgo/ids"
	"github.com/MetalBlockchain/metalgo/network/p2p/gossip"
	"go.uber.org/zap"
)

// Custom handler IDs for btcvm
// We use values that don't conflict with metalgo's predefined IDs
const (
	// BTCGossipHandlerID is the unified handler ID for both tx and block gossip
	// We start at 100 to avoid conflicts with metalgo's handler IDs (0-2)
	BTCGossipHandlerID = 100
)

// BTCGossipMarshaller implements Marshaller[BTCGossip] for unified gossip
type BTCGossipMarshaller struct{}

// MarshalGossip serializes a BTCGossip item to bytes
func (m *BTCGossipMarshaller) MarshalGossip(item *BTCGossip) ([]byte, error) {
	if item == nil {
		return nil, fmt.Errorf("nil gossip item")
	}

	var buf bytes.Buffer
	// Write type discriminator
	buf.WriteByte(byte(item.ItemType))

	switch item.ItemType {
	case GossipItemTypeTx:
		if item.Tx == nil {
			return nil, fmt.Errorf("nil transaction in gossip item")
		}
		msgTx := item.Tx.MsgTx()
		if err := msgTx.BtcEncode(&buf, 0, wire.WitnessEncoding); err != nil {
			return nil, fmt.Errorf("failed to encode tx: %w", err)
		}

	case GossipItemTypeBlock:
		if item.Block == nil {
			return nil, fmt.Errorf("nil block in gossip item")
		}
		msgBlock := item.Block.MsgBlock()
		if err := msgBlock.BtcEncode(&buf, 0, wire.WitnessEncoding); err != nil {
			return nil, fmt.Errorf("failed to encode block: %w", err)
		}

	default:
		return nil, fmt.Errorf("unknown gossip item type: %d", item.ItemType)
	}

	return buf.Bytes(), nil
}

// UnmarshalGossip deserializes bytes to a BTCGossip item
func (m *BTCGossipMarshaller) UnmarshalGossip(data []byte) (*BTCGossip, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty gossip data")
	}

	itemType := GossipItemType(data[0])
	buf := bytes.NewReader(data[1:])

	switch itemType {
	case GossipItemTypeTx:
		msgTx := wire.NewMsgTx(wire.TxVersion)
		if err := msgTx.BtcDecode(buf, 0, wire.WitnessEncoding); err != nil {
			return nil, fmt.Errorf("failed to decode tx: %w", err)
		}
		return &BTCGossip{
			ItemType: itemType,
			Tx:       btcutil.NewTx(msgTx),
		}, nil

	case GossipItemTypeBlock:
		msgBlock := &wire.MsgBlock{}
		if err := msgBlock.BtcDecode(buf, 0, wire.WitnessEncoding); err != nil {
			return nil, fmt.Errorf("failed to decode block: %w", err)
		}
		return &BTCGossip{
			ItemType: itemType,
			Block:    btcutil.NewBlock(msgBlock),
		}, nil

	default:
		return nil, fmt.Errorf("unknown gossip item type: %d", itemType)
	}
}

// UnifiedBTCSet manages gossiped items (transactions and blocks)
// Implements the gossip.Set[BTCGossip] interface
// Blocks are stored in btcd's database, not cached here
type UnifiedBTCSet struct {
	vm    *VM
	bloom *gossip.BloomFilter
	lock  sync.RWMutex
}

// NewUnifiedBTCSet creates a new unified set for gossiped items
func NewUnifiedBTCSet(vm *VM, bloom *gossip.BloomFilter) *UnifiedBTCSet {
	return &UnifiedBTCSet{
		vm:    vm,
		bloom: bloom,
	}
}

// Add adds a gossip item to the set and processes it
func (s *UnifiedBTCSet) Add(item *BTCGossip) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if item == nil {
		return fmt.Errorf("nil gossip item")
	}

	switch item.ItemType {
	case GossipItemTypeTx:
		if item.Tx == nil {
			return fmt.Errorf("nil transaction in gossip item")
		}

		txHash := item.Tx.Hash()
		s.vm.ctx.Log.Debug("UnifiedBTCSet.Add: received transaction",
			zap.String("txID", txHash.String()))

		// Check if already in mempool
		if s.vm.btcdAdapter.TxMemPool().HaveTransaction(txHash) {
			s.vm.ctx.Log.Debug("UnifiedBTCSet.Add: transaction already known",
				zap.String("txID", txHash.String()))
			s.bloom.Add(item)
			return nil
		}

		// Process the transaction
		acceptedTxs, err := s.vm.btcdAdapter.TxMemPool().ProcessTransaction(item.Tx, false, false, 0)
		if err != nil {
			s.vm.ctx.Log.Error("UnifiedBTCSet.Add: failed to process transaction",
				zap.String("txID", txHash.String()),
				zap.Error(err),
			)
			return err
		}

		s.vm.ctx.Log.Info("UnifiedBTCSet.Add: successfully processed transaction",
			zap.String("txID", txHash.String()),
			zap.Int("acceptedCount", len(acceptedTxs)),
		)

		// Add to bloom filter
		s.bloom.Add(item)

		// Re-gossip accepted transactions
		if len(acceptedTxs) > 0 && s.vm.btcdAdapter.OnTxRelay != nil {
			s.vm.btcdAdapter.OnTxRelay(acceptedTxs)
		}

	case GossipItemTypeBlock:
		if item.Block == nil {
			return fmt.Errorf("nil block in gossip item")
		}

		blockHash := item.Block.Hash()
		s.vm.ctx.Log.Debug("UnifiedBTCSet.Add: received block",
			zap.String("blockHash", blockHash.String()))
		if hasBlock, err := s.vm.chain.HaveBlock(blockHash); err != nil {
			s.vm.ctx.Log.Error("UnifiedBTCSet.Add: failed to check for existing block",
				zap.String("blockHash", blockHash.String()),
				zap.Error(err),
			)
			return err
		} else if hasBlock {
			s.vm.ctx.Log.Debug("UnifiedBTCSet.Add: block already known",
				zap.String("blockHash", blockHash.String()))
			s.bloom.Add(item)
			return nil
		}

		// Route through btcd's ProcessBlock for validation and storage
		// This ensures blocks are properly validated, stored in the database,
		// and added to the block index before being used by Snowman
		isMainChain, isOrphan, err := s.vm.chain.ProcessBlock(item.Block, blockchain.BFNone)
		if err != nil {
			s.vm.ctx.Log.Debug("UnifiedBTCSet.Add: failed to process block",
				zap.String("blockHash", blockHash.String()),
				zap.Error(err),
			)
			// Don't return error - block may be orphan or duplicate
			// Just log and continue
		} else {
			s.vm.ctx.Log.Info("UnifiedBTCSet.Add: processed block",
				zap.String("blockHash", blockHash.String()),
				zap.Bool("isMainChain", isMainChain),
				zap.Bool("isOrphan", isOrphan),
			)
		}

		// Add to bloom filter to track that we've seen this block
		s.bloom.Add(item)

		// Note: OnBlockRelay will be triggered automatically via blockchain
		// notifications when the block is connected to the chain

	default:
		return fmt.Errorf("unknown gossip item type: %d", item.ItemType)
	}

	return nil
}

// Has checks if the set contains an item with the given ID
func (s *UnifiedBTCSet) Has(id ids.ID) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	hash := idToHash(id)

	// Check mempool for transactions
	if s.vm.btcdAdapter.TxMemPool().HaveTransaction(hash) {
		return true
	}

	// Check btcd's block index for blocks (includes main and side chains)
	haveBlock, err := s.vm.chain.HaveBlock(hash)
	if err != nil {
		return false
	}
	return haveBlock
}

// Iterate iterates over all items in the set
func (s *UnifiedBTCSet) Iterate(f func(*BTCGossip) bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	s.vm.ctx.Log.Debug("UnifiedBTCSet.Iterate: iterating over gossiped items")

	// Iterate transactions from mempool
	txDescs := s.vm.btcdAdapter.TxMemPool().TxDescs()
	s.vm.ctx.Log.Debug("UnifiedBTCSet.Iterate: found transactions in mempool",
		zap.Int("count", len(txDescs)))

	for _, desc := range txDescs {
		item := &BTCGossip{
			ItemType: GossipItemTypeTx,
			Tx:       desc.Tx,
		}
		if !f(item) {
			s.vm.ctx.Log.Debug("UnifiedBTCSet.Iterate: iteration stopped early during tx iteration")
			return
		}
	}

	// Note: Blocks are NOT included in pull gossip iteration
	// Block propagation is handled exclusively via push gossip (OnBlockRelay)
	// and regossip (every 30s), which provides proper tracking and frequency limits.
	// Including blocks here caused continuous re-gossip as Iterate() creates
	// new BTCGossip objects that bypass the PushGossiper's tracking system.

	s.vm.ctx.Log.Debug("UnifiedBTCSet.Iterate: finished iterating")
}

// GetFilter returns the bloom filter and salt for this set
func (s *UnifiedBTCSet) GetFilter() ([]byte, []byte) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	bloom, salt := s.bloom.Marshal()
	return bloom, salt
}

// idToHash converts an Avalanche ids.ID to a Bitcoin chainhash.Hash
func idToHash(id ids.ID) *chainhash.Hash {
	hash, err := chainhash.NewHash(id[:])
	if err != nil {
		// Note: This is a best-effort conversion, returning empty hash on error
		return &chainhash.Hash{}
	}
	return hash
}
