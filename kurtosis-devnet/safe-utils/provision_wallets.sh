#!/bin/bash

# --contracts-input should point to the output of deploy_contracts.sh

# --roles-spec should be a json file structured as
# request_example.json. It associates each role to a set of owners, and
# a threshold of signatures to collect.

# The output will be of the form:
# { "role": "safe_address", ... }

# Parse command line arguments
while [ $# -gt 0 ]; do
    case "$1" in
        --private-key)
            export PK="$2"
            shift 2
            ;;
        --rpc-url)
            export NODE_URL="$2"
            shift 2
            ;;
        --contracts-input)
            export CONTRACTS_JSON="$2"
            shift 2
            ;;
        --roles-spec)
            export ROLES_JSON="$2"
            shift 2
            ;;
        --safes-output)
            export OUTPUT="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 --private-key <key> --rpc-url <url> --contracts-input <file> --roles-spec <file> --safes-output <file>"
            exit 1
            ;;
    esac
done

# Validate required arguments
if [ -z "$PK" ] || [ -z "$NODE_URL" ] || [ -z "$CONTRACTS_JSON" ] || [ -z "$ROLES_JSON" ] || [ -z "$OUTPUT" ]; then
    echo "Missing required arguments"
    echo "Usage: $0 --private-key <key> --rpc-url <url> --contracts-input <file> --roles-spec <file> --safes-output <file>"
    exit 1
fi

export PATH=/venv/bin:$PATH

if [ ! -f "$CONTRACTS_JSON" ]; then
    echo "Error: $CONTRACTS_JSON not found. Did you run deploy_contracts.sh?"
    exit 1
fi

if [ ! -f "$ROLES_JSON" ]; then
    echo "Error: $ROLES_JSON not found. Please provide a valid roles specification file."
    exit 1
fi

EXT_FALLBACK_HANDLER=$(cat "$CONTRACTS_JSON" | jq -r .ExtensibleFallbackHandler)
SAFE=$(cat "$CONTRACTS_JSON" | jq -r .Safe)
SAFE_PROXY_FACTORY=$(cat "$CONTRACTS_JSON" | jq -r .SafeProxyFactory)

# just something to make the calls repeatable
SALT_NONCE=1234567890

# Start with an empty JSON document
echo "{}" > "$OUTPUT"

# Iterate through each role in the JSON file
for role in $(cat "$ROLES_JSON" | jq -r 'keys[]'); do
    echo "Creating Safe wallet for role: $role"

    # Extract owners and threshold for this role
    owners=$(cat "$ROLES_JSON" | jq -r ".$role.owners | join(\" \")")
    threshold=$(cat "$ROLES_JSON" | jq -r ".$role.threshold")

    # Run safe-creator and capture the output
    safe_output=$(safe-creator \
        --callback-handler "$EXT_FALLBACK_HANDLER" \
        --safe-contract "$SAFE" \
        --proxy-factory "$SAFE_PROXY_FACTORY" \
        --salt-nonce "$SALT_NONCE" \
        --owners "$owners" \
        --threshold "$threshold" \
        --no-confirm \
        "$NODE_URL" "$PK" 2>&1)

    # Extract the safe address from the output
    safe_address=$(echo "$safe_output" | grep -E "(contract_address=|Safe on)" | head -1 | sed -E "s/.*contract_address='([^']*)'.*/\1/" | sed -E 's/.*Safe on (0x[0-9a-fA-F]*).*/\1/')

    # Add the role and safe address to the output JSON
    jq --arg role "$role" --arg address "$safe_address" '. + {($role): $address}' "$OUTPUT" > "$OUTPUT.tmp" && mv "$OUTPUT.tmp" "$OUTPUT"
done

# Display the result
echo "Deployed Safes:"
cat "$OUTPUT"
