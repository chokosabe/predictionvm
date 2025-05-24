// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package consts

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/version"
)

const (
	Name   = "predictionvm" // Changed from "morpheusvm"
	Symbol = "PRED"       // Changed from "RED"

	// MaxMarketDataSize defines the maximum expected size for marshaled market data.
	MaxMarketDataSize = 1024

	// Action Type IDs (used in Tx parsing)
	// Note: TransferID is often 0 in HyperSDK examples.
	// If these actions are specific to this VM and don't overlap with a common Transfer action type,
	// starting iota from 0 for these specific actions is fine.
	// Or, if TransferID is a common action type ID 0, these should start from 1.
	// For now, assuming they are distinct and can start their own iota sequence if needed, or be explicit.
	// Let's be explicit to avoid iota confusion for now.
	CreateMarketID uint8 = 0
	BuyYesID       uint8 = 1
	BuyNoID        uint8 = 2
	ResolveID      uint8 = 3 
	ClaimID        uint8 = 4
	// TODO: Add other action IDs etc.
)

// ActionOutputPrefix are prefixed to action outputs so they can be parsed.
const (
	CreateMarketOutput byte = 0x0
	BuyYesOutput       byte = 0x1
	BuyNoOutput        byte = 0x2
	ResolveOutput      byte = 0x3
	ClaimOutput        byte = 0x4
	// TODO: Add other output prefixes etc.
)

const (
	// CodecVersionDefault is the default version for marshalling/unmarshalling.
	CodecVersionDefault uint16 = 0

	// Storage Prefixes
	BalancePrefix byte = 0x3 // Assuming 0x0-height, 0x1-timestamp, 0x2-fee

	// Storage Chunk Sizes/Info
	BalanceChunks uint16 = 1
	Uint16Len     int    = 2

	// Limits
	MaxActionSize = 1024 // 1KB limit for action byte size
)

// Share Types
const (
	YesShareType uint8 = 1 // Aligns with storage.Outcome_Yes
	NoShareType  uint8 = 2 // Aligns with storage.Outcome_No
)

// ShareTypeToString converts a share type to its string representation.
func ShareTypeToString(shareType uint8) string {
	switch shareType {
	case YesShareType:
		return "YES"
	case NoShareType:
		return "NO"
	default:
		return "UnknownShareType"
	}
}

var ID ids.ID

func init() {
	b := make([]byte, ids.IDLen)
	copy(b, []byte(Name)) // Will now use "predictionvm"
	vmID, err := ids.ToID(b)
	if err != nil {
		panic(err)
	}
	ID = vmID
}

var Version = &version.Semantic{
	Major: 0,
	Minor: 0,
	Patch: 1,
}
