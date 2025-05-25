# HyperSDK Client

The HyperSDK Client is a TypeScript/JavaScript library designed to interact with the HyperSDK blockchain. It provides a set of tools and utilities to connect to the blockchain, manage accounts, and execute transactions.

## Features

- Connect to the HyperSDK blockchain using different signer types (Metamask Snap, Ephemeral, Private Key).
- Fetch and manage account balances.
- Execute read-only actions and transactions.
- Encode and decode data using the ABI (Application Binary Interface).

## Installation

```bash
npm install hypersdk-client
```

## Example Usage

To use the HyperSDK Client, make sure you have the latest version installed. You can refer to the JavaScript examples in the [HyperSDK Starter](https://github.com/ava-labs/hypersdk-starter) repository for more detailed usage. Please note that the API and features are evolving rapidly, and there may be frequent changes. The v1.0 release is expected to stabilize the user experience.

## Changelog

### 0.4.14
- Added support for the new 4-byte hash suffix for addresses

### 0.4.13
- TS to JS compilation

### 0.4.12
- Move Snap into its own [repo](https://github.com/ava-labs/hypersdk-snap)
- Move files around, but keep the API the same. You need to update imports

### 0.4.11
- Support PR [Simulate a chain of actions #1635](https://github.com/ava-labs/hypersdk/pull/1635)

### 0.4.10
- Support `bool` fields in ABI. [HyperSDK PR](https://github.com/ava-labs/hypersdk/pull/1648)

### 0.4.9
- Updated the indexer API to the latest iteration.
- Simplified indexer transformation in favor of easier support.

### 0.4.8
- Updated the indexer API to the [latest iteration](https://github.com/ava-labs/hypersdk/pull/1606)
- Added support for block subscriptions
- Note: internal changes require reinstalling the Snap!

### 0.4.7
- Added support for the [new indexer API](https://github.com/ava-labs/hypersdk/pull/1597)
- Renamed most of the methods. Now they are `simulateAction`, `sendTransaction`, `formatNativeTokens`, `convertToNativeTokens`
- Removed WebSocket support and replaced it with indexer API
- Added `getTransaction` method to get the transaction status

### 0.4.6
- The HyperSDK client is no longer an abstract class and does not need to be extended
- Added support for [Go language arrays](https://github.com/ava-labs/hypersdk/pull/1587)
- Implemented support for WebSockets for transactions (block support is not yet available)

## TODO:
- Separate the Snap into its own package to make it lighter
