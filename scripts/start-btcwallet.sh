#!/bin/bash

# Script to start btcwallet connected to BTCVM
# This wallet connects to the BTCVM blockchain and provides RPC interface

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Check if CHAIN_ID is set
if [ -z "$CHAIN_ID" ]; then
    echo -e "${RED}Error: CHAIN_ID environment variable is not set${NC}"
    echo ""
    echo "Please set your BTCVM blockchain's chain ID:"
    echo "  export CHAIN_ID=your-chain-id-here"
    echo ""
    echo "Example:"
    echo "  export CHAIN_ID=oS9Ror7f9mtwTWm5LFGvzxdXJVt8cvLf5nky3qvj6CeWNNs6H"
    exit 1
fi

# Check if btcwallet is installed
if ! command -v btcwallet &> /dev/null; then
    echo -e "${RED}Error: btcwallet is not installed${NC}"
    echo ""
    echo "Please install btcwallet from:"
    echo "  https://github.com/btcsuite/btcwallet"
    echo ""
    echo "Or install with Go:"
    echo "  go install github.com/btcsuite/btcwallet@latest"
    exit 1
fi

# Configuration
BTCVM_RPC_HOST="${BTCVM_RPC_HOST:-localhost:9650}"
WALLET_RPC_USER="${WALLET_RPC_USER:-test}"
WALLET_RPC_PASS="${WALLET_RPC_PASS:-test}"
WALLET_DATA_DIR="${WALLET_DATA_DIR:-$HOME/.btcwallet}"

echo -e "${BLUE}=== Starting btcwallet for BTCVM ===${NC}"
echo ""
echo "Configuration:"
echo "  Chain ID: $CHAIN_ID"
echo "  BTCVM RPC: $BTCVM_RPC_HOST/ext/bc/$CHAIN_ID/rpc"
echo "  Wallet RPC: localhost:8332"
echo "  Data Dir: $WALLET_DATA_DIR"
echo ""

# Check if wallet exists
if [ ! -f "$WALLET_DATA_DIR/mainnet/wallet.db" ]; then
    echo -e "${YELLOW}Wallet database not found. Creating new wallet...${NC}"
    echo ""
    echo "You will be prompted to:"
    echo "  1. Enter a private passphrase (encrypts private keys)"
    echo "  2. Enter a public passphrase (optional)"
    echo "  3. Input a seed (or generate new one)"
    echo ""
    read -p "Press Enter to create wallet or Ctrl+C to cancel..."

    btcwallet --create --datadir="$WALLET_DATA_DIR"

    if [ $? -ne 0 ]; then
        echo -e "${RED}Failed to create wallet${NC}"
        exit 1
    fi

    echo ""
    echo -e "${GREEN}Wallet created successfully!${NC}"
    echo ""
fi

echo -e "${GREEN}Starting btcwallet...${NC}"
echo ""
echo "btcwallet will connect to BTCVM and listen for:"
echo "  - JSON-RPC on port 8332 (for btcvm-wallet CLI and btcctl)"
echo ""
echo "Use Ctrl+C to stop"
echo ""

# Start btcwallet
btcwallet \
  --datadir="$WALLET_DATA_DIR" \
  --noservertls \
  --noclienttls \
  --rpcconnect="$BTCVM_RPC_HOST/ext/bc/${CHAIN_ID}/rpc" \
  --username="$WALLET_RPC_USER" \
  --password="$WALLET_RPC_PASS" \
  --debuglevel=info

echo ""
echo -e "${YELLOW}btcwallet stopped${NC}"
