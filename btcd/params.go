// license that can be found in the LICENSE file.

package btcd

import (
	"math/big"
	"time"

	"github.com/MetalBlockchain/btcvm/btcd/chaincfg"
	"github.com/MetalBlockchain/btcvm/btcd/chaincfg/chainhash"
	"github.com/MetalBlockchain/btcvm/btcd/wire"
)

// activeNetParams is a pointer to the parameters specific to the
// currently active bitcoin network.
var activeNetParams = &btcVMTestNetParms

// params is used to group parameters for various networks such as the main
// network and test networks.
type params struct {
	*chaincfg.Params
	rpcPort string
}

// mainNetParams contains parameters specific to the main network
// (wire.MainNet).  NOTE: The RPC port is intentionally different than the
// reference implementation because btcd does not handle wallet requests.  The
// separate wallet process listens on the well-known port and forwards requests
// it does not handle on to btcd.  This approach allows the wallet process
// to emulate the full reference implementation RPC API.
var mainNetParams = params{
	Params:  &chaincfg.MainNetParams,
	rpcPort: "8334",
}

// genesisMerkleRoot is the hash of the first transaction in the genesis block
var (
	bigOne               = big.NewInt(1)
	btcVMTestNetPowLimit = new(big.Int).Sub(new(big.Int).Lsh(bigOne, 255), bigOne)

	btcVMTestNetGenesisMerkleRoot = chainhash.Hash([chainhash.HashSize]byte{
		0x43, 0x6a, 0xde, 0xcf, 0xe4, 0xd9, 0x31, 0x22, /* |Cj....1"| */
		0x0b, 0xd4, 0xdd, 0x44, 0xce, 0x55, 0x44, 0xba, /* |...D.UD.| */
		0xa7, 0x69, 0x8c, 0xaf, 0xe0, 0x08, 0xdc, 0xef, /* |.i......| */
		0x4b, 0x23, 0xd1, 0x7b, 0x75, 0x83, 0xac, 0x5d, /* |K#.{u..]| */
	})

	btcVMTestNetGenesisHash = chainhash.Hash([chainhash.HashSize]byte{
		0xe4, 0x3b, 0x3b, 0x00, 0xc5, 0xf7, 0x44, 0x44, /* |.;;...DD| */
		0xd6, 0x86, 0xf7, 0x7b, 0xf9, 0x31, 0xe3, 0x3f, /* |...{.1.?| */
		0xc7, 0x98, 0x31, 0xcb, 0x7b, 0xa9, 0x68, 0x91, /* |..1.{.h.| */
		0xfb, 0x6b, 0xcf, 0x17, 0x00, 0x00, 0x00, 0x00, /* |.k......| */
	})

	genesisCoinbaseTx = wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{
					Hash:  chainhash.Hash{},
					Index: 0xffffffff,
				},
				SignatureScript: []byte{
					0x42, 0x54, 0x43, 0x56, 0x4d, 0x20, 0x47, 0x65, /* |BTCVM Ge| */
					0x6e, 0x65, 0x73, 0x69, 0x73, 0x20, 0x42, 0x6c, /* |nesis Bl| */
					0x6f, 0x63, 0x6b, 0x20, 0x2d, 0x20, 0x50, 0x6f, /* |ock - Po| */
					0x77, 0x65, 0x72, 0x65, 0x64, 0x20, 0x62, 0x79, /* |wered by| */
					0x20, 0x4d, 0x65, 0x74, 0x61, 0x6c, 0x20, 0x42, /* | Metal B| */
					0x6c, 0x6f, 0x63, 0x6b, 0x63, 0x68, 0x61, 0x69, /* |lockchai| */
					0x6e, /* |n.......| */
				},
				Sequence: 0xffffffff,
			},
		},
		TxOut: []*wire.TxOut{
			{
				Value: 0x12a05f200,
				PkScript: []byte{
					0x76, 0xa9, 0x14, 0x5d, 0x77, 0x8a, 0xa0, 0xa0, /* |v..]w...| */
					0x12, 0xa3, 0x59, 0x5c, 0x2c, 0x2f, 0xd4, 0xe6, /* |..Y\,/..| */
					0x04, 0xe0, 0x73, 0x99, 0x40, 0x15, 0xcb, 0x88, /* |..s.@...| */
					0xac, /* |........| */
				},
			},
		},
		LockTime: 0,
	}

	// btcVMTestNetGenesisBlock defines the genesis block of the block chain which
	// serves as the public transaction ledger for the test network (version 3).
	btcVMTestNetGenesisBlock = wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:    1,
			PrevBlock:  chainhash.Hash{},              // 0000000000000000000000000000000000000000000000000000000000000000
			MerkleRoot: btcVMTestNetGenesisMerkleRoot, // 5dac83757bd1234befdc08e0af8c69a7ba4455ce44ddd40b2231d9e4cfde6a43
			Timestamp:  time.Unix(1766342623, 0),      // 2025-12-21T12:43:43-06:00
			Bits:       0x1d00ffff,                    // 486604799
			Nonce:      0x13DC5589,                    // 333206921
		},
		Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
	}
)

// BtcVMTestNetParams defines the network parameters for the test Bitcoin network
// (version 3).  Not to be confused with the regression test network, this
// network is sometimes simply called "testnet".
var BtcvmTestNetParms = chaincfg.Params{
	Name:        "btcvmtestnet",
	Net:         wire.SimNet,
	DefaultPort: "18555",
	DNSSeeds:    []chaincfg.DNSSeed{}, // NOTE: There must NOT be any seeds.

	// Chain parameters
	GenesisBlock:             &btcVMTestNetGenesisBlock,
	GenesisHash:              &btcVMTestNetGenesisHash,
	PowLimit:                 btcVMTestNetPowLimit,
	PowLimitBits:             0x1d00ffff,
	BIP0034Height:            0, // Always active on simnet
	BIP0065Height:            0, // Always active on simnet
	BIP0066Height:            0, // Always active on simnet
	CoinbaseMaturity:         0,
	SubsidyReductionInterval: 210000,
	TargetTimespan:           time.Hour * 24 * 14, // 14 days
	TargetTimePerBlock:       time.Minute * 10,    // 10 minutes
	RetargetAdjustmentFactor: 4,                   // 25% less, 400% more
	ReduceMinDifficulty:      true,
	MinDiffReductionTime:     time.Minute * 20, // TargetTimePerBlock * 2
	GenerateSupported:        true,

	// Checkpoints ordered from oldest to newest.
	Checkpoints: nil,

	// Consensus rule change deployments.
	//
	// The miner confirmation window is defined as:
	//   target proof of work timespan / target proof of work spacing
	RuleChangeActivationThreshold: 75, // 75% of MinerConfirmationWindow
	MinerConfirmationWindow:       100,
	Deployments: [chaincfg.DefinedDeployments]chaincfg.ConsensusDeployment{
		chaincfg.DeploymentTestDummy: {
			BitNumber: 28,
			DeploymentStarter: chaincfg.NewMedianTimeDeploymentStarter(
				time.Time{}, // Always available for vote
			),
			DeploymentEnder: chaincfg.NewMedianTimeDeploymentEnder(
				time.Time{}, // Never expires
			),
		},
		chaincfg.DeploymentTestDummyMinActivation: {
			BitNumber:                 22,
			CustomActivationThreshold: 50,  // Only needs 50% hash rate.
			MinActivationHeight:       600, // Can only activate after height 600.
			DeploymentStarter: chaincfg.NewMedianTimeDeploymentStarter(
				time.Time{}, // Always available for vote
			),
			DeploymentEnder: chaincfg.NewMedianTimeDeploymentEnder(
				time.Time{}, // Never expires
			),
		},
		chaincfg.DeploymentCSV: {
			BitNumber: 0,
			DeploymentStarter: chaincfg.NewMedianTimeDeploymentStarter(
				time.Time{}, // Always available for vote
			),
			DeploymentEnder: chaincfg.NewMedianTimeDeploymentEnder(
				time.Time{}, // Never expires
			),
		},
		chaincfg.DeploymentSegwit: {
			BitNumber: 1,
			DeploymentStarter: chaincfg.NewMedianTimeDeploymentStarter(
				time.Time{}, // Always available for vote
			),
			DeploymentEnder: chaincfg.NewMedianTimeDeploymentEnder(
				time.Time{}, // Never expires.
			),
		},
		chaincfg.DeploymentTaproot: {
			BitNumber: 2,
			DeploymentStarter: chaincfg.NewMedianTimeDeploymentStarter(
				time.Time{}, // Always available for vote
			),
			DeploymentEnder: chaincfg.NewMedianTimeDeploymentEnder(
				time.Time{}, // Never expires.
			),
			CustomActivationThreshold: 75, // Only needs 75% hash rate.
		},
		chaincfg.DeploymentTestDummyAlwaysActive: {
			BitNumber: 29,
			DeploymentStarter: chaincfg.NewMedianTimeDeploymentStarter(
				time.Time{}, // Always available for vote
			),
			DeploymentEnder: chaincfg.NewMedianTimeDeploymentEnder(
				time.Time{}, // Never expires
			),
			AlwaysActiveHeight: 1,
		},
	},

	// Mempool parameters
	RelayNonStdTxs: true,

	// Human-readable part for Bech32 encoded segwit addresses, as defined in
	// BIP 173.
	Bech32HRPSegwit: "sb", // always sb for sim net

	// Address encoding magics
	PubKeyHashAddrID:        0x3f, // starts with S
	ScriptHashAddrID:        0x7b, // starts with s
	PrivateKeyID:            0x64, // starts with 4 (uncompressed) or F (compressed)
	WitnessPubKeyHashAddrID: 0x19, // starts with Gg
	WitnessScriptHashAddrID: 0x28, // starts with ?

	// BIP32 hierarchical deterministic extended key magics
	HDPrivateKeyID: [4]byte{0x04, 0x20, 0xb9, 0x00}, // starts with sprv
	HDPublicKeyID:  [4]byte{0x04, 0x20, 0xbd, 0x3a}, // starts with spub

	// BIP44 coin type used in the hierarchical deterministic path for
	// address generation.
	HDCoinType: 115, // ASCII for s
}

// btcvmTestNetParms contains parameters specific to the test network (version 3)
// (wire.TestNet).  NOTE: The RPC port is intentionally different than the
// reference implementation - see the mainNetParams comment for details.
var btcVMTestNetParms = params{
	Params:  &BtcvmTestNetParms,
	rpcPort: "18334",
}

// netName returns the name used when referring to a bitcoin network.  At the
// time of writing, btcd currently places blocks for testnet version 3 in the
// data and log directory "testnet", which does not match the Name field of the
// chaincfg parameters.  This function can be used to override this directory
// name as "testnet" when the passed active network matches wire.TestNet3.
//
// A proper upgrade to move the data and log directories for this network to
// "testnet3" is planned for the future, at which point this function can be
// removed and the network parameter's name used instead.
func netName(chainParams *params) string {
	switch chainParams.Net {
	case wire.TestNet3:
		return "testnet"
	default:
		return chainParams.Name
	}
}
