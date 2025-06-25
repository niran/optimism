#!/bin/bash

# output will be a dictionary of the form
# { "contract1": "addr1", "contract2": "addr2", ... }

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
        --contracts-output)
            export OUTPUT="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 --private-key <key> --rpc-url <url> --contracts-output <file>"
            exit 1
            ;;
    esac
done

# Validate required arguments
if [ -z "$PK" ] || [ -z "$NODE_URL" ] || [ -z "$OUTPUT" ]; then
    echo "Missing required arguments"
    echo "Usage: $0 --private-key <key> --rpc-url <url> --contracts-output <file>"
    exit 1
fi

export PATH=/root/.foundry/bin:$PATH
export RPC=$NODE_URL

# silence stupid telemetry interactive question.
export CI=1

# create a hash of the versions involved
SINGLETON_SHA1=$(git --git-dir /safe-singleton-factory/.git rev-parse HEAD)
CONTRACT_SHA1=$(git --git-dir /safe-smart-account/.git rev-parse HEAD)

# a seed for new-mnemonic entropy. It doesn't have to be super robust, just to
# differentiate between successive runs if we operate under different
# conditions.
SEED=$(jq -n \
  --arg funded_wallet "$PK" \
  --arg singleton_sha1 "$SINGLETON_SHA1" \
  --arg contract_sha1 "$CONTRACT_SHA1" \
  '{
    "singleton_sha1": $singleton_sha1,
    "contract_sha1": $contract_sha1,
    "funded_wallet": $funded_wallet
  }' | md5sum | cut -d' ' -f1)

# we need a fresh mnemonic to own the singleton factory, because the installer requires its NONCE to be 0 to succeed.
TMP_MNEMONIC=$(cast wallet new-mnemonic -e "0x$SEED" --json | jq -r '.mnemonic')

# owner of the singleton factory
OWNER_ADDR=$(cast wallet address --mnemonic "$TMP_MNEMONIC")

NONCE=$(cast nonce --rpc-url "$NODE_URL" "$OWNER_ADDR")
if [ "$NONCE" -eq 0 ]; then # otherwise, we need to assume the singleton factory is already deployed
    # fund the owner of the singleton factory
    cast send --rpc-url "$NODE_URL" --private-key "$PK" --value 1ether "$OWNER_ADDR"

    # deploy the singleton factory
    pushd /safe-singleton-factory || exit 1
    MNEMONIC="$TMP_MNEMONIC" npm run estimate-compile
    MNEMONIC="$TMP_MNEMONIC" npm run submit
    popd || exit
fi

# This part is idempotent, so for clarify let's leave it alone. Worst case it's
# slightly wasteful when re-running.
pushd /safe-smart-account || exit 1
npx --yes hardhat --network custom deploy \
    | grep 0x \
    | jq -Rn '[inputs |
        if contains("reusing") then
            capture("reusing \"(?<key>.*)\" at (?<value>.*)")
        else
            capture("deploying \"(?<key>.*)\" \\(tx: [^)]+\\)\\.\\.\\.: deployed at (?<value>[0-9a-fA-Fx]+)")
        end
    ] | from_entries' \
    | tee "$OUTPUT"
npx hardhat --network custom local-verify
popd || exit
