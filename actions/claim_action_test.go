package actions

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/stretchr/testify/require"

	"github.com/chokosabe/predictionvm/asset"
	"github.com/chokosabe/predictionvm/storage"
)

func TestClaim_Execute_Success_YesWins(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	// Setup State
	mu := chaintest.NewInMemoryStore()

	// Setup Rules
	mr := &MockRules{
		GetTimeFunc: func() int64 {
			return 100 // Arbitrary time, not critical for claim if market is resolved
		},
	}

	// Test Data
	actorAddr := codec.Address{0x01}
	marketID := ids.GenerateTestID()
	collateralAssetID := ids.GenerateTestID()
	yesAssetID := ids.GenerateTestID()
	noAssetID := ids.GenerateTestID()

	initialYesSharesBalance := uint64(50)
	initialCollateralBalance := uint64(10)
	const payoutAmountPerShare uint64 = 1 // Matches const in claim_action.go

	// 1. Create and store the market - RESOLVED YES
	market := &storage.Market{
		ID:                marketID,
		Question:          "Will YES win?",
		CollateralAssetID: collateralAssetID,
		ClosingTime:       50, // In the past
		OracleAddr:        codec.EmptyAddress,
		Status:            storage.MarketStatus_Resolved,
		ResolvedOutcome:   storage.Outcome_Yes,
		YesAssetID:        yesAssetID,
		NoAssetID:         noAssetID,
		Creator:           codec.Address{0x02},
		ResolutionTime:    75, // In the past
	}
	err := storage.SetMarket(ctx, mu, market)
	require.NoError(err)

	// 2. Initialize actor's YES shares balance for this market
	// Assuming storage.Outcome_Yes (used in market.ResolvedOutcome) corresponds to consts.YesShareType
	err = storage.SetShareBalance(ctx, mu, marketID, actorAddr, uint8(storage.Outcome_Yes), initialYesSharesBalance)
	require.NoError(err)

	// 3. Initialize actor's collateral balance (optional, could be 0)
	err = asset.SetAssetBalance(ctx, mu, actorAddr, collateralAssetID, initialCollateralBalance)
	require.NoError(err)

	// 4. Create Claim Action
	claimAction := &Claim{
		MarketID: marketID,
	}

	// 5. Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID // Not strictly needed for this action's logic but part of interface
	output, err := claimAction.Execute(ctx, mr, mu, txTimestamp, actorAddr, txID)

	// 6. Assertions for successful execution
	require.NoError(err)
	require.NotNil(output) // Expect non-nil output (ClaimResult bytes) on success

	// Check actor's YES share balance (should be 0 or entry removed)
	finalYesSharesBalance, getYesErr := storage.GetShareBalance(ctx, mu, marketID, actorAddr, uint8(storage.Outcome_Yes))
	// If SetShareBalance to 0 removes the key, ErrNotFound is possible. Otherwise, it should be 0.
	if getYesErr != nil {
		require.ErrorIs(getYesErr, database.ErrNotFound, "Expected ErrNotFound if balance entry removed for YES shares, or no error if balance is just zeroed")
		// If it's ErrNotFound, the effective balance is 0 for our check.
		finalYesSharesBalance = 0 
	}
	require.Equal(uint64(0), finalYesSharesBalance, "Actor's YES share balance should be 0 after claim")

	// Check actor's collateral balance (should be increased)
	expectedCollateralPayout := initialYesSharesBalance * payoutAmountPerShare
	expectedFinalCollateralBalance := initialCollateralBalance + expectedCollateralPayout
	finalCollateralBalance, getCollateralErr := asset.GetAssetBalance(ctx, mu, actorAddr, collateralAssetID)
	require.NoError(getCollateralErr, "Fetching collateral balance should not error")
	require.Equal(expectedFinalCollateralBalance, finalCollateralBalance, "Actor's collateral balance should be correctly increased")
}
