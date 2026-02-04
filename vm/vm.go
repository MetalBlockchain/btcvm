package vm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	btcd "github.com/MetalBlockchain/btcvm/btcd"
	"github.com/MetalBlockchain/btcvm/btcd/blockchain"
	"github.com/MetalBlockchain/btcvm/btcd/btcutil"
	"github.com/MetalBlockchain/btcvm/btcd/mempool"
	"github.com/MetalBlockchain/metalgo/database"
	"github.com/MetalBlockchain/metalgo/ids"
	"github.com/MetalBlockchain/metalgo/network/p2p"
	"github.com/MetalBlockchain/metalgo/network/p2p/gossip"
	"github.com/MetalBlockchain/metalgo/snow"
	"github.com/MetalBlockchain/metalgo/snow/consensus/snowman"
	"github.com/MetalBlockchain/metalgo/snow/engine/common"
	"github.com/MetalBlockchain/metalgo/snow/engine/snowman/block"
	"github.com/MetalBlockchain/metalgo/version"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var (
	_ block.ChainVM = (*VM)(nil)

	errNotInitialized     = errors.New("VM not initialized")
	errAlreadyInitialized = errors.New("VM already initialized")
)

const (
	Name = "btcvm"
)

var Version = &version.Semantic{
	Major: 0,
	Minor: 0,
	Patch: 1,
}

// VM implements the Metal ChainVM interface for Bitcoin
type VM struct {
	// Metal context
	ctx      *snow.Context
	db       database.Database
	toEngine chan<- common.Message

	config *btcd.Config

	// btcd adapter (encapsulates blockchain, mempool, RPC, etc.)
	btcdAdapter *btcd.Server

	// Unified gossip system (replaces separate tx/block gossipers)
	gossipConfig  GossipConfig
	btcSet        *UnifiedBTCSet
	pushGossiper  *gossip.PushGossiper[*BTCGossip]
	pullGossiper  gossip.Gossiper
	p2pNetwork    *p2p.Network
	p2pValidators *p2p.Validators

	// Bitcoin components (legacy, kept for compatibility)
	chain *blockchain.BlockChain

	appSender common.AppSender

	// Block management
	preferred    ids.ID
	lastAccepted ids.ID
	blocksMu     sync.RWMutex

	// Block building
	buildBlockLock sync.Mutex
	blockBuilder   *blockBuilder
	builderLock    sync.Mutex

	// Lifecycle management for gossip goroutines
	cancel       context.CancelFunc
	gossipCtx    context.Context
	shutdownWg   sync.WaitGroup
	bootstrapped bool

	// Lifecycle
	initialized  bool
	stopped      bool
	shutdownChan chan struct{}
}

type genesisBytes struct {
	Config btcd.Config `json:"config"`
}

// parseGenesisBytes parses genesis bytes from JSON
func parseGenesisBytes(data []byte) (*genesisBytes, error) {
	if len(data) == 0 {
		return &genesisBytes{}, nil
	}

	var genesis genesisBytes
	if err := json.Unmarshal(data, &genesis); err != nil {
		return nil, fmt.Errorf("failed to unmarshal genesis bytes: %w", err)
	}

	return &genesis, nil
}

// Initialize initializes the VM
func (vm *VM) Initialize(
	ctx context.Context,
	snowCtx *snow.Context,
	db database.Database,
	genesisBytes []byte,
	upgradeBytes []byte,
	configBytes []byte,
	toEngine chan<- common.Message,
	_ []*common.Fx,
	appSender common.AppSender,
) error {
	// Store context first so we can use the logger
	vm.ctx = snowCtx

	vm.ctx.Log.Debug("entering Initialize")
	defer vm.ctx.Log.Debug("exiting Initialize")

	if vm.initialized {
		return errAlreadyInitialized
	}

	vm.db = db
	vm.toEngine = toEngine
	vm.appSender = appSender
	vm.shutdownChan = make(chan struct{})

	// Parse genesis to get config
	gb, err := parseGenesisBytes(genesisBytes)
	if err != nil {
		return fmt.Errorf("failed to parse genesis: %w", err)
	}

	config, _, err := btcd.LoadConfig(vm.ctx.NodeID.String(), &gb.Config)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Disable legacy networking
	config.DisableListen = true
	config.DisableDNSSeed = true
	config.MaxPeers = 0
	config.Upnp = false

	vm.config = config

	// Initialize gossip configuration with defaults
	vm.gossipConfig = DefaultGossipConfig()
	if err := vm.gossipConfig.Validate(); err != nil {
		return fmt.Errorf("invalid gossip config: %w", err)
	}

	vm.ctx.Log.Info("initializing Bitcoin VM",
		zap.String("network", config.ChainParams.Name),
		zap.String("dbPath", config.DataDir),
		zap.String("dbType", config.DbType),
		zap.String("gossipConfig", fmt.Sprintf("%+v", vm.gossipConfig)),
	)

	// Initialize btcd adapter (replaces btcd server)
	vm.ctx.Log.Info("Initializing btcd adapter with Metal database")

	btcdAdapter, err := btcd.BtcdMain(vm.config)
	if err != nil {
		return fmt.Errorf("failed to initialize btcd adapter: %w", err)
	}
	vm.btcdAdapter = btcdAdapter

	// Initialize block builder and set callback before starting server
	vm.blockBuilder = newBlockBuilder(vm)
	vm.btcdAdapter.SetOnTxAccepted(vm.blockBuilder.onTxAccepted)
	vm.btcdAdapter.Start()

	// Initialize p2p network
	vm.ctx.Log.Info("Initializing p2p network")
	reg := prometheus.NewRegistry()
	p2pNet, err := p2p.NewNetwork(vm.ctx.Log, appSender, reg, "p2p")
	if err != nil {
		return fmt.Errorf("failed to create p2p network: %w", err)
	}
	vm.p2pNetwork = p2pNet
	vm.ctx.Log.Info("p2p network initialized successfully")

	// Note: Unified gossip system will be initialized in onNormalOperationsStarted()
	// when SetState(snow.NormalOp) is called

	// Use btcd components for compatibility
	vm.chain = vm.btcdAdapter.Chain()
	vm.ctx.Log.Info("btcd adapter initialized successfully")

	// Get the latest block from the chain and set it as lastAccepted
	bestSnapshot := vm.chain.BestSnapshot()
	if bestSnapshot != nil {
		// Convert btcd hash to Metal ID
		vm.lastAccepted = hashToID(&bestSnapshot.Hash)
		vm.preferred = vm.lastAccepted
		vm.ctx.Log.Info("Set lastAccepted to best block",
			zap.Int32("height", bestSnapshot.Height),
			zap.String("hash", bestSnapshot.Hash.String()),
			zap.String("id", vm.lastAccepted.String()),
		)
	} else {
		vm.ctx.Log.Warn("No best block found, lastAccepted remains empty")
	}

	// Set the callback for relaying transactions via unified gossip
	vm.btcdAdapter.OnTxRelay = func(txns []*mempool.TxDesc) {
		for _, txD := range txns {
			// Use unified gossip if available
			if vm.pushGossiper != nil {
				item := NewTxGossip(txD.Tx)
				vm.pushGossiper.Add(item)
				vm.ctx.Log.Debug("Gossiped transaction via unified gossip",
					zap.String("hash", txD.Tx.Hash().String()))
			}
		}
	}

	// Set the callback for relaying blocks via unified gossip
	vm.btcdAdapter.OnBlockRelay = func(block *btcutil.Block) {
		// Run gossip asynchronously to avoid blocking block processing
		go func(b *btcutil.Block) {
			// Use unified gossip if available
			if vm.pushGossiper != nil {
				item := NewBlockGossip(b)

				// Check if we already gossiped this block to avoid continuous re-gossip
				// The bloom filter tracks blocks we've seen/gossiped
				if vm.btcSet != nil && vm.btcSet.bloom != nil {
					if vm.btcSet.bloom.Has(item) {
						vm.ctx.Log.Debug("Skipping block gossip - already in bloom filter",
							zap.String("hash", b.Hash().String()),
							zap.Int32("height", b.Height()),
						)
						return
					}
				}

				vm.pushGossiper.Add(item)
				vm.ctx.Log.Info("Gossiped block via unified gossip",
					zap.String("hash", b.Hash().String()),
					zap.Int32("height", b.Height()))
			}
		}(block)
	}

	vm.initialized = true

	vm.ctx.Log.Info("Bitcoin VM initialized successfully",
		zap.String("lastAccepted", vm.lastAccepted.String()))

	return nil
}

// SetState sets the VM state
func (vm *VM) SetState(ctx context.Context, state snow.State) error {
	vm.ctx.Log.Debug("entering SetState", zap.String("state", state.String()))
	defer vm.ctx.Log.Debug("exiting SetState")

	vm.ctx.Log.Info("setting VM state", zap.String("state", state.String()))

	switch state {
	case snow.StateSyncing:
		vm.bootstrapped = false
		vm.ctx.Log.Info("Bitcoin VM entering state sync")
		return nil

	case snow.Bootstrapping:
		vm.bootstrapped = false
		vm.ctx.Log.Info("Bitcoin VM bootstrapping")
		return nil

	case snow.NormalOp:
		// Only initialize gossip once
		if vm.bootstrapped {
			vm.ctx.Log.Debug("Bitcoin VM already bootstrapped, skipping gossip initialization")
			return nil
		}
		vm.bootstrapped = true
		vm.ctx.Log.Info("Bitcoin VM entering normal operation")

		if err := vm.onNormalOperationsStarted(); err != nil {
			return err
		}

		// Initialize block building after gossip is set up
		return vm.initBlockBuilding()

	default:
		return fmt.Errorf("unknown state: %s", state)
	}
}

// onNormalOperationsStarted initializes gossip and starts gossip goroutines
func (vm *VM) onNormalOperationsStarted() error {
	vm.ctx.Log.Info("Starting normal operations")

	// Create context for gossip goroutines
	vm.gossipCtx, vm.cancel = context.WithCancel(context.Background())

	// Initialize unified gossip system
	if err := vm.initializeGossip(); err != nil {
		return fmt.Errorf("failed to initialize gossip: %w", err)
	}

	// Start gossip loops
	vm.startGossipLoops()

	vm.ctx.Log.Info("Normal operations started successfully")
	return nil
}

// initBlockBuilding starts the block builder goroutines
func (vm *VM) initBlockBuilding() error {
	vm.ctx.Log.Info("initBlockBuilding starting")

	vm.builderLock.Lock()
	defer vm.builderLock.Unlock()

	if vm.blockBuilder == nil {
		return fmt.Errorf("block builder not initialized")
	}

	vm.blockBuilder.start()
	vm.ctx.Log.Info("initBlockBuilding blockBuilder started")

	// Check for existing transactions in mempool
	if mempool := vm.btcdAdapter.TxMemPool(); mempool != nil {
		txCount := len(mempool.MiningDescs())
		vm.ctx.Log.Info("initBlockBuilding checking mempool", zap.Int("txCount", txCount))
		if txCount > 0 {
			vm.blockBuilder.signalCanBuild()
			vm.ctx.Log.Info("initBlockBuilding signaled can build due to existing transactions")
		}
	}

	vm.ctx.Log.Info("initBlockBuilding completed")
	return nil
}

// WaitForEvent waits for block building events
// Implements the block.ChainVM interface for event-driven block production
func (vm *VM) WaitForEvent(ctx context.Context) (common.Message, error) {
	vm.ctx.Log.Info("WaitForEvent called by Snowman engine")

	vm.builderLock.Lock()
	builder := vm.blockBuilder
	vm.builderLock.Unlock()

	if builder == nil {
		vm.ctx.Log.Warn("WaitForEvent called but blockBuilder is nil")
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-vm.shutdownChan:
			return 0, context.Canceled
		}
	}

	msg, err := builder.waitForEvent(ctx)
	vm.ctx.Log.Info("WaitForEvent returning", zap.Uint32("message", uint32(msg)), zap.Error(err))
	return msg, err
}

// Shutdown shuts down the VM
func (vm *VM) Shutdown(ctx context.Context) error {
	vm.ctx.Log.Debug("entering Shutdown")
	defer vm.ctx.Log.Debug("exiting Shutdown")

	if vm.stopped {
		return nil
	}

	vm.ctx.Log.Info("shutting down Bitcoin VM")

	// Cancel gossip context to stop goroutines
	if vm.cancel != nil {
		vm.ctx.Log.Info("Cancelling gossip context")
		vm.cancel()
	}

	// Note: p2pNetwork cleanup is handled by the network layer automatically

	// Stop btcd adapter (gracefully closes database and other resources)
	if vm.btcdAdapter != nil {
		vm.ctx.Log.Info("Stopping btcd adapter")
		if err := vm.btcdAdapter.Stop(); err != nil {
			vm.ctx.Log.Error("Error stopping btcd adapter", zap.Error(err))
		}
	}

	// Signal shutdown
	close(vm.shutdownChan)

	// Wait for all gossip goroutines to finish
	vm.ctx.Log.Info("Waiting for gossip goroutines to finish")
	vm.shutdownWg.Wait()

	vm.stopped = true

	vm.ctx.Log.Info("Bitcoin VM shutdown complete")
	return nil
}

// BuildBlock builds a new block
func (vm *VM) BuildBlock(ctx context.Context) (snowman.Block, error) {
	vm.ctx.Log.Info("BuildBlock called by Snowman engine")

	vm.buildBlockLock.Lock()
	defer vm.buildBlockLock.Unlock()

	if vm.btcdAdapter == nil {
		return nil, fmt.Errorf("btcd adapter not initialized")
	}

	// Get current block to track parent
	currentBlock, err := vm.getCurrentBlock()
	if err != nil {
		vm.ctx.Log.Error("BuildBlock failed to get current block", zap.Error(err))
		return nil, fmt.Errorf("failed to get current block: %w", err)
	}

	vm.ctx.Log.Info("BuildBlock starting", zap.String("parentHash", currentBlock.Hash().String()))

	// Record build attempt for delay calculation
	if vm.blockBuilder != nil {
		vm.blockBuilder.handleBuildAttempt(*currentBlock.Hash())
	}

	generator := vm.btcdAdapter.GetBlockTemplateGenerator()
	if generator == nil {
		return nil, fmt.Errorf("block template generator not available")
	}

	if len(vm.config.MiningAddrs) == 0 {
		return nil, fmt.Errorf("no mining address configured")
	}

	payToAddr, err := btcutil.DecodeAddress(vm.config.MiningAddrs[0], vm.config.ChainParams)
	if err != nil {
		return nil, fmt.Errorf("failed to decode mining address: %w", err)
	}

	template, err := generator.NewBlockTemplate(payToAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create block template: %w", err)
	}

	template.Block.Header.Nonce = 0
	block := btcutil.NewBlock(template.Block)

	isMainChain, isOrphan, err := vm.btcdAdapter.ProcessBlockNoPoW(block)
	if err != nil {
		return nil, fmt.Errorf("failed to process block: %w", err)
	}

	if isOrphan {
		return nil, fmt.Errorf("generated block is orphan (parent missing)")
	}

	blockAdapter, err := NewBlockAdapter(vm, block)
	if err != nil {
		return nil, fmt.Errorf("failed to create block adapter: %w", err)
	}

	if vm.blockBuilder != nil {
		vm.blockBuilder.clearPendingSignal()
	}

	vm.ctx.Log.Info("Built block",
		zap.String("id", blockAdapter.ID().String()),
		zap.Uint64("height", blockAdapter.Height()),
		zap.Int("txs", len(block.Transactions())-1),
		zap.Bool("mainChain", isMainChain))

	return blockAdapter, nil
}

// ParseBlock parses a block from bytes
func (vm *VM) ParseBlock(ctx context.Context, blockBytes []byte) (snowman.Block, error) {
	if !vm.initialized {
		return nil, errNotInitialized
	}

	// Create block adapter from the serialized bytes
	blockAdapter, err := NewBlockAdapterFromBytes(vm, blockBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse block: %w", err)
	}

	vm.ctx.Log.Info("Successfully parsed block",
		zap.String("blockID", blockAdapter.ID().String()),
		zap.Uint64("height", blockAdapter.Height()),
	)
	return blockAdapter, nil
}

// GetBlock returns a block by ID
func (vm *VM) GetBlock(ctx context.Context, blockID ids.ID) (snowman.Block, error) {
	if !vm.initialized {
		return nil, errNotInitialized
	}
	vm.ctx.Log.Debug("getting block", zap.String("id", blockID.String()))

	block, err := vm.getBlock(blockID)
	if err != nil {
		vm.ctx.Log.Error("failed to get block",
			zap.String("id", blockID.String()),
			zap.Error(err))
		return nil, err
	}

	return block, nil
}

// getBlock returns a block by ID (internal)
func (vm *VM) getBlock(blockID ids.ID) (snowman.Block, error) {
	// Use the block adapter to fetch and wrap the Bitcoin block
	blockAdapter, err := NewBlockAdapterFromID(vm, blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get block adapter: %w", err)
	}

	return blockAdapter, nil
}

// getCurrentBlock returns the current best block from the blockchain
func (vm *VM) getCurrentBlock() (*btcutil.Block, error) {
	bestHash := vm.chain.BestSnapshot().Hash
	block, err := vm.chain.BlockByHash(&bestHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get current block: %w", err)
	}
	return block, nil
}

// SetPreference sets the preferred block
func (vm *VM) SetPreference(ctx context.Context, blockID ids.ID) error {
	if !vm.initialized {
		return errNotInitialized
	}

	vm.preferred = blockID
	vm.ctx.Log.Debug("set preference", zap.String("id", blockID.String()))
	return nil
}

// LastAccepted returns the last accepted block ID
func (vm *VM) LastAccepted(ctx context.Context) (ids.ID, error) {
	if !vm.initialized {
		return ids.Empty, errNotInitialized
	}

	return vm.lastAccepted, nil
}

// GetBlockIDAtHeight returns the block ID at a given height
func (vm *VM) GetBlockIDAtHeight(ctx context.Context, height uint64) (ids.ID, error) {
	if !vm.initialized {
		return ids.Empty, errNotInitialized
	}

	// Get block hash at the specified height
	blockHash, err := vm.chain.BlockHashByHeight(int32(height))
	if err != nil {
		vm.ctx.Log.Error("failed to get block hash at height",
			zap.Uint64("height", height),
			zap.Error(err))
		return ids.Empty, fmt.Errorf("failed to get block hash at height %d: %w", height, err)
	}

	// Convert Bitcoin hash to Metal ID
	blockID := hashToID(blockHash)
	vm.ctx.Log.Debug("retrieved block ID at height",
		zap.Uint64("height", height),
		zap.String("id", blockID.String()))

	return blockID, nil
}

// Version returns the VM version
func (vm *VM) Version(ctx context.Context) (string, error) {
	return Version.String(), nil
}

// HealthCheck returns health status
func (vm *VM) HealthCheck(ctx context.Context) (interface{}, error) {
	if !vm.initialized {
		return nil, errNotInitialized
	}

	return map[string]interface{}{
		"initialized":  vm.initialized,
		"lastAccepted": vm.lastAccepted.String(),
	}, nil
}

// AppGossip handles incoming gossip messages
func (vm *VM) AppGossip(ctx context.Context, nodeID ids.NodeID, msgBytes []byte) error {
	if !vm.initialized {
		return errNotInitialized
	}

	return vm.p2pNetwork.AppGossip(ctx, nodeID, msgBytes)
}

// AppRequest handles incoming app requests
func (vm *VM) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	deadline time.Time,
	msgBytes []byte,
) error {
	// Not implemented yet
	return nil
}

// AppRequestFailed handles failed app requests
func (vm *VM) AppRequestFailed(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	appErr *common.AppError,
) error {
	// Log the failure
	return nil
}

// AppResponse handles responses to app requests
func (vm *VM) AppResponse(ctx context.Context, nodeID ids.NodeID, requestID uint32, msgBytes []byte) error {
	// Not implemented yet
	return nil
}

// Connected is called when a new connection is established
func (vm *VM) Connected(ctx context.Context, nodeID ids.NodeID, nodeVersion *version.Application) error {
	return nil
}

// Disconnected is called when a connection is terminated
func (vm *VM) Disconnected(ctx context.Context, nodeID ids.NodeID) error {
	return nil
}

// CrossChainAppRequest handles incoming cross-chain app requests
func (vm *VM) CrossChainAppRequest(
	ctx context.Context,
	chainID ids.ID,
	requestID uint32,
	deadline time.Time,
	msgBytes []byte,
) error {
	return errors.New("cross-chain requests not supported")
}

// CrossChainAppRequestFailed handles failed cross-chain app requests
func (vm *VM) CrossChainAppRequestFailed(
	ctx context.Context,
	chainID ids.ID,
	requestID uint32,
	appErr *common.AppError,
) error {
	return nil
}

// CrossChainAppResponse handles responses to cross-chain app requests
func (vm *VM) CrossChainAppResponse(ctx context.Context, chainID ids.ID, requestID uint32, msgBytes []byte) error {
	return nil
}

// CreateHandlers creates and returns HTTP handlers
func (vm *VM) CreateHandlers(context.Context) (map[string]http.Handler, error) {
	if vm.btcdAdapter == nil {
		return nil, fmt.Errorf("btcd adapter not initialized")
	}

	// Get RPC server from btcd adapter
	rpcServer := vm.btcdAdapter.RPCServer()
	if rpcServer == nil {
		return nil, fmt.Errorf("RPC server not initialized")
	}

	// Start RPC server and get the HTTP handler
	rpcHandler, wsHandler := rpcServer.Start()
	if rpcHandler == nil || wsHandler == nil {
		return nil, fmt.Errorf("failed to get RPC handler")
	}

	vm.ctx.Log.Info("RPC handlers created successfully using btcd adapter",
		zap.Strings("endpoints", []string{"Bitcoin RPC methods via btcd adapter"}),
	)

	return map[string]http.Handler{
		"/rpc": rpcHandler,
		"/ws":  wsHandler,
	}, nil
}
