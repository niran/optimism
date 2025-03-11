#!/usr/bin/env bash

set -e  # Exit immediately if a command exits with a non-zero status

# Usage function
usage() {
    echo "Usage: $0 <vm-profile> <baseline-report>"
    echo "Valid profiles: cannon-singlethreaded-32, cannon-multithreaded-64"
    exit 1
}

# Validate input
if [[ $# -ne 2 ]]; then
    usage
fi

VM_PROFILE=$1
BASELINE_REPORT=$2

if [[ "$VM_PROFILE" != "cannon-singlethreaded-32" && "$VM_PROFILE" != "cannon-multithreaded-64" ]]; then
    echo "Error: Invalid vm-profile '$VM_PROFILE'"
    usage
fi

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
BIN_DIR="bin"
ANALYZER_BIN="${BIN_DIR}/analyzer"

# Normalize architecture naming
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
esac

# Detect package manager
if [[ "$OS" == "linux" ]]; then
    if command -v apt-get &>/dev/null; then
        PKG_MANAGER="sudo apt-get install -y"
    elif command -v yum &>/dev/null; then
        PKG_MANAGER="sudo yum install -y"
    elif command -v pacman &>/dev/null; then
        PKG_MANAGER="sudo pacman -Sy --noconfirm"
    else
        echo "No supported package manager found. Install llvm manually."
        exit 1
    fi
elif [[ "$OS" == "darwin" ]]; then
    PKG_MANAGER="brew install"
else
    echo "Unsupported OS: $OS"
    exit 1
fi

# Detect LLVM path
LLVM_PATH=""
if [[ "$OS" == "darwin" ]]; then
    if [[ "$ARCH" == "amd64" ]]; then
        LLVM_PATH="/usr/local/opt/llvm/bin"
    else
        LLVM_PATH="/opt/homebrew/opt/llvm/bin"
    fi
elif [[ "$OS" == "linux" ]]; then
    if command -v brew &>/dev/null; then
        LLVM_PATH="/home/linuxbrew/.linuxbrew/opt/llvm/bin"
    elif [[ -f "/usr/bin/llvm-objdump" ]]; then
        LLVM_PATH="/usr/bin"
    fi
fi

# Install llvm-objdump if missing
if ! command -v llvm-objdump &>/dev/null; then
    echo "llvm-objdump not found, installing..."
    $PKG_MANAGER llvm
else
    echo "llvm-objdump found at $(which llvm-objdump)"
fi

# Export LLVM_PATH if detected
if [[ -n "$LLVM_PATH" ]]; then
    echo "Adding $LLVM_PATH to PATH"
    export PATH="$LLVM_PATH:$PATH"
fi

# Define the binary based on OS and ARCH
ANALYZER_URL="https://github.com/ChainSafe/vm-compat/releases/latest/download/analyzer-${OS}-${ARCH}"

# Fetch Analyzer if not present
if [[ -f "$ANALYZER_BIN" ]]; then
    echo "Analyzer binary already exists, skipping download."
else
    echo "Fetching Analyzer Binary for ${OS}-${ARCH}..."
    mkdir -p "$BIN_DIR"
    curl -L -o "$ANALYZER_BIN" "$ANALYZER_URL"
    chmod +x "$ANALYZER_BIN"
fi

# Run the analyzer
echo "Running analysis with VM profile: $VM_PROFILE using baseline report: $BASELINE_REPORT..."
OUTPUT_FILE=$(mktemp)

"$ANALYZER_BIN" analyze --with-trace=true --format=json --vm-profile "$VM_PROFILE" --baseline-report "$BASELINE_REPORT" --skip-warnings --report-output-path "$OUTPUT_FILE" ./client/cmd/main.go

# Check if JSON output contains any issues
ISSUE_COUNT=$(jq 'length' "$OUTPUT_FILE")

if [ "$ISSUE_COUNT" -gt 0 ]; then
    echo "❌ Analysis found $ISSUE_COUNT issues!"
    cat "$OUTPUT_FILE"
    rm -f "$OUTPUT_FILE"
    exit 1
else
    echo "✅ No issues found."
    rm -f "$OUTPUT_FILE"
    exit 0
fi
