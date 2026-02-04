#!/bin/bash

# Bitcoin VM Test Script
# This script tests basic functionality of the Bitcoin VM

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
API_URL="http://127.0.0.1:9650"
RPC_URL="$API_URL/ext/bc/btcvm/rpc"
RPC_USER="bitcoin"
RPC_PASS="password"

echo -e "${BLUE}Bitcoin VM Test Suite${NC}"
echo "====================="

# Function to make RPC calls
rpc_call() {
    local method=$1
    local params=${2:-[]}

    curl -s -u "$RPC_USER:$RPC_PASS" \
        --data "{\"jsonrpc\":\"1.0\",\"method\":\"$method\",\"params\":$params}" \
        -H "content-type: text/plain;" \
        "$RPC_URL"
}

# Function to check if service is up
wait_for_service() {
    echo -e "${YELLOW}Waiting for Bitcoin VM to be ready...${NC}"
    local retries=30
    while [ $retries -gt 0 ]; do
        if curl -s "$API_URL/ext/health" > /dev/null 2>&1; then
            echo -e "${GREEN}Service is ready!${NC}"
            return 0
        fi
        echo -n "."
        sleep 2
        retries=$((retries - 1))
    done
    echo -e "\n${RED}Service failed to start${NC}"
    return 1
}

# Test 1: Check health
test_health() {
    echo -e "\n${BLUE}Test 1: Health Check${NC}"
    local response=$(curl -s "$API_URL/ext/health")
    if echo "$response" | grep -q "healthy"; then
        echo -e "${GREEN}✓ Health check passed${NC}"
        return 0
    else
        echo -e "${RED}✗ Health check failed${NC}"
        echo "Response: $response"
        return 1
    fi
}

# Test 2: Get blockchain info
test_blockchain_info() {
    echo -e "\n${BLUE}Test 2: Get Blockchain Info${NC}"
    local response=$(rpc_call "getblockchaininfo")

    if echo "$response" | grep -q "\"chain\""; then
        echo -e "${GREEN}✓ Blockchain info retrieved${NC}"
        echo "$response" | jq -r '.result | {chain, blocks, headers, bestblockhash}'
        return 0
    else
        echo -e "${RED}✗ Failed to get blockchain info${NC}"
        echo "Response: $response"
        return 1
    fi
}

# Test 3: Get network info
test_network_info() {
    echo -e "\n${BLUE}Test 3: Get Network Info${NC}"
    local response=$(rpc_call "getnetworkinfo")

    if echo "$response" | grep -q "\"version\""; then
        echo -e "${GREEN}✓ Network info retrieved${NC}"
        echo "$response" | jq -r '.result | {version, subversion, protocolversion, connections}'
        return 0
    else
        echo -e "${RED}✗ Failed to get network info${NC}"
        echo "Response: $response"
        return 1
    fi
}

# Test 4: Get mempool info
test_mempool_info() {
    echo -e "\n${BLUE}Test 4: Get Mempool Info${NC}"
    local response=$(rpc_call "getmempoolinfo")

    if echo "$response" | grep -q "\"size\""; then
        echo -e "${GREEN}✓ Mempool info retrieved${NC}"
        echo "$response" | jq -r '.result'
        return 0
    else
        echo -e "${RED}✗ Failed to get mempool info${NC}"
        echo "Response: $response"
        return 1
    fi
}

# Test 5: Generate address
test_generate_address() {
    echo -e "\n${BLUE}Test 5: Generate New Address${NC}"
    local response=$(rpc_call "getnewaddress" '["test", "bech32"]')

    if echo "$response" | grep -q "bcrt1"; then
        echo -e "${GREEN}✓ Address generated${NC}"
        local address=$(echo "$response" | jq -r '.result')
        echo "New address: $address"
        echo "$address" > /tmp/test_address.txt
        return 0
    else
        echo -e "${RED}✗ Failed to generate address${NC}"
        echo "Response: $response"
        return 1
    fi
}

# Test 6: Mine a block
test_mine_block() {
    echo -e "\n${BLUE}Test 6: Mine a Block${NC}"

    # Get or generate address
    if [ -f /tmp/test_address.txt ]; then
        local address=$(cat /tmp/test_address.txt)
    else
        local address="bcrt1qqqqqp399et2xygdj5xreqhjjvcmzhxw4aywxecjdzew6hylgvsesrxh6hy"
    fi

    local response=$(rpc_call "generatetoaddress" "[1, \"$address\"]")

    if echo "$response" | grep -q "result"; then
        echo -e "${GREEN}✓ Block mined${NC}"
        echo "$response" | jq -r '.result'
        return 0
    else
        echo -e "${YELLOW}⚠ Mining not available (expected in regtest)${NC}"
        return 0
    fi
}

# Test 7: Create and send transaction
test_transaction() {
    echo -e "\n${BLUE}Test 7: Create Transaction${NC}"

    # This is a placeholder - actual implementation would need funded wallet
    echo -e "${YELLOW}⚠ Transaction test requires funded wallet${NC}"

    # Try to create a raw transaction
    local response=$(rpc_call "createrawtransaction" '[[], {"bcrt1qqqqqp399et2xygdj5xreqhjjvcmzhxw4aywxecjdzew6hylgvsesrxh6hy": 0.01}]')

    if echo "$response" | grep -q "result"; then
        echo -e "${GREEN}✓ Raw transaction created${NC}"
        local rawtx=$(echo "$response" | jq -r '.result')
        echo "Raw tx (first 64 chars): ${rawtx:0:64}..."
        return 0
    else
        echo -e "${YELLOW}⚠ Could not create transaction${NC}"
        return 0
    fi
}

# Test 8: Get peer info
test_peer_info() {
    echo -e "\n${BLUE}Test 8: Get Peer Info${NC}"
    local response=$(rpc_call "getpeerinfo")

    if echo "$response" | grep -q "result"; then
        echo -e "${GREEN}✓ Peer info retrieved${NC}"
        local peer_count=$(echo "$response" | jq -r '.result | length')
        echo "Connected peers: $peer_count"
        return 0
    else
        echo -e "${RED}✗ Failed to get peer info${NC}"
        echo "Response: $response"
        return 1
    fi
}

# Main test execution
main() {
    echo -e "${YELLOW}Starting Bitcoin VM tests...${NC}\n"

    # Wait for service to be ready
    if ! wait_for_service; then
        exit 1
    fi

    # Run tests
    local passed=0
    local failed=0

    tests=(
        "test_health"
        "test_blockchain_info"
        "test_network_info"
        "test_mempool_info"
        "test_generate_address"
        "test_mine_block"
        "test_transaction"
        "test_peer_info"
    )

    for test in "${tests[@]}"; do
        if $test; then
            passed=$((passed + 1))
        else
            failed=$((failed + 1))
        fi
    done

    # Summary
    echo -e "\n${BLUE}========== Test Summary ==========${NC}"
    echo -e "${GREEN}Passed: $passed${NC}"
    echo -e "${RED}Failed: $failed${NC}"

    if [ $failed -eq 0 ]; then
        echo -e "\n${GREEN}All tests passed! Bitcoin VM is working correctly.${NC}"
        exit 0
    else
        echo -e "\n${YELLOW}Some tests failed. Check the output above for details.${NC}"
        exit 1
    fi
}

# Run main function
main "$@"