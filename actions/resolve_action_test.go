package actions

import (
	"context"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	// "github.com/ava-labs/hypersdk/state" // Not directly used, chaintest.NewInMemoryStore provides state.Mutable
	"github.com/stretchr/testify/require"

	"github.com/chokosabe/predictionvm/consts"
	"github.com/chokosabe/predictionvm/storage"
)

func TestResolve_Execute_Success(t *testing.T) {
	require := require.New(t)

	ctx := context.Background()
	st := chaintest.NewInMemoryStore()
	var rules chain.Rules // Pass nil as Resolve.Execute doesn't use it, or use MockRules from other tests

	// Define an oracle address (hardcoded for testing)
	oracleAddr := codec.Address{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14}

	// Define a dummy creator address (hardcoded for testing)
	creatorAddr := codec.Address{0xA1, 0xB2, 0xC3, 0xD4, 0xE5, 0xF6, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14}

	// Create a market
	marketID := ids.GenerateTestID()
	market := &storage.Market{
		ID:                marketID,
		Question:          "Will it rain tomorrow?",
		CollateralAssetID: ids.GenerateTestID(),
		ClosingTime:       time.Now().Unix() - 3600, // 1 hour ago, so it's resolvable
		OracleAddr:        oracleAddr,
		Status:            storage.MarketStatus_Open, // Or Locked, depending on flow
		ResolvedOutcome:   storage.Outcome_Pending,
		YesAssetID:        ids.GenerateTestID(),
		NoAssetID:         ids.GenerateTestID(),
		Creator:           creatorAddr, 
		ResolutionTime:    0,
	}
	// err was removed when key generation was removed, storage.SetMarket can return an error.
	err := storage.SetMarket(ctx, st, market)
	require.NoError(err)

	// Define the resolve action
	resolveAction := &Resolve{
		MarketID: marketID,
		Outcome:  storage.Outcome_Yes,
	}

	// Execute the action
	blockTime := time.Now().Unix()
	txID := ids.GenerateTestID()
	outputBytes, err := resolveAction.Execute(ctx, rules, st, blockTime, oracleAddr, txID)
	require.NoError(err)
	require.NotNil(outputBytes)

	// Unmarshal and verify the result
	var result ResolveResult
	p := codec.NewReader(outputBytes, MaxResolveResultSize)
	outputTypeID := p.UnpackByte() // Read the TypeID
	require.Equal(consts.ResolveID, outputTypeID) // Check if it's the correct TypeID

	err = result.UnmarshalCodec(p)
	require.NoError(err)
	require.Equal(marketID, result.MarketID)
	require.Equal(storage.Outcome_Yes, result.Resolution)

	// Verify market state after resolution
	updatedMarket, err := storage.GetMarket(ctx, st, marketID)
	require.NoError(err)
	require.Equal(storage.MarketStatus_Resolved, updatedMarket.Status)
	require.Equal(storage.Outcome_Yes, updatedMarket.ResolvedOutcome)
	require.Equal(blockTime, updatedMarket.ResolutionTime)
}
