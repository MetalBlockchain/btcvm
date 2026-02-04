package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

func main() {
	// Command line flags
	generateKeys := flag.Bool("generate", false, "Generate new key pair")
	address := flag.String("address", "", "Bitcoin address for genesis coinbase (P2PKH or P2WPKH)")
	coinbaseMsg := flag.String("message", "BTCVM Genesis Block - Powered by Metal Blockchain", "Coinbase message")
	reward := flag.Int64("reward", 5000000000, "Coinbase reward in satoshis (default: 50 BTC)")
	timestamp := flag.Int64("timestamp", 0, "Block timestamp (unix seconds, default: now)")
	network := flag.String("net", "mainnet", "Network to use (mainnet, testnet, regtest, simnet, signet)")

	flag.Parse()

	// Select network parameters
	var netParams *chaincfg.Params
	switch *network {
	case "mainnet":
		netParams = &chaincfg.MainNetParams
	case "testnet":
		netParams = &chaincfg.TestNet3Params
	case "regtest":
		netParams = &chaincfg.RegressionNetParams
	case "simnet":
		netParams = &chaincfg.SimNetParams
	case "signet":
		netParams = &chaincfg.SigNetParams
	default:
		fmt.Printf("Error: Unknown network '%s'\n", *network)
		os.Exit(1)
	}

	// Generate keys if requested
	if *generateKeys {
		generateKeyPair(netParams)
		return
	}

	// Validate address is provided
	if *address == "" {
		fmt.Printf(`Error: You must provide a Bitcoin address with -address flag

Usage:
  Generate keys:      go run main.go -generate -net <network>
  Create genesis:     go run main.go -address <bitcoin-address> -net <network>

`)
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Parse address
	addr, err := btcutil.DecodeAddress(*address, netParams)
	if err != nil {
		fmt.Printf("Error: Invalid Bitcoin address for network %s: %v\n", *network, err)
		os.Exit(1)
	}

	// Create genesis block
	genesisBlock, err := createGenesisBlock(addr, *coinbaseMsg, *reward, *timestamp)
	if err != nil {
		fmt.Printf("Error creating genesis block: %v\n", err)
		os.Exit(1)
	}

	// Serialize genesis block
	var buf bytes.Buffer
	if err := genesisBlock.Serialize(&buf); err != nil {
		fmt.Printf("Error serializing genesis block: %v\n", err)
		os.Exit(1)
	}

	genesisBytes := buf.Bytes()
	genesisHex := hex.EncodeToString(genesisBytes)
	blockHash := genesisBlock.BlockHash()

	// Output results
	fmt.Printf(`========================================
Custom Genesis Block Generated
========================================

Block Hash: %s
Merkle Root: %s
Timestamp: %s
Coinbase Reward: %s
Recipient Address: %s

Genesis Block (hex):
%s

`, blockHash.String(),
		genesisBlock.Header.MerkleRoot.String(),
		time.Unix(genesisBlock.Header.Timestamp.Unix(), 0).Format(time.RFC3339),
		btcutil.Amount(*reward).String(),
		addr.String(),
		genesisHex,
	)
	fmt.Printf(`========================================
		Configuration
		========================================
		
		Add this to your VM genesis configuration:
		
		{
		  "genesisBlock": "%s",
		  "coinbaseAddress": "%s",
		  "blockHash": "%s",
		  "timestamp": %d
		}
		
		Or save the hex to a file for use with createBlockchain:
		echo '%s' > genesis.hex
		
		========================================
		Go Code (btcd/params.go)
		========================================

`,
		genesisHex,
		addr.String(),
		blockHash.String(),
		genesisBlock.Header.Timestamp.Unix(),
		genesisHex,
	)
	printHashAsGoStruct(genesisBlock.Header.MerkleRoot, "btcVMTestNetGenesisMerkleRoot")
	fmt.Println()
	printHashAsGoStruct(blockHash, "btcVMTestNetGenesisHash")
	fmt.Println()
	printTxAsGoStruct(genesisBlock.Transactions[0], "genesisCoinbaseTx")
	fmt.Println()
	printBlockAsGoStruct(genesisBlock, "btcVMTestNetGenesisBlock")
	fmt.Println()
}

func printHashAsGoStruct(hash chainhash.Hash, varName string) {
	fmt.Printf("%s = chainhash.Hash([chainhash.HashSize]byte{\n", varName)
	for i := range chainhash.HashSize {
		if i%8 == 0 {
			fmt.Print("\t")
		}
		fmt.Printf("0x%02x, ", hash[i])
		if i%8 == 7 {
			// Print ASCII representation for the last 8 bytes
			fmt.Print("/* |")
			for j := i - 7; j <= i; j++ {
				b := hash[j]
				if b >= 32 && b <= 126 {
					fmt.Printf("%c", b)
				} else {
					fmt.Print(".")
				}
			}
			fmt.Println("| */")
		}
	}
	fmt.Println("})")
}

func printTxAsGoStruct(tx *wire.MsgTx, varName string) {
	fmt.Printf(`%s = wire.MsgTx{
	Version: %d,
	TxIn: []*wire.TxIn{
`, varName, tx.Version)
	for _, txIn := range tx.TxIn {
		fmt.Printf(`		{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash{},
				Index: 0xffffffff,
			},
			SignatureScript: []byte{
`)
		printBytesWithAscii(txIn.SignatureScript, 4)
		fmt.Printf(`			},
			Sequence: 0xffffffff,
		},
`)
	}
	fmt.Printf(`	},
	TxOut: []*wire.TxOut{
`)
	for _, txOut := range tx.TxOut {
		fmt.Printf(`		{
			Value: 0x%x,
			PkScript: []byte{
`, txOut.Value)
		printBytesWithAscii(txOut.PkScript, 4)
		fmt.Printf(`			},
		},
`)
	}
	fmt.Printf(`	},
	LockTime: %d,
}
`, tx.LockTime)
}

func printBlockAsGoStruct(block *wire.MsgBlock, varName string) {
	fmt.Printf(`// %s defines the genesis block of the block chain which
// serves as the public transaction ledger for the test network (version 3).
%s = wire.MsgBlock{
	Header: wire.BlockHeader{
		Version:    %d,
		PrevBlock:  chainhash.Hash{}, // %s
		MerkleRoot: btcVMTestNetGenesisMerkleRoot, // %s
		Timestamp:  time.Unix(%d, 0), // %s
		Bits:       0x%x, // %d
		Nonce:      0x%X, // %d
	},
	Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
}
`, varName,
		varName,
		block.Header.Version,
		block.Header.PrevBlock.String(),
		block.Header.MerkleRoot.String(),
		block.Header.Timestamp.Unix(),
		block.Header.Timestamp.Format(time.RFC3339),
		block.Header.Bits, block.Header.Bits,
		block.Header.Nonce, block.Header.Nonce,
	)
}

func printBytesWithAscii(data []byte, indentLevel int) {
	indent := ""
	for range indentLevel {
		indent += "\t"
	}

	for i := range data {
		if i%8 == 0 {
			fmt.Print(indent)
		}
		fmt.Printf("0x%02x, ", data[i])
		if i%8 == 7 || i == len(data)-1 {
			// Pad if not a full line
			if i == len(data)-1 && i%8 != 7 {
				for k := 0; k < 7-(i%8); k++ {
					fmt.Print("      ")
				}
			}

			// Print ASCII representation for the current line
			fmt.Print("/* |")
			start := (i / 8) * 8
			for j := start; j <= i; j++ {
				b := data[j]
				if b >= 32 && b <= 126 {
					fmt.Printf("%c", b)
				} else {
					fmt.Print(".")
				}
			}
			// Pad ASCII if not a full line
			if i == len(data)-1 && i%8 != 7 {
				for k := 0; k < 7-(i%8); k++ {
					fmt.Print(".")
				}
			}
			fmt.Println("| */")
		}
	}
}

func generateKeyPair(netParams *chaincfg.Params) {
	// Generate new private key
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		fmt.Printf("Error generating private key: %v\n", err)
		os.Exit(1)
	}

	// Get public key
	pubKey := privKey.PubKey()

	// Create addresses
	// P2PKH (Pay to Public Key Hash)
	pubKeyHash := btcutil.Hash160(pubKey.SerializeCompressed())
	addressPubKeyHash, err := btcutil.NewAddressPubKeyHash(pubKeyHash, netParams)
	if err != nil {
		fmt.Printf("Error creating P2PKH address: %v\n", err)
		os.Exit(1)
	}

	// P2WPKH (Pay to Witness Public Key Hash) - SegWit
	addressWitness, err := btcutil.NewAddressWitnessPubKeyHash(pubKeyHash, netParams)
	if err != nil {
		fmt.Printf("Error creating P2WPKH address: %v\n", err)
		os.Exit(1)
	}

	// Create WIF for private key
	wif, err := btcutil.NewWIF(privKey, netParams, true)
	if err != nil {
		fmt.Printf("Error creating WIF: %v\n", err)
		os.Exit(1)
	}

	// Output key information
	fmt.Printf(`========================================
New Key Pair Generated (%s)
========================================

⚠️  PRIVATE KEY - KEEP THIS SECRET! ⚠️
Private Key (WIF): %s
Private Key (hex): %s

Public Key:
  Compressed: %s
  Uncompressed: %s

Addresses:
  P2PKH (Legacy): %s
  P2WPKH (SegWit): %s

========================================

⚠️  IMPORTANT SECURITY NOTES:
1. Save your private key in a secure location
2. Never share your private key with anyone
3. Backup your private key offline
4. The private key controls all funds in the genesis block

To create a genesis block with this key, run:
  go run main.go -address %s -net %s

`, netParams.Name,
		wif.String(),
		hex.EncodeToString(privKey.Serialize()),
		hex.EncodeToString(pubKey.SerializeCompressed()),
		hex.EncodeToString(pubKey.SerializeUncompressed()),
		addressPubKeyHash.String(),
		addressWitness.String(),
		addressPubKeyHash.String(), netParams.Name,
	)
}

func createGenesisBlock(
	addr btcutil.Address,
	coinbaseMsg string,
	reward int64,
	timestamp int64,
) (*wire.MsgBlock, error) {
	// Set timestamp
	var blockTime time.Time
	if timestamp == 0 {
		blockTime = time.Now()
	} else {
		blockTime = time.Unix(timestamp, 0)
	}

	// Create coinbase transaction
	coinbaseTx := wire.NewMsgTx(wire.TxVersion)

	// Coinbase input
	coinbaseTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  chainhash.Hash{},
			Index: 0xffffffff,
		},
		SignatureScript: []byte(coinbaseMsg),
		Sequence:        0xffffffff,
	})

	// Create output script
	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create output script: %w", err)
	}

	// Coinbase output
	coinbaseTx.AddTxOut(&wire.TxOut{
		Value:    reward,
		PkScript: pkScript,
	})

	// Calculate merkle root
	merkleRoot := coinbaseTx.TxHash()

	// Create block header
	header := wire.BlockHeader{
		Version:    1,
		PrevBlock:  chainhash.Hash{}, // Genesis has no parent
		MerkleRoot: merkleRoot,
		Timestamp:  blockTime,
		Bits:       0x1d00ffff, // Difficulty (same as Bitcoin genesis)
		Nonce:      0,          // We'll mine this
	}

	// Create block
	block := &wire.MsgBlock{
		Header:       header,
		Transactions: []*wire.MsgTx{coinbaseTx},
	}

	// Mine the block (find a valid nonce)
	fmt.Println("Mining genesis block...")
	target := compactToBig(header.Bits)
	for {
		blockHash := block.BlockHash()
		hashNum := hashToBig(&blockHash)

		if hashNum.Cmp(target) <= 0 {
			fmt.Printf("Found valid nonce: %d\n", block.Header.Nonce)
			break
		}

		block.Header.Nonce++
		// if block.Header.Nonce%100000 == 0 {
		// 	fmt.Printf("Tried %d nonces...\n", block.Header.Nonce)
		// }
	}

	return block, nil
}

// Helper functions for mining

func compactToBig(compact uint32) *big.Int {
	// This is a simplified version
	// Extract the exponent and mantissa
	exponent := uint(compact >> 24)
	mantissa := compact & 0x00ffffff

	var bn big.Int
	bn.SetUint64(uint64(mantissa))

	if exponent <= 3 {
		bn.Rsh(&bn, 8*(3-exponent))
	} else {
		bn.Lsh(&bn, 8*(exponent-3))
	}

	return &bn
}

func hashToBig(hash *chainhash.Hash) *big.Int {
	// Convert hash to big int (in reverse byte order)
	var hashNum big.Int

	// Reverse the hash bytes
	buf := make([]byte, chainhash.HashSize)
	for i := range chainhash.HashSize {
		buf[i] = hash[chainhash.HashSize-1-i]
	}

	hashNum.SetBytes(buf)
	return &hashNum
}
