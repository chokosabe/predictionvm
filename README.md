# PredictionVM: Decentralized Prediction Markets on Avalanche

PredictionVM is a custom blockchain built using the Avalanche HyperSDK, designed to host decentralized prediction markets. Users can create, participate in, and resolve markets on various real-world events.

## Overview

The platform allows anyone to pose a question about a future event and create a market for it. Other users can then buy shares representing their belief in a particular outcome ("Yes" or "No"). Once the event's outcome is determined, the market is resolved, and users who held shares for the correct outcome can claim their payouts.

## Core Features

*   **Decentralized Markets:** No central authority controls market creation or resolution (resolution mechanisms to be defined by market creators or oracles).
*   **Market Creation:** Users can define new prediction markets with specific questions, resolution criteria, and end times.
*   **Participation:** Users can buy "Yes" or "No" shares for active markets.
*   **ERC-404 Token Standard:** Market shares (or the underlying participation tokens) are envisioned to utilize the ERC-404 standard. This experimental standard blends characteristics of fungible (ERC-20) and non-fungible (ERC-721) tokens. This could enable unique features for prediction market shares, such as fractional ownership of unique outcome-tied assets or novel trading mechanisms.
*   **Market Resolution:** A process for determining and recording the official outcome of a market.
*   **Claiming Payouts:** Users with winning shares can claim their portion of the market's pool.

## Getting Started

These instructions assume you have Docker and Docker Compose installed on your system.

### Prerequisites

*   [Docker](https://docs.docker.com/get-docker/)
*   [Docker Compose](https://docs.docker.com/compose/install/)

### Setup & Running

1.  **Clone the repository (if you haven't already):**
    ```bash
    git clone <repository-url>
    cd predictionvm
    ```

2.  **Build and Run with Docker Compose:**
    The `docker-compose.yml` file is configured to build the PredictionVM plugin and run an AvalancheGo node with the VM enabled.
    ```bash
    docker-compose up -d
    ```
    This command will:
    *   Build the `predictionvm_node` Docker image (which includes compiling the PredictionVM Go code).
    *   Start the AvalancheGo node in detached mode.
    *   The PredictionVM will be accessible via the chain alias `pvm`.

3.  **View Logs:**
    To see the logs from the running node and VM:
    ```bash
    docker-compose logs -f predictionvm_node
    ```

### Interacting with the API

Once the node is running and the PredictionVM chain (`pvm`) has bootstrapped, you can interact with its custom API endpoints. The API is typically available at:
`http://localhost:9650/ext/bc/pvm/predictionapi`

(Note: The exact API endpoints and request/response formats will be defined in the `predictionapi` service within the VM.)

Example (conceptual) `curl` to check genesis (once API is confirmed working):
```bash
curl -X POST --data '{
    "jsonrpc":"2.0",
    "id"     :1,
    "method" :"predictionapi.Genesis"
}' -H 'content-type:application/json;' http://localhost:9650/ext/bc/pvm/predictionapi
```

## Development

PredictionVM is built with:

*   Go
*   Avalanche HyperSDK

Key directories:

*   `vm/`: Core VM logic, including state transitions and actions.
*   `controller/`: Implements the `chain.Controller` interface for the VM.
*   `actions/`: Defines the different types of transactions (e.g., create market, buy shares).
*   `genesis/`: Defines the structure of the genesis file for the VM.
*   `cmd/predictionvm/`: Main package for the VM plugin.
*   `scripts/`: Helper scripts for running and managing the VM.
*   `api/` or `rpc/`: (Likely location for API service definitions - to be confirmed based on `predictionapi` implementation).

## Contributing

(Details to be added - e.g., contribution guidelines, code style.)

## License

(Specify your project's license - e.g., MIT, Apache 2.0. If not yet decided, you can put "To be determined".)
