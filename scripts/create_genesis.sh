#!/bin/bash

set -e

echo "========================================="
echo "BTCVM Custom Genesis Block Creator"
echo "========================================="
echo ""

# Check if genesis-generator exists
if [ ! -f "cmd/genesis-generator/main.go" ]; then
    echo "Error: genesis-generator not found"
    echo "Please run this script from the btcvm root directory"
    exit 1
fi

echo "What would you like to do?"
echo "1) Generate a new key pair"
echo "2) Create genesis block with an existing address"
echo ""
read -p "Enter choice [1-2]: " choice

case $choice in
    1)
        echo ""
        echo "Generating new key pair..."
        echo ""
        cd cmd/genesis-generator
        go run main.go -generate | tee ../../keys.txt
        echo ""
        echo "========================================="
        echo "⚠️  IMPORTANT: Your keys have been saved to keys.txt"
        echo "Please move this file to a secure location!"
        echo "========================================="
        echo ""

        # Extract address for next step
        ADDRESS=$(grep "P2PKH" ../../keys.txt | awk '{print $3}')

        read -p "Would you like to create a genesis block with this address now? [y/N]: " create_now
        if [[ $create_now =~ ^[Yy]$ ]]; then
            echo ""
            read -p "Enter coinbase message (optional): " MESSAGE
            if [ -z "$MESSAGE" ]; then
                MESSAGE="BTCVM Genesis Block - $(date)"
            fi

            echo ""
            echo "Creating genesis block..."
            go run main.go -address "$ADDRESS" -message "$MESSAGE" | tee ../../genesis-config.txt

            # Extract hex
            GENESIS_HEX=$(grep "Genesis Block (hex):" ../../genesis-config.txt -A 1 | tail -1 | tr -d '[:space:]')
            echo "$GENESIS_HEX" > ../../genesis.hex

            echo ""
            echo "Genesis block hex saved to genesis.hex"
            echo "Configuration saved to genesis-config.txt"
        fi
        ;;

    2)
        echo ""
        read -p "Enter Bitcoin address: " ADDRESS

        if [ -z "$ADDRESS" ]; then
            echo "Error: Address cannot be empty"
            exit 1
        fi

        echo ""
        read -p "Enter coinbase message (optional): " MESSAGE
        if [ -z "$MESSAGE" ]; then
            MESSAGE="BTCVM Genesis Block - $(date)"
        fi

        read -p "Enter coinbase reward in BTC [default: 50]: " REWARD_BTC
        if [ -z "$REWARD_BTC" ]; then
            REWARD_BTC=50
        fi

        # Convert BTC to satoshis
        REWARD_SATS=$(echo "$REWARD_BTC * 100000000" | bc | cut -d'.' -f1)

        echo ""
        echo "Creating genesis block..."
        cd cmd/genesis-generator
        go run main.go \
            -address "$ADDRESS" \
            -message "$MESSAGE" \
            -reward "$REWARD_SATS" | tee ../../genesis-config.txt

        # Extract hex
        GENESIS_HEX=$(grep "Genesis Block (hex):" ../../genesis-config.txt -A 1 | tail -1 | tr -d '[:space:]')
        echo "$GENESIS_HEX" > ../../genesis.hex

        echo ""
        echo "Genesis block hex saved to genesis.hex"
        echo "Configuration saved to genesis-config.txt"
        ;;

    *)
        echo "Invalid choice"
        exit 1
        ;;
esac

echo ""
echo "========================================="
echo "Next Steps"
echo "========================================="
echo ""
echo "1. Review your genesis configuration in genesis-config.txt"
echo "2. Secure your private key (if generated)"
echo "3. Use genesis.hex when creating your blockchain:"
echo ""
echo "   GENESIS_HEX=\$(cat genesis.hex)"
echo "   curl -X POST --data \"{"
echo "       \\\"jsonrpc\\\": \\\"2.0\\\","
echo "       \\\"id\\\": 1,"
echo "       \\\"method\\\": \\\"platform.createBlockchain\\\","
echo "       \\\"params\\\": {"
echo "           \\\"subnetID\\\": \\\"11111111111111111111111111111111LpoYY\\\","
echo "           \\\"vmID\\\": \\\"kMtihm7W3KssmcJb9mzwZfC6gkiPrJhWaa5KMLHdEB9R8Q4pp\\\","
echo "           \\\"name\\\": \\\"btcvm\\\","
echo "           \\\"genesisData\\\": \\\"\$GENESIS_HEX\\\""
echo "       }"
echo "   }\" -H 'content-type:application/json;' http://127.0.0.1:9650/ext/P"
echo ""
echo "See GENESIS_SETUP.md for more details"
echo ""
