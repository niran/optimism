#!/bin/bash

cast send --rpc-url "$ETH_RPC_URL" "$TO_ADDRESS" --data "$1"
