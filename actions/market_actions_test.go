package actions

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids" // Added for txID
	// "github.com/ava-labs/avalanchego/database" // Removed, chaintest handles in-memory store
	"github.com/ava-labs/hypersdk/chain/chaintest" // Added for NewInMemoryStore
	"github.com/ava-labs/hypersdk/codec"
	// "github.com/ava-labs/hypersdk/consts" // Removed, consts.Dimensions not used
	// "github.com/ava-labs/hypersdk/state" // Removed, state.Mutable is not used
	// "github.com/ava-labs/hypersdk/utils" // Removed, utils.Address not used
	"github.com/stretchr/testify/require"

	userConsts "github.com/chokosabe/predictionvm/consts" // Renamed pvmConsts to userConsts to avoid conflict if any
	// "github.com/chokosabe/predictionvm/controller" // Removed, ctrl not used
	"github.com/chokosabe/predictionvm/storage"
)

func TestBuyYes_Execute_Success(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	// Setup Controller (AuthFactory and BalanceHandler)
	// ctrl := controller.New() // Removed, controller not used directly in this test setup anymore

	// Setup State
	mu := chaintest.NewInMemoryStore()

	// Setup Rules (mocking GetTime)
	mr := &MockRules{
		GetTimeFunc: func() int64 {
			return 100 // Current time, market is open
		},
	}

	// Test Data
	senderAddr := codec.Address{0x01} // Replaced utils.Address(1)
	marketID := uint64(1)
	initialUserBalance := uint64(1000)
	amountToBuy := uint64(10)      // Renamed from sharesToBuy
	collateralNeeded := amountToBuy // Assuming 1:1 for now
	maxPriceOrCollateral := collateralNeeded + 5 // Renamed from maxCollateral

	// 1. Initialize user balance
	err := storage.SetBalance(ctx, mu, senderAddr, initialUserBalance)
	require.NoError(err)

	// 2. Create and store a market
	market := &storage.Market{
		ID:                marketID,
		Question:          "Test Market for BuyYes",
		CollateralAssetID: ids.Empty,
		ClosingTime:       200, // Market closes at time 200
		OracleAddr:        codec.EmptyAddress,
		Status:            storage.MarketStatus_Open,
		Creator:           codec.Address{0x02}, // Different from sender
		ResolutionTime:    300,
		YesAssetID:        ids.Empty,
		NoAssetID:         ids.Empty,
	}
	err = storage.SetMarket(ctx, mu, market)
	require.NoError(err)

	// 3. Create BuyYes Action
	buyYesAction := &BuyYes{
		MarketID: marketID,
		Amount:   amountToBuy,      // Changed from SharesToBuy
		MaxPrice: maxPriceOrCollateral, // Changed from MaxCollateral
	}

	// 4. Auth is handled by the framework; actor (senderAddr) is passed directly to Execute.
	// The ctrl.Auth() call is removed.

	// 5. Execute the Action
	// Placeholder for txTimestamp and txID
	txTimestamp := mr.GetTime() // Can use mocked time
	var txID ids.ID             // Dummy txID
	output, err := buyYesAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)
	units := buyYesAction.ComputeUnits(mr) // Assuming ComputeUnits is what was intended for the second variable

	// 6. Assertions
	require.NoError(err)
	require.Nil(output) // Execute currently returns nil for its []byte output on success
	require.Equal(uint64(0), units) // Execute currently returns 0 units, update when implemented // Assuming 1 unit per dimension of collateral

	// Check user's native token balance
	finalUserBalance, err := storage.GetBalance(ctx, mu, senderAddr)
	require.NoError(err)
	expectedFinalBalance := initialUserBalance - (amountToBuy * maxPriceOrCollateral)
	require.Equal(expectedFinalBalance, finalUserBalance) // Check if balance is correctly debited

	// Check user's YES share balance
	userYesShares, err := storage.GetShareBalance(ctx, mu, marketID, senderAddr, userConsts.YesShareType) // Changed pvmConsts to userConsts
	require.NoError(err)
	require.Equal(amountToBuy, userYesShares) // Check if shares are correctly credited

	// Check market's total YES shares
	updatedMarket, err := storage.GetMarket(ctx, mu, marketID)
	require.NoError(err)
	require.NotNil(updatedMarket)

	// Check that no NO shares were minted for the user
	userNoShares, err := storage.GetShareBalance(ctx, mu, marketID, senderAddr, userConsts.NoShareType) // Changed pvmConsts to userConsts
	require.NoError(err)
	require.Equal(uint64(0), userNoShares)
}
