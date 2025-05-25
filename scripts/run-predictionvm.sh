#!/usr/bin/env bash
set -e

# Script to run an AvalancheGo node with the PredictionVM plugin.

# Environment variables from Dockerfile.predictionvm
# HYPERSDK_DIR_PATH=/root/.hypersdk
# AVALANCHEGO_VERSION_SEMVER=v1.11.12-rc.2 (example)
# AVALANCHEGO_INSTALL_PATH=${HYPERSDK_DIR_PATH}/avalanchego-${AVALANCHEGO_VERSION_SEMVER}
# AVALANCHEGO_EXEC_PATH=${AVALANCHEGO_INSTALL_PATH}/avalanchego
# AVALANCHEGO_PLUGINS_DIR=${AVALANCHEGO_INSTALL_PATH}/plugins

PREDICTIONVM_ID="2YnmQD1F6XYx45WTv84R5ar7WDbqzW9pNP4gENbGo6oSJ29QfC"

# Define directories
DATA_DIR="${HYPERSDK_DIR_PATH}/data/${PREDICTIONVM_ID}"
CONFIGS_DIR="${HYPERSDK_DIR_PATH}/configs"
CHAIN_CONFIGS_DIR="${CONFIGS_DIR}/chains/${PREDICTIONVM_ID}"
SUBNET_CONFIG_DIR="${CONFIGS_DIR}/subnets"

# Create directories if they don't exist
mkdir -p "${DATA_DIR}/db" # Also create db and logs subdir for clarity
mkdir -p "${DATA_DIR}/logs"
mkdir -p "${CHAIN_CONFIGS_DIR}"
mkdir -p "${SUBNET_CONFIG_DIR}"

# --- Create PredictionVM Genesis File ---
PREDICTIONVM_GENESIS_FILE="${CHAIN_CONFIGS_DIR}/genesis.json"
if [ ! -f "${PREDICTIONVM_GENESIS_FILE}" ]; then
  echo "Creating PredictionVM genesis file at ${PREDICTIONVM_GENESIS_FILE}"
  # This genesis content needs to match what your VM's `Genesis()` method expects.
  # If your VM uses `genesis.DefaultGenesis()`, it might expect allocations and a config section.
  # The following is a generic example based on hypersdk defaults.
  # You may need to adjust this based on `predictionvm/genesis/genesis.go` or similar.
  cat <<EOF > "${PREDICTIONVM_GENESIS_FILE}"
{
  "magic": 727272,
  "timestamp": $(date +%s),
  "custom": {
    "markets": [
      {
        "id": 1,
        "question": "Will X happen by Y date?",
        "closingTime": $(($(date +%s) + 86400)),
        "collateralAssetId": "AVAX"
      }
    ]
  },
  "allocations": []
}
EOF
fi

# --- Create Chain Config File for PredictionVM ---
CHAIN_CONFIG_FILE="${CHAIN_CONFIGS_DIR}/config.json"
if [ ! -f "${CHAIN_CONFIG_FILE}" ]; then
  echo "Creating PredictionVM chain config file at ${CHAIN_CONFIG_FILE}"
  cat <<EOF > "${CHAIN_CONFIG_FILE}"
{
  "vm-id": "${PREDICTIONVM_ID}",
  "enabled": true
}
EOF
fi

# --- Create Main Node Config File for Chain Aliases ---
MAIN_NODE_CONFIG_FILE="${CONFIGS_DIR}/config.json"
if [ ! -f "${MAIN_NODE_CONFIG_FILE}" ]; then
  echo "Creating main node config file at ${MAIN_NODE_CONFIG_FILE} for chain aliases"
  cat <<EOF > "${MAIN_NODE_CONFIG_FILE}"
{
  "chain-aliases": {
    "pvm": ["${PREDICTIONVM_ID}"]
  }
}
EOF
fi

# --- Subnet Config File (Simplified for Dev) ---
# For a simple dev setup with staking disabled, explicit subnet configuration
# via a separate file and avalanchego-cli is often not strictly necessary.
# AvalancheGo can run the VM on the primary network or a default subnet.
# We will omit creating a specific subnet JSON file and related CLI calls.

# --- Start AvalancheGo Node ---
echo "Starting AvalancheGo node with PredictionVM..."

# Ensure AVALANCHEGO_EXEC_PATH is set (should be by Dockerfile ENV)
if [ -z "${AVALANCHEGO_EXEC_PATH}" ]; then
  echo "Error: AVALANCHEGO_EXEC_PATH is not set. Check Dockerfile ENV." >&2
  exit 1
fi

exec ${AVALANCHEGO_EXEC_PATH} \
  --network-id="local" \
  --db-type="leveldb" \
  --db-dir="${DATA_DIR}/db" \
  --log-dir="${DATA_DIR}/logs" \
  --http-host="0.0.0.0" \
  --http-port="9650" \
  --staking-port="9651" \
  --plugin-dir="${AVALANCHEGO_PLUGINS_DIR}" \
  --chain-config-dir="${CONFIGS_DIR}/chains" \
  --log-level="debug" \
  --config-file="${CONFIGS_DIR}/config.json"
  # --subnet-config-dir="${SUBNET_CONFIG_DIR}" # Removed as we are not creating specific subnet config
  # --track-subnets="..." # Removed for simplicity in dev mode
