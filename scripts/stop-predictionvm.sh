#!/usr/bin/env bash
set -e

# Script to stop the AvalancheGo node.

echo "Attempting to stop AvalancheGo node..."

# Try to find and kill the avalanchego process.
# Using pkill for simplicity in a dev environment.
if pgrep -x "avalanchego" > /dev/null
then
    pkill -x "avalanchego"
    echo "AvalancheGo process found and stop signal sent."
    # Wait a moment for it to shut down
    sleep 2
    if pgrep -x "avalanchego" > /dev/null
    then
        echo "AvalancheGo still running, sending SIGKILL..."
        pkill -9 -x "avalanchego"
    else
        echo "AvalancheGo stopped successfully."
    fi
else
    echo "AvalancheGo process not found."
fi

# Clean up lock file if it exists (adjust path if necessary based on avalanchego config)
# This path might vary or not be used depending on the version and config.
LOCK_FILE="${HYPERSDK_DIR_PATH}/data/${PREDICTIONVM_ID}/db/LOCK"
if [ -f "${LOCK_FILE}" ]; then
    echo "Removing database lock file: ${LOCK_FILE}"
    rm -f "${LOCK_FILE}"
fi

exit 0
