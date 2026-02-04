#!/bin/bash

set -e

# Configuration
NODE_URL=${NODE_URL:-"http://127.0.0.1:9650"}
CHAIN_NAME=${CHAIN_NAME:-"btcvm"}

echo "Testing BTCVM RPC endpoints..."
echo "Node URL: $NODE_URL"
echo ""

# Function to make RPC call
function rpc_call() {
    local method=$1
    local params=$2
    local chain_id=$3

    curl -s -X POST --data "{
        \"jsonrpc\": \"2.0\",
        \"id\": 1,
        \"method\": \"$method\",
        \"params\": [$params]
    }" -H 'content-type:application/json;' "$NODE_URL/ext/bc/$chain_id/rpc"
}

# Function to make platform API call
function platform_call() {
    local method=$1
    local params=$2

    curl -s -X POST --data "{
        \"jsonrpc\": \"2.0\",
        \"id\": 1,
        \"method\": \"$method\",
        \"params\": {$params}
    }" -H 'content-type:application/json;' "$NODE_URL/ext/P"
}

echo "Step 1: Getting blockchain list..."
BLOCKCHAINS=$(platform_call "platform.getBlockchains" "")
echo "$BLOCKCHAINS" | jq '.'

# Extract chain ID for btcvm
CHAIN_ID=$(echo "$BLOCKCHAINS" | jq -r ".result.blockchains[] | select(.name==\"$CHAIN_NAME\") | .id")

if [ -z "$CHAIN_ID" ] || [ "$CHAIN_ID" = "null" ]; then
    echo ""
    echo "ERROR: Could not find chain with name '$CHAIN_NAME'"
    echo "Available chains:"
    echo "$BLOCKCHAINS" | jq -r '.result.blockchains[] | "  - \(.name) (\(.id))"'
    echo ""
    echo "Please ensure the BTCVM is running and try again."
    echo "Or set CHAIN_NAME environment variable to the correct chain name."
    exit 1
fi

echo ""
echo "Found chain: $CHAIN_NAME"
echo "Chain ID: $CHAIN_ID"
echo ""
echo "========================================="
echo ""

# Test 1: GetNetworkInfo
echo "Test 1: GetNetworkInfo"
echo "------------------------"
RESULT=$(rpc_call "btcvm.GetNetworkInfo" "{}" "$CHAIN_ID")
echo "$RESULT" | jq '.'
echo ""

# Test 2: GetHealth
echo "Test 2: GetHealth"
echo "------------------------"
RESULT=$(rpc_call "btcvm.GetHealth" "{}" "$CHAIN_ID")
echo "$RESULT" | jq '.'
echo ""

# Test 3: GetLastAccepted
echo "Test 3: GetLastAccepted"
echo "------------------------"
RESULT=$(rpc_call "btcvm.GetLastAccepted" "{}" "$CHAIN_ID")
echo "$RESULT" | jq '.'
echo ""

# Test 4: GetBlock (genesis block at height 0)
echo "Test 4: GetBlock (height=0)"
echo "------------------------"
RESULT=$(rpc_call "btcvm.GetBlock" "{\"height\": 0}" "$CHAIN_ID")
echo "$RESULT" | jq '.'
echo ""

# Test 5: GetMempool
echo "Test 5: GetMempool"
echo "------------------------"
RESULT=$(rpc_call "btcvm.GetMempool" "{}" "$CHAIN_ID")
echo "$RESULT" | jq '.'
echo ""

# Test 6: Test error handling - invalid block
echo "Test 6: Error Handling (invalid block height)"
echo "------------------------"
RESULT=$(rpc_call "btcvm.GetBlock" "{\"height\": 99999}" "$CHAIN_ID")
echo "$RESULT" | jq '.'
echo ""

echo "========================================="
echo "RPC endpoint tests completed!"
echo ""
echo "To make manual RPC calls, use:"
echo "  CHAIN_ID=$CHAIN_ID"
echo ""
echo "Example:"
echo "  curl -X POST --data '{"
echo "      \"jsonrpc\": \"2.0\","
echo "      \"id\": 1,"
echo "      \"method\": \"btcvm.GetNetworkInfo\","
echo "      \"params\": [{}]"
echo "  }' -H 'content-type:application/json;' $NODE_URL/ext/bc/$CHAIN_ID/rpc | jq"
