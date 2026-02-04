# Quick Start: Custom Genesis Block

Create your own Bitcoin VM genesis block in 3 simple steps.

## Option 1: Interactive Script (Easiest)

```bash
./scripts/create_genesis.sh
```

Follow the prompts to:
1. Generate new keys OR use existing address
2. Create custom genesis block
3. Save genesis configuration

## Option 2: Command Line

### Step 1: Generate Keys

```bash
./build/genesis-generator -generate
```

**Output:**
```
Private Key (WIF): L44GYYv2EbWi7m1rJKLCAMsKTTNavWE22iRyBCANLfBu2VcYU9qs
Addresses:
  P2PKH (Legacy): 1NbKzrXqNYuNc9FBAoBjEi86P2nDKwbNDa
```

**⚠️ Save your private key securely!**

### Step 2: Create Genesis Block

```bash
./build/genesis-generator -address "1NbKzrXqNYuNc9FBAoBjEi86P2nDKwbNDa"
```

Optional parameters:
```bash
./build/genesis-generator \
  -address "1NbKzrXqNYuNc9FBAoBjEi86P2nDKwbNDa" \
  -message "My Custom Chain" \
  -reward 5000000000
```

This will mine a genesis block and output the hex.

### Step 3: Use Your Genesis

Save the genesis hex:
```bash
echo "YOUR_GENESIS_HEX" > genesis.hex
```

When creating your blockchain:
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

## Verify Your Genesis

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

## Complete Example

```bash
# 1. Build the generator
make build

# 2. Generate keys
./build/genesis-generator -generate > my-keys.txt

# ⚠️ BACKUP my-keys.txt SECURELY!

# 3. Extract address
ADDRESS=$(grep "P2PKH" my-keys.txt | awk '{print $3}')

# 4. Create genesis
./build/genesis-generator \
  -address "$ADDRESS" \
  -message "My BTCVM - $(date)" \
  > genesis-config.txt

# 5. Extract and save hex
grep "Genesis Block (hex):" genesis-config.txt -A 1 | \
  tail -1 | tr -d '[:space:]' > genesis.hex

# 6. Display
echo "Your address: $ADDRESS"
echo "Genesis hex saved to: genesis.hex"
echo ""
echo "⚠️ Backup my-keys.txt to a secure location!"
```

## Parameters

| Flag | Description | Default |
|------|-------------|---------|
| `-generate` | Generate new key pair | - |
| `-address` | Bitcoin address for genesis | Required |
| `-message` | Coinbase message | "BTCVM Genesis..." |
| `-reward` | Reward in satoshis | 5000000000 (50 BTC) |
| `-timestamp` | Unix timestamp | Current time |

## Security Checklist

- ✅ Generated keys on secure computer
- ✅ Backed up private key to secure location
- ✅ Backed up private key offline (paper/USB)
- ✅ Never shared private key
- ✅ Deleted plaintext keys from system
- ✅ Tested genesis block creation

## Need Help?

- **Full Guide**: `GENESIS_SETUP.md`
- **RPC Testing**: `TESTING_RPC.md`
- **Usage Examples**: `RPC_USAGE.md`

## Common Issues

**"Invalid Bitcoin address"**
- Use P2PKH address (starts with `1`)
- Or P2WPKH address (starts with `bc1`)

**"Mining takes too long"**
- Be patient, it can take 1-5 minutes
- Difficulty is same as Bitcoin genesis

**"Cannot find genesis-generator"**
- Run `make build` first
- Or `go build -o build/genesis-generator ./cmd/genesis-generator`
