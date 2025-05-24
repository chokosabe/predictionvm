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

	// Action Type IDs
	TransferID uint8 = 0
	CreateMarketID uint8 = iota
	BuyYesID       // Automatically 1
	BuyNoID        // Automatically 2
	// TODO: Add ResolveMarketID, BuyNoID etc.
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
	YesShareType uint8 = 0
	NoShareType  uint8 = 1
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
