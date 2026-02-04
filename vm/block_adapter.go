package vm

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/MetalBlockchain/metalgo/ids"
	"github.com/MetalBlockchain/btcvm/btcd/blockchain"
	"github.com/MetalBlockchain/btcvm/btcd/btcutil"
	"github.com/MetalBlockchain/btcvm/btcd/chaincfg/chainhash"
	"github.com/MetalBlockchain/btcvm/btcd/wire"
	"go.uber.org/zap"
)

// BlockAdapter wraps a Bitcoin block and implements the snowman.Block interface
type BlockAdapter struct {
	vm        *VM
	btcBlock  *btcutil.Block
	id        ids.ID
	parentID  ids.ID
	height    uint64
	timestamp time.Time
	bytes     []byte
}

// NewBlockAdapter creates a new block adapter from a Bitcoin block
func NewBlockAdapter(vm *VM, btcBlock *btcutil.Block) (*BlockAdapter, error) {
	// Convert block hash to Metal ID
	blockHash := btcBlock.Hash()
	id := hashToID(blockHash)

	// Get parent block hash
	msgBlock := btcBlock.MsgBlock()
	parentHash := &msgBlock.Header.PrevBlock
	parentID := hashToID(parentHash)

	// Get block height
	height := uint64(btcBlock.Height())

	// Get timestamp
	timestamp := msgBlock.Header.Timestamp

	// Serialize block to bytes (use btcutil.Block's serialization)
	bytes, err := btcBlock.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize block: %w", err)
	}

	return &BlockAdapter{
		vm:        vm,
		btcBlock:  btcBlock,
		id:        id,
		parentID:  parentID,
		height:    height,
		timestamp: timestamp,
		bytes:     bytes,
	}, nil
}

// NewBlockAdapterFromHash fetches a block by hash and creates an adapter
func NewBlockAdapterFromHash(vm *VM, hash *chainhash.Hash) (*BlockAdapter, error) {
	// Get block from blockchain (returns *btcutil.Block)
	// Use BlockByHashAny to retrieve blocks from any chain (main or side)
	block, err := vm.chain.BlockByHashAny(hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get block by hash: %w", err)
	}

	return NewBlockAdapter(vm, block)
}

// NewBlockAdapterFromID fetches a block by Metal ID and creates an adapter
func NewBlockAdapterFromID(vm *VM, blockID ids.ID) (*BlockAdapter, error) {
	// Convert Metal ID to Bitcoin hash
	hash := idToHash(blockID)

	return NewBlockAdapterFromHash(vm, hash)
}

// NewBlockAdapterFromBytes deserializes a block from bytes and processes it through btcd
func NewBlockAdapterFromBytes(vm *VM, blockBytes []byte) (*BlockAdapter, error) {
	// Deserialize the Bitcoin block from bytes
	var msgBlock wire.MsgBlock
	reader := bytes.NewReader(blockBytes)
	err := msgBlock.BtcDecode(reader, 0, wire.WitnessEncoding)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize block: %w", err)
	}

	// Wrap in btcutil.Block
	block := btcutil.NewBlock(&msgBlock)
	blockHash := block.Hash()

	vm.ctx.Log.Info("Deserialized block from bytes",
		zap.String("blockHash", blockHash.String()))

	// Process the block through btcd's validation and storage pipeline
	// This ensures the block is validated and stored in the database
	isMainChain, isOrphan, err := vm.chain.ProcessBlock(block, blockchain.BFNone)
	if err != nil {
		vm.ctx.Log.Error("Failed to process parsed block",
			zap.String("blockHash", blockHash.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to process block: %w", err)
	}

	vm.ctx.Log.Info("Processed parsed block through btcd",
		zap.String("blockHash", blockHash.String()),
		zap.Bool("isMainChain", isMainChain),
		zap.Bool("isOrphan", isOrphan),
	)

	// Now create the adapter using the stored block
	// Use BlockByHashAny to retrieve it (works for main and side chains)
	storedBlock, err := vm.chain.BlockByHashAny(blockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve processed block: %w", err)
	}

	return NewBlockAdapter(vm, storedBlock)
}

// ID returns the block ID
func (b *BlockAdapter) ID() ids.ID {
	return b.id
}

// Parent returns the parent block ID
func (b *BlockAdapter) Parent() ids.ID {
	return b.parentID
}

// Height returns the block height
func (b *BlockAdapter) Height() uint64 {
	return b.height
}

// Timestamp returns the block timestamp
func (b *BlockAdapter) Timestamp() time.Time {
	return b.timestamp
}

// Bytes returns the serialized block bytes
func (b *BlockAdapter) Bytes() []byte {
	return b.bytes
}

// Verify verifies the block
func (b *BlockAdapter) Verify(ctx context.Context) error {
	// The block should already be validated by btcd when we retrieve it
	// from the blockchain. We could add additional validation here if needed.
	b.vm.ctx.Log.Debug("Block verified",
		zap.String("id", b.id.String()),
		zap.Uint64("height", b.height))
	return nil
}

// Accept accepts the block
func (b *BlockAdapter) Accept(ctx context.Context) error {
	b.vm.blocksMu.Lock()
	defer b.vm.blocksMu.Unlock()

	// Update last accepted
	b.vm.lastAccepted = b.id
	b.vm.preferred = b.id

	b.vm.ctx.Log.Info("Block accepted",
		zap.String("id", b.id.String()),
		zap.Uint64("height", b.height))

	// Note: Do NOT automatically signal block building here.
	// Block building should only be triggered by new transactions arriving via onTxAccepted(),
	// not by accepting blocks. This prevents spurious block building at startup.

	return nil
}

// Reject rejects the block
func (b *BlockAdapter) Reject(ctx context.Context) error {
	b.vm.ctx.Log.Info("Block rejected",
		zap.String("id", b.id.String()),
		zap.Uint64("height", b.height))
	return nil
}
