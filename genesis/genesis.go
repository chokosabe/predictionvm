package genesis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/btcsuite/btcd/btcutil/bech32"
)

var _ chain.Genesis = (*Genesis)(nil)

// CustomGenesisState defines the structure for custom genesis data specific to PredictionVM.
type CustomGenesisState struct {
	Markets []struct {
		ID              uint64 `json:"id"`
		Question        string `json:"question"`
		ClosingTime     int64  `json:"closingTime"` // Unix timestamp
		CollateralAssetID string `json:"collateralAssetId"`
	} `json:"markets"`
}

// Genesis is the genesis data for the PredictionVM.
type Genesis struct {
	Magic     uint64 `json:"magic"`
	Timestamp int64  `json:"timestamp"` // Unix timestamp for the genesis block

	Custom CustomGenesisState `json:"custom"`

	Allocations []struct {
		Address string `json:"address"` // Bech32 address
		Balance uint64 `json:"balance"`
	} `json:"allocations"`
}

func (g *Genesis) Load(raw []byte) error {
	return json.Unmarshal(raw, g)
}

func (g *Genesis) GetMagic() uint64 {
	return g.Magic
}

func (g *Genesis) GetTimestamp() int64 {
	return g.Timestamp
}

func (g *Genesis) InitializeState(ctx context.Context, tracer trace.Tracer, mu state.Mutable, bh chain.BalanceHandler) error {
	for _, alloc := range g.Allocations {
		hrp, data5bit, err := bech32.Decode(alloc.Address)
		if err != nil {
			return fmt.Errorf("failed to decode bech32 address %s: %w", alloc.Address, err)
		}
		_ = hrp // predictionvm allows any HRP for now, or check specific like "morpheus", "prediction"

		data8bit, err := bech32.ConvertBits(data5bit, 5, 8, false)
		if err != nil {
			return fmt.Errorf("failed to convert bech32 data bits for address %s: %w", alloc.Address, err)
		}

		var addr codec.Address
		if len(data8bit) > codec.AddressLen {
			return fmt.Errorf("decoded address %s is too long: got %d bytes, expected max %d", alloc.Address, len(data8bit), codec.AddressLen)
		}
		copy(addr[:], data8bit)
		if err := bh.AddBalance(ctx, addr, mu, alloc.Balance); err != nil { // Using AddBalance from BalanceHandler, removed createAccount argument
			return err
		}
	}
	// TODO: Initialize markets from g.Custom.Markets
	return nil
}

func GetDefault() *Genesis {
	return &Genesis{
		Magic:     12345,
		Timestamp: time.Now().Unix(),
		Custom: CustomGenesisState{
			Markets: []struct {
				ID              uint64 `json:"id"`
				Question        string `json:"question"`
				ClosingTime     int64  `json:"closingTime"`
				CollateralAssetID string `json:"collateralAssetId"`
			}{
				{ID: 1, Question: "Will X happen by Y date?", ClosingTime: time.Now().Add(24 * time.Hour).Unix(), CollateralAssetID: "AVAX"},
			},
		},
		Allocations: []struct {
			Address string `json:"address"`
			Balance uint64 `json:"balance"`
		}{
			// Add default allocations if needed
		},
	}
}
