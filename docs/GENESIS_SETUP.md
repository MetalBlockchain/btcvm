# Custom Genesis Block Setup Guide

This guide shows you how to create a custom genesis block for BTCVM with your own keys.

## ⚠️ Security Warning

**NEVER share your private key with anyone!** Your private key controls all funds in the genesis block. Keep it safe and secure.

## Quick Reference

The genesis-generator tool provides three main commands:

```bash
# Generate a new Bitcoin key pair
go run main.go -generate

# Create a custom genesis block
go run main.go -address <bitcoin-address>

# Convert hex to Go byte array format
go run main.go -hex2go <hex-string>
```

## Option 1: Generate New Key Pair (Recommended)

### Step 1: Generate Keys

```bash
cd cmd/genesis-generator
go run main.go -generate
```

This will output:
```
========================================
New Key Pair Generated
========================================

⚠️  PRIVATE KEY - KEEP THIS SECRET! ⚠️
Private Key (WIF): 12345...
Private Key (hex): abcdef...

Public Key:
  Compressed: 02abc123...
  Uncompressed: 04abc123...

Addresses:
  P2PKH (Legacy): 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa
  P2WPKH (SegWit): bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4

⚠️  IMPORTANT SECURITY NOTES:
1. Save your private key in a secure location
2. Never share your private key with anyone
3. Backup your private key offline
4. The private key controls all funds in the genesis block
```

### Step 2: Save Your Private Key

**Create a secure backup:**

```bash
# Create a secure directory
mkdir -p ~/.btcvm-keys
chmod 700 ~/.btcvm-keys

# Save your private key (REPLACE with your actual key)
echo "YOUR_PRIVATE_KEY_HEX" > ~/.btcvm-keys/genesis-key.txt
chmod 600 ~/.btcvm-keys/genesis-key.txt

# IMPORTANT: Also backup offline (USB drive, paper wallet, etc.)
```

### Step 3: Create Genesis Block

Using the P2PKH address from Step 1:

```bash
go run main.go -address "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
```

Optional parameters:
```bash
go run main.go \
  -address "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa" \
  -message "My Custom BTCVM Genesis - $(date)" \
  -reward 5000000000 \
  -timestamp $(date +%s)
```

This will mine a genesis block and output:

```
Mining genesis block...
Found valid nonce: 2083236893
========================================
Custom Genesis Block Generated
========================================

Block Hash: 000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f
Merkle Root: 4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b
Timestamp: 2025-10-06T14:35:00Z
Coinbase Reward: 50 BTC
Recipient Address: 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa

Genesis Block (hex):
0100000000000000000000000000000000000000000000000000000000000000...

========================================
Configuration
========================================

Add this to your VM genesis configuration:

{
  "genesisBlock": "0100000000000000...",
  "coinbaseAddress": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
  "blockHash": "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f",
  "timestamp": 1728227700
}
```

### Step 4: Save Genesis Block Hex

```bash
# Save to file
echo "0100000000000000..." > genesis.hex
```

## Option 2: Use Existing Bitcoin Address

If you already have a Bitcoin address and private key:

### Step 1: Create Genesis Block

```bash
cd cmd/genesis-generator
go run main.go -address "YOUR_BITCOIN_ADDRESS"
```

### Step 2: Save the genesis hex output

Save the hex output to a file for later use.

## Using Your Custom Genesis Block

There are two ways to use your custom genesis block:

### Method A: Using genesis.hex File

1. Save your genesis block hex to a file:
   ```bash
   echo "YOUR_GENESIS_HEX" > genesis.hex
   ```

2. When creating the blockchain, use the hex as genesisData:
   ```bash
   GENESIS_HEX=$(cat genesis.hex)

   curl -X POST --data "{
       \"jsonrpc\": \"2.0\",
       \"id\": 1,
       \"method\": \"platform.createBlockchain\",
       \"params\": {
           \"subnetID\": \"11111111111111111111111111111111LpoYY\",
           \"vmID\": \"kMtihm7W3KssmcJb9mzwZfC6gkiPrJhWaa5KMLHdEB9R8Q4pp\",
           \"name\": \"btcvm\",
           \"genesisData\": \"$GENESIS_HEX\"
       }
   }" -H 'content-type:application/json;' http://127.0.0.1:9650/ext/P
   ```

### Method B: Hardcode in VM

Update `vm/vm.go` to use your custom genesis:

```go
// createDefaultGenesis creates a default genesis block
func (vm *VM) createDefaultGenesis() *wire.MsgBlock {
	// Decode custom genesis from hex
	genesisHex := "YOUR_GENESIS_HEX_HERE"
	genesisBytes, err := hex.DecodeString(genesisHex)
	if err != nil {
		log.Error("failed to decode genesis hex", "error", err)
		// Fallback to standard genesis
		return vm.config.ChainParams.GenesisBlock
	}

	msgBlock := &wire.MsgBlock{}
	if err := msgBlock.Deserialize(bytes.NewReader(genesisBytes)); err != nil {
		log.Error("failed to deserialize genesis", "error", err)
		// Fallback to standard genesis
		return vm.config.ChainParams.GenesisBlock
	}

	return msgBlock
}
```

## Verifying Your Genesis Block

After starting your chain, verify the genesis block:

```bash
# Get chain ID
CHAIN_ID=$(curl -s -X POST --data '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "platform.getBlockchains"
}' -H 'content-type:application/json;' http://127.0.0.1:9650/ext/P | \
jq -r '.result.blockchains[] | select(.name=="btcvm") | .id')

# Get genesis block
curl -X POST --data '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "btcvm.GetBlock",
    "params": [{"height": 0}]
}' -H 'content-type:application/json;' \
http://127.0.0.1:9650/ext/bc/$CHAIN_ID/rpc | jq
```

Verify:
- The block hash matches your generated hash
- The coinbase transaction pays to your address
- The timestamp is correct

## Spending Genesis Coins

To spend the coins from your genesis block, you'll need:

1. **Your private key** (from Step 1)
2. **A Bitcoin transaction tool** (btcd, bitcoin-cli, or custom code)
3. **The UTXO from the genesis coinbase**

Example using btcd tools:

```bash
# Create a raw transaction spending the genesis coinbase
# Input: Genesis coinbase output (txid + vout 0)
# Output: Your destination address

# Sign with your private key
# Broadcast via SubmitTransaction RPC
```

## Security Best Practices

### DO:
✅ Generate keys on an offline/secure computer
✅ Backup your private key in multiple secure locations
✅ Use strong encryption for key storage
✅ Test with small amounts first
✅ Keep a paper backup of your private key

### DON'T:
❌ Share your private key with anyone
❌ Store private keys in plain text files
❌ Commit private keys to git repositories
❌ Send private keys over the internet
❌ Screenshot or photograph your private key

## Utility: Converting Hex to Go Code

The genesis-generator includes a utility to convert hex strings into Go byte array format with ASCII comments, useful for hardcoding data in your code.

### Usage

```bash
cd cmd/genesis-generator
go run main.go -hex2go "04ffff001d0104455468652054696d6573..."
```

Or using the built binary:
```bash
./build/genesis-generator -hex2go "your-hex-string"
```

### Example Output

Input:
```bash
./build/genesis-generator -hex2go "04ffff001d0104455468652054696d65732030332f4a616e2f32303039204368616e63656c6c6f72206f6e206272696e6b206f66207365636f6e64206261696c6f757420666f722062616e6b73"
```

Output:
```go
// Hex converted to Go byte array
[]byte{
	0x04, 0xff, 0xff, 0x00, 0x1d, 0x01, 0x04, 0x45, /* |.......E| */
	0x54, 0x68, 0x65, 0x20, 0x54, 0x69, 0x6d, 0x65, /* |The Time| */
	0x73, 0x20, 0x30, 0x33, 0x2f, 0x4a, 0x61, 0x6e, /* |s 03/Jan| */
	0x2f, 0x32, 0x30, 0x30, 0x39, 0x20, 0x43, 0x68, /* |/2009 Ch| */
	0x61, 0x6e, 0x63, 0x65, 0x6c, 0x6c, 0x6f, 0x72, /* |ancellor| */
	0x20, 0x6f, 0x6e, 0x20, 0x62, 0x72, 0x69, 0x6e, /* | on brin| */
	0x6b, 0x20, 0x6f, 0x66, 0x20, 0x73, 0x65, 0x63, /* |k of sec| */
	0x6f, 0x6e, 0x64, 0x20, 0x62, 0x61, 0x69, 0x6c, /* |ond bail| */
	0x6f, 0x75, 0x74, 0x20, 0x66, 0x6f, 0x72, 0x20, /* |out for | */
	0x62, 0x61, 0x6e, 0x6b, 0x73,                   /* |banks| */
}

// Length: 77 bytes
```

### Use Cases

**1. Hardcoding Genesis Data in VM Code**

Convert your genesis block hex and paste it directly into your VM code:

```bash
# Generate genesis
go run main.go -address "1YourAddress..." > genesis.txt

# Extract hex
GENESIS_HEX=$(grep "Genesis Block (hex):" genesis.txt -A 1 | tail -1)

# Convert to Go code
./build/genesis-generator -hex2go "$GENESIS_HEX" > genesis_bytes.go
```

Then use in `vm/vm.go`:
```go
var genesisBlockBytes = []byte{
	0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, /* |........| */
	// ... rest of bytes
}

func (vm *VM) createDefaultGenesis() *wire.MsgBlock {
	msgBlock := &wire.MsgBlock{}
	if err := msgBlock.Deserialize(bytes.NewReader(genesisBlockBytes)); err != nil {
		log.Error("failed to deserialize genesis", "error", err)
		return vm.config.ChainParams.GenesisBlock
	}
	return msgBlock
}
```

**2. Creating Custom Coinbase Messages**

Convert your coinbase message to Go format:

```bash
# Convert text to hex first
echo -n "My Custom BTCVM Genesis" | xxd -p -c 1000

# Then convert to Go
./build/genesis-generator -hex2go "4d7920437573746f6d2042544356204d2047656e65736973"
```

**3. Encoding Transaction Scripts**

Convert Bitcoin script hex to Go byte arrays for custom transactions or test cases.

### Features

- **Automatic 0x prefix handling**: Strips `0x` or `0X` prefixes automatically
- **8 bytes per line**: Matches Bitcoin Core and btcd formatting conventions
- **ASCII comments**: Shows readable characters, `.` for non-printable
- **Proper alignment**: Maintains consistent spacing even on the last line
- **Length information**: Displays total byte count

## Advanced: Custom Parameters

You can customize various aspects of your genesis block:

```bash
go run main.go \
  -address "1YourAddress..." \
  -message "Custom message in coinbase" \
  -reward 10000000000 \           # 100 BTC (in satoshis)
  -timestamp 1231006505           # Specific timestamp
```

Parameters:
- `-address`: Bitcoin address to receive genesis reward
- `-message`: Message embedded in coinbase (max ~80 bytes)
- `-reward`: Coinbase reward in satoshis (default: 50 BTC = 5,000,000,000)
- `-timestamp`: Unix timestamp (default: current time)

## Troubleshooting

### "Invalid Bitcoin address"
- Ensure you're using a valid mainnet Bitcoin address
- Address should start with `1` (P2PKH) or `bc1` (P2WPKH)

### "Failed to create genesis block"
- Check that all parameters are valid
- Ensure you have enough memory for mining

### Mining takes too long
- The difficulty is set to Bitcoin genesis difficulty
- It should take a few seconds to a few minutes
- If it takes longer, consider lowering difficulty (requires code changes)

### Genesis block rejected
- Verify the hex is correct and complete
- Check that the block hash meets difficulty target
- Ensure coinbase transaction is valid

## Example: Complete Workflow

```bash
# 1. Generate keys
cd cmd/genesis-generator
go run main.go -generate > keys.txt

# 2. Extract address (save keys.txt securely!)
ADDRESS=$(grep "P2PKH" keys.txt | awk '{print $3}')

# 3. Create genesis block
go run main.go -address "$ADDRESS" -message "My BTCVM $(date)" > genesis.txt

# 4. Extract genesis hex
GENESIS_HEX=$(grep "Genesis Block (hex):" genesis.txt -A 1 | tail -1)

# 5. Save for later use
echo "$GENESIS_HEX" > ../../genesis.hex

# 6. Use when creating blockchain
echo "Genesis hex saved to genesis.hex"
echo "Use this when calling platform.createBlockchain"
```

## Next Steps

After creating your custom genesis:

1. **Test locally** with a single-node network
2. **Verify** you can query the genesis block via RPC
3. **Test transactions** to ensure your keys work
4. **Document** your genesis parameters for your network
5. **Distribute** the genesis configuration to network participants

## Support

For issues or questions:
- Check the logs: `~/.metalgo/logs/`
- Review genesis block with RPC: `btcvm.GetBlock`
- Verify with block explorer tools
