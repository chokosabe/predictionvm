package actions

import (
	"context"
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/stretchr/testify/require"

	userConsts "github.com/chokosabe/predictionvm/consts"
	"github.com/chokosabe/predictionvm/storage"
)

func TestBuyYes_Execute_Success(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	// Setup State
	mu := chaintest.NewInMemoryStore()

	// Setup Rules (mocking GetTime)
	mr := &MockRules{
		GetTimeFunc: func() int64 {
			return 100 // Current time, market is open
		},
	}

	// Test Data
	senderAddr := codec.Address{0x01}
	marketID_uint64 := uint64(1)
	marketID_ids := ids.ID{byte(marketID_uint64)} // Simple conversion for test
	collateralAssetID_test := ids.ID{0xA1, 0xA2}
	yesAssetID_test := ids.ID{0xB1, 0xB2}
	initialUserBalance := uint64(1000)
	amountToBuy := uint64(10)
	collateralNeeded := amountToBuy
	maxPriceOrCollateral := collateralNeeded + 5

	// 1. Initialize user balance
	err := storage.SetBalance(ctx, mu, senderAddr, initialUserBalance)
	require.NoError(err)

	// 2. Create and store a market
	market := &storage.Market{
		ID:                marketID_ids, // storage.Market uses uint64 for ID
		Question:          "Test Market for BuyYes",
		CollateralAssetID: collateralAssetID_test, // Updated for consistency
		ClosingTime:       200, // Market closes at time 200
		OracleAddr:        codec.EmptyAddress,
		Status:            storage.MarketStatus_Open,
		Creator:           codec.Address{0x02}, // Different from sender
		ResolutionTime:    300,
		YesAssetID:        yesAssetID_test, // Updated for consistency
		NoAssetID:         ids.ID{0xC1, 0xC2}, // Example NoAssetID
	}
	err = storage.SetMarket(ctx, mu, market)
	require.NoError(err)

	// 3. Create BuyYes Action
	buyYesAction := &BuyYes{
		MarketID:          marketID_ids,
		CollateralAssetID: collateralAssetID_test,
		YesAssetID:        yesAssetID_test,
		Amount:            amountToBuy,
		MaxPrice:          maxPriceOrCollateral,
	}

	// 4. Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID
	output, err := buyYesAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)
	units := buyYesAction.ComputeUnits(mr)

	// 5. Assertions for successful execution
	require.NoError(err)
	require.NotNil(output) // Expect non-nil output
	require.Equal(uint64(0), units) // ComputeUnits currently returns 0

	// Manually unmarshal the output into BuyYesResult
	require.NotEmpty(output, "Output should not be empty")
	// The first byte is the type ID, UnmarshalCodec expects the rest.
	resultBytes := output[1:] 
	packer := codec.NewReader(resultBytes, len(resultBytes))
	
	buyYesResult := &BuyYesResult{}
	require.NoError(buyYesResult.UnmarshalCodec(packer), "Failed to unmarshal output into BuyYesResult")

	// Assert the fields of BuyYesResult
	expectedCost := amountToBuy * maxPriceOrCollateral
	require.Equal(amountToBuy, buyYesResult.SharesBought, "BuyYesResult.SharesBought should match amountToBuy")
	require.Equal(expectedCost, buyYesResult.CostPaid, "BuyYesResult.CostPaid should match expectedCost")

	// Check user's native token balance
	// expectedCost is already defined above
	expectedFinalUserBalance := initialUserBalance - expectedCost
	finalUserBalance, getBalErr := storage.GetBalance(ctx, mu, senderAddr)
	require.NoError(getBalErr)
	require.Equal(expectedFinalUserBalance, finalUserBalance, "User balance should be correctly deducted")

	// Check user's YES share balance
	userYesShares, getShareErr := storage.GetShareBalance(ctx, mu, marketID_ids, senderAddr, userConsts.YesShareType)
	require.NoError(getShareErr)
	require.Equal(amountToBuy, userYesShares, "User should have the correct amount of YES shares")

	// Check market's state (TotalYesShares is managed by HybridAsset module, so no direct check here)
	updatedMarket, getMarketErr := storage.GetMarket(ctx, mu, marketID_ids)
	require.NoError(getMarketErr)
	require.NotNil(updatedMarket)
}

func TestBuyYes_Execute_Error_MarketNotFound(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	// Setup State
	mu := chaintest.NewInMemoryStore()

	// Setup Rules (mocking GetTime)
	mr := &MockRules{
		GetTimeFunc: func() int64 {
			return 100 // Current time
		},
	}

	// Test Data
	senderAddr := codec.Address{0x01}
	nonExistentMarketID_uint64 := uint64(999)
	nonExistentMarketID_ids := ids.ID{byte(nonExistentMarketID_uint64)} // Simple conversion for test
	collateralAssetID_test := ids.ID{0xA1, 0xA2} // Consistent test asset ID
	yesAssetID_test := ids.ID{0xB1, 0xB2}      // Consistent test asset ID
	initialUserBalance := uint64(1000)
	amountToBuy := uint64(10)
	maxPrice := uint64(50)

	// 1. Initialize user balance (optional, as it shouldn't be touched if market not found)
	err := storage.SetBalance(ctx, mu, senderAddr, initialUserBalance)
	require.NoError(err)

	// 2. Create BuyYes Action for a non-existent market
	buyYesAction := &BuyYes{
		MarketID:          nonExistentMarketID_ids,
		CollateralAssetID: collateralAssetID_test,
		YesAssetID:        yesAssetID_test,
		Amount:            amountToBuy,
		MaxPrice:          maxPrice,
	}

	// 3. Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID
	output, err := buyYesAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)

	// 4. Assertions
	require.Error(err, "Expected an error for market not found")
	require.ErrorIs(err, ErrMarketNotFound, "Error should be ErrMarketNotFound")
	require.Contains(err.Error(), fmt.Sprintf("market %s not found when fetching", nonExistentMarketID_ids.String()), "Error message mismatch")
	require.Nil(output, "Output should be nil on error")

	// Check user's native token balance (should be unchanged)
	finalUserBalance, getBalErr := storage.GetBalance(ctx, mu, senderAddr)
	require.NoError(getBalErr, "Fetching balance should not error")
	require.Equal(initialUserBalance, finalUserBalance, "User balance should remain unchanged")

	// Check user's YES share balance (should be 0, and fetching might error if key never created)
	userYesShares, getShareErr := storage.GetShareBalance(ctx, mu, nonExistentMarketID_ids, senderAddr, userConsts.YesShareType)
	if getShareErr != nil {
		require.ErrorIs(getShareErr, database.ErrNotFound, "Expected ErrNotFound or 0 shares if key doesn't exist")
		require.Equal(uint64(0), userYesShares, "User YES shares should be 0 if error is ErrNotFound")
	} else {
		require.Equal(uint64(0), userYesShares, "User YES shares should be 0")
	}
}

func TestBuyYes_Execute_Error_MarketResolved(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	// Setup Rules (mocking GetTime)
	mr := &MockRules{
		GetTimeFunc: func() int64 {
			return 100 // Current time, before market EndTime
		},
	}

	// Test Data
	senderAddr := codec.Address{0x01}
	marketID_uint64 := uint64(1)
	marketID_ids := ids.ID{byte(marketID_uint64)} // Simple conversion for test
	collateralAssetID_test := ids.ID{0xA1, 0xA2} // Consistent test asset ID
	yesAssetID_test := ids.ID{0xB1, 0xB2}      // Consistent test asset ID
	initialUserBalance := uint64(1000)
	amountToBuy := uint64(10)
	maxPrice := uint64(50)

	marketBase := &storage.Market{
		ID:                marketID_ids, // storage.Market uses uint64 for ID
		Question:          "Test Market Resolved for BuyYes",
		CollateralAssetID: collateralAssetID_test, // Updated
		ClosingTime:       200,
		OracleAddr:        codec.EmptyAddress,
		Creator:           codec.Address{0x02},
		ResolutionTime:    300,
		YesAssetID:        yesAssetID_test, // Updated
		NoAssetID:         ids.ID{0xC1, 0xC2}, // Example NoAssetID
	}

	buyYesAction := &BuyYes{
		MarketID:          marketID_ids,
		CollateralAssetID: collateralAssetID_test,
		YesAssetID:        yesAssetID_test,
		Amount:            amountToBuy,
		MaxPrice:          maxPrice,
	}

	testCases := []struct {
		name            string
		marketStatus    storage.MarketStatus
		resolvedOutcome storage.OutcomeType
	}{
		{"MarketResolvedYes", storage.MarketStatus_Resolved, storage.Outcome_Yes},
		{"MarketResolvedNo", storage.MarketStatus_Resolved, storage.Outcome_No},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mu := chaintest.NewInMemoryStore() // Fresh store for each sub-test

			// 1. Initialize user balance
			err := storage.SetBalance(ctx, mu, senderAddr, initialUserBalance)
			require.NoError(err)

			// 2. Create and store the market with the specific resolved status
			market := *marketBase // Copy base market
			market.Status = tc.marketStatus
			market.ResolvedOutcome = tc.resolvedOutcome
			err = storage.SetMarket(ctx, mu, &market)
			require.NoError(err)

			// 3. Execute the Action
			txTimestamp := mr.GetTime()
			var txID ids.ID
			output, err := buyYesAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)

			// 4. Assertions
			require.Error(err, "Expected an error for resolved market")
			require.ErrorIs(err, ErrMarketInteraction, "Error should be ErrMarketInteraction")
			require.Contains(err.Error(), fmt.Sprintf("market %s is already resolved", marketID_ids.String()), "Error message mismatch")
			require.Nil(output, "Output should be nil on error")

			// Check user's native token balance (should be unchanged)
			finalUserBalance, getBalErr := storage.GetBalance(ctx, mu, senderAddr)
			require.NoError(getBalErr, "Fetching balance should not error")
			require.Equal(initialUserBalance, finalUserBalance, "User balance should remain unchanged")

			// Check user's YES share balance (should be 0)
			userYesShares, getShareErr := storage.GetShareBalance(ctx, mu, marketID_ids, senderAddr, userConsts.YesShareType)
			if getShareErr != nil {
				require.ErrorIs(getShareErr, database.ErrNotFound, "Expected ErrNotFound or 0 shares")
				require.Equal(uint64(0), userYesShares)
			} else {
				require.Equal(uint64(0), userYesShares)
			}

			// Check market's state (TotalYesShares is managed by HybridAsset module)
			updatedMarket, getMarketErr := storage.GetMarket(ctx, mu, marketID_ids)
			require.NoError(getMarketErr)
			require.NotNil(updatedMarket)
		})
	}
}

func TestBuyYes_Execute_Error_InsufficientFunds(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	// Setup Rules (mocking GetTime)
	mr := &MockRules{
		GetTimeFunc: func() int64 {
			return 100 // Current time, market is open
		},
	}

	// Test Data
	senderAddr := codec.Address{0x01}
	marketID_uint64 := uint64(1)
	marketID_ids := ids.ID{byte(marketID_uint64)} // Simple conversion for test
	collateralAssetID_test := ids.ID{0xA1, 0xA2} // Consistent test asset ID
	yesAssetID_test := ids.ID{0xB1, 0xB2}      // Consistent test asset ID
	initialUserBalance := uint64(49) // Cost will be 10 * 5 = 50. Balance is 49.
	amountToBuy := uint64(10)
	maxPrice := uint64(5)

	// 1. Initialize user balance
	err := storage.SetBalance(ctx, mu, senderAddr, initialUserBalance)
	require.NoError(err)

	// 2. Create and store an open market
	market := &storage.Market{
		ID:                marketID_ids, // storage.Market uses uint64 for ID
		Question:          "Test Market Insufficient Funds for BuyYes",
		CollateralAssetID: collateralAssetID_test, // Updated
		ClosingTime:       200,
		OracleAddr:        codec.EmptyAddress,
		Status:            storage.MarketStatus_Open,
		Creator:           codec.Address{0x02},
		ResolutionTime:    300,
		YesAssetID:        yesAssetID_test, // Updated
		NoAssetID:         ids.ID{0xC1, 0xC2}, // Example NoAssetID
	}
	err = storage.SetMarket(ctx, mu, market)
	require.NoError(err)

	// 3. Create BuyYes Action
	buyYesAction := &BuyYes{
		MarketID:          marketID_ids,
		CollateralAssetID: collateralAssetID_test,
		YesAssetID:        yesAssetID_test,
		Amount:            amountToBuy,
		MaxPrice:          maxPrice,
	}

	// 4. Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID
	output, err := buyYesAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)

	// 5. Assertions
	require.Error(err, "Expected an error for insufficient funds")
	require.ErrorIs(err, ErrInsufficientFunds, "Error should be ErrInsufficientFunds")
	cost := amountToBuy * maxPrice
	require.Contains(err.Error(), fmt.Sprintf("actor balance %d, cost %d", initialUserBalance, cost), "Error message mismatch")
	require.Nil(output, "Output should be nil on error")

	// Check user's native token balance (should be unchanged)
	finalUserBalance, getBalErr := storage.GetBalance(ctx, mu, senderAddr)
	require.NoError(getBalErr, "Fetching balance should not error")
	require.Equal(initialUserBalance, finalUserBalance, "User balance should remain unchanged")

	// Check user's YES share balance (should be 0)
	userYesShares, getShareErr := storage.GetShareBalance(ctx, mu, marketID_ids, senderAddr, userConsts.YesShareType)
	if getShareErr != nil {
		require.ErrorIs(getShareErr, database.ErrNotFound, "Expected ErrNotFound or 0 shares")
		require.Equal(uint64(0), userYesShares)
	} else {
		require.Equal(uint64(0), userYesShares)
	}

	// Check market's state
	updatedMarket, getMarketErr := storage.GetMarket(ctx, mu, marketID_ids) // Corrected marketID here
	require.NoError(getMarketErr)
	require.NotNil(updatedMarket)
	// Ensure other market properties are as expected if necessary
}

func TestBuyYes_Execute_Error_AmountZero(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore() // Not strictly needed for this validation, but good practice

	// Setup Rules (mocking GetTime)
	mr := &MockRules{
		GetTimeFunc: func() int64 {
			return 100
		},
	}

	// Test Data
	senderAddr := codec.Address{0x01}
	marketID_uint64 := uint64(1)
	marketID_ids := ids.ID{byte(marketID_uint64)} // Simple conversion for test
	collateralAssetID_test := ids.ID{0xA1, 0xA2} // Consistent test asset ID
	yesAssetID_test := ids.ID{0xB1, 0xB2}      // Consistent test asset ID

	// Create BuyYes Action with Amount = 0
	buyYesAction := &BuyYes{
		MarketID:          marketID_ids,
		CollateralAssetID: collateralAssetID_test,
		YesAssetID:        yesAssetID_test,
		Amount:            0, // Amount is zero
		MaxPrice:          uint64(50),
	}

	// Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID
	output, err := buyYesAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)

	// Assertions
	require.Error(err, "Expected an error for zero amount")
	require.ErrorIs(err, ErrAmountCannotBeZero, "Error should be ErrAmountCannotBeZero")
	require.Nil(output, "Output should be nil on error")
}

func TestBuyYes_Execute_Error_MaxPriceZero(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore() // Not strictly needed, but good practice

	// Setup Rules (mocking GetTime)
	mr := &MockRules{
		GetTimeFunc: func() int64 {
			return 100
		},
	}

	// Test Data
	senderAddr := codec.Address{0x01}
	marketID_uint64 := uint64(1)
	marketID_ids := ids.ID{byte(marketID_uint64)} // Simple conversion for test
	collateralAssetID_test := ids.ID{0xA1, 0xA2} // Consistent test asset ID
	yesAssetID_test := ids.ID{0xB1, 0xB2}      // Consistent test asset ID

	// Create BuyYes Action with MaxPrice = 0
	buyYesAction := &BuyYes{
		MarketID:          marketID_ids,
		CollateralAssetID: collateralAssetID_test,
		YesAssetID:        yesAssetID_test,
		Amount:            uint64(10),
		MaxPrice:          0, // MaxPrice is zero
	}

	// Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID
	output, err := buyYesAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)

	// Assertions
	require.Error(err, "Expected an error for zero max price")
	require.ErrorIs(err, ErrMaxPriceCannotBeZero, "Error should be ErrMaxPriceCannotBeZero")
	require.Nil(output, "Output should be nil on error")
}

func TestBuyYes_Execute_Error_NoBalanceRecord_InsufficientFunds(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	// Setup Rules (mocking GetTime)
	mr := &MockRules{
		GetTimeFunc: func() int64 {
			return 100 // Current time, market is open
		},
	}

	// Test Data
	senderAddr := codec.Address{0x01} // Actor with no balance record
	marketID_uint64 := uint64(1) // This is the actual market that exists
	marketID_ids := ids.ID{byte(marketID_uint64)} // Simple conversion for test
	collateralAssetID_test := ids.ID{0xA1, 0xA2}    // Consistent test asset ID, though market uses ids.Empty for now
	yesAssetID_test := ids.ID{0xB1, 0xB2}         // Consistent test asset ID, though market uses ids.Empty for now
	amountToBuy := uint64(10)
	maxPrice := uint64(50)

	// 1. Create and store the market (ensure it exists for other checks, though actor has no balance)
	market := &storage.Market{
		ID:                marketID_ids, // Uses uint64 for storage.Market
		Question:          "Test Market for No Balance Record",
		CollateralAssetID: ids.Empty,       // storage.Market uses ids.Empty here
		ClosingTime:       200,
		OracleAddr:        codec.EmptyAddress,
		Status:            storage.MarketStatus_Open,
		Creator:           codec.Address{0x02},
		ResolutionTime:    300,
		YesAssetID:        ids.Empty,       // storage.Market uses ids.Empty here
		NoAssetID:         ids.Empty,
	}
	err := storage.SetMarket(ctx, mu, market)
	require.NoError(err)

	// 2. Create BuyYes Action
	buyYesAction := &BuyYes{
		MarketID:          marketID_ids, // Action uses the ID of the existing market
		CollateralAssetID: collateralAssetID_test,
		YesAssetID:        yesAssetID_test,
		Amount:            amountToBuy,
		MaxPrice:          maxPrice,
	}

	// 3. Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID
	output, err := buyYesAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)

	// 4. Assertions
	require.Error(err, "Expected an error for insufficient funds due to no balance record")
	require.ErrorIs(err, ErrInsufficientFunds, "Error should be ErrInsufficientFunds")
	cost := amountToBuy * maxPrice
	require.Contains(err.Error(), fmt.Sprintf("actor %s has no balance record, cost is %d", senderAddr.String(), cost), "Error message mismatch")
	require.Nil(output, "Output should be nil on error")

	// Check user's native token balance (should still be effectively 0 or non-existent)
	finalUserBalance, getBalErr := storage.GetBalance(ctx, mu, senderAddr)
	if getBalErr != nil {
		require.ErrorIs(getBalErr, database.ErrNotFound, "Expected ErrNotFound for balance")
		require.Equal(uint64(0), finalUserBalance, "Balance should be 0 if ErrNotFound")
	} else {
		require.Equal(uint64(0), finalUserBalance, "User balance should be 0 if no record existed")
	}

	// Check user's YES share balance (should be 0)
	userYesShares, getShareErr := storage.GetShareBalance(ctx, mu, marketID_ids, senderAddr, userConsts.YesShareType)
	if getShareErr != nil {
		require.ErrorIs(getShareErr, database.ErrNotFound, "Expected ErrNotFound or 0 shares")
		require.Equal(uint64(0), userYesShares)
	} else {
		require.Equal(uint64(0), userYesShares)
	}

	// Check market's state - this should check the actual market that was set up
	updatedMarket, getMarketErr := storage.GetMarket(ctx, mu, marketID_ids) // Corrected: Check the market that exists
	require.NoError(getMarketErr)
	require.NotNil(updatedMarket)
}

func TestBuyYes_Execute_Error_MarketTradingClosed(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	// Setup Rules (mocking GetTime)
	mr := &MockRules{
		GetTimeFunc: func() int64 {
			return 100 // Current time, before market EndTime
		},
	}

	// Test Data
	senderAddr := codec.Address{0x01}
	marketID_uint64 := uint64(1)
	marketID_ids := ids.ID{byte(marketID_uint64)} // Simple conversion for test
	collateralAssetID_test := ids.ID{0xA1, 0xA2}    // Consistent test asset ID
	yesAssetID_test := ids.ID{0xB1, 0xB2}         // Consistent test asset ID
	initialUserBalance := uint64(1000)
	amountToBuy := uint64(10)
	maxPrice := uint64(50)

	// 1. Initialize user balance
	err := storage.SetBalance(ctx, mu, senderAddr, initialUserBalance)
	require.NoError(err)

	// 2. Create and store the market with TradingClosed status
	market := &storage.Market{
		ID:                marketID_ids, // Uses uint64 for storage.Market
		Question:          "Test Market Trading Closed for BuyYes",
		CollateralAssetID: ids.Empty,       // storage.Market uses ids.Empty here
		ClosingTime:       200, // Ensure ClosingTime is in the future relative to txTimestamp
		OracleAddr:        codec.EmptyAddress,
		Status:            storage.MarketStatus_Locked, // Market is locked
		Creator:           codec.Address{0x02},
		ResolutionTime:    300,
		YesAssetID:        ids.Empty,       // storage.Market uses ids.Empty here
		NoAssetID:         ids.Empty,
	}
	err = storage.SetMarket(ctx, mu, market)
	require.NoError(err)

	// 3. Create BuyYes Action
	buyYesAction := &BuyYes{
		MarketID:          marketID_ids,
		CollateralAssetID: collateralAssetID_test,
		YesAssetID:        yesAssetID_test,
		Amount:            amountToBuy,
		MaxPrice:          maxPrice,
	}

	// 4. Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID
	output, err := buyYesAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)

	// 5. Assertions
	require.Error(err, "Expected an error for market trading closed")
	require.ErrorIs(err, ErrMarketInteraction, "Error should be ErrMarketInteraction")
	require.Contains(err.Error(), fmt.Sprintf("market %s trading is closed (status: Locked)", marketID_ids.String()), "Error message mismatch")
	require.Nil(output, "Output should be nil on error")

	// Check user's native token balance (should be unchanged)
	finalUserBalance, getBalErr := storage.GetBalance(ctx, mu, senderAddr)
	require.NoError(getBalErr, "Fetching balance should not error")
	require.Equal(initialUserBalance, finalUserBalance, "User balance should remain unchanged")

	// Check user's YES share balance (should be 0)
	userYesShares, getShareErr := storage.GetShareBalance(ctx, mu, marketID_ids, senderAddr, userConsts.YesShareType)
	if getShareErr != nil {
		require.ErrorIs(getShareErr, database.ErrNotFound, "Expected ErrNotFound or 0 shares")
		require.Equal(uint64(0), userYesShares)
	} else {
		require.Equal(uint64(0), userYesShares)
	}

	// Check market's state
	updatedMarket, getMarketErr := storage.GetMarket(ctx, mu, marketID_ids)
	require.NoError(getMarketErr)
	require.NotNil(updatedMarket)
}

func TestBuyYes_Execute_Error_MarketEndTimePassed(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	// Setup Rules (mocking GetTime)
	// txTimestamp will be 100. Market EndTime will be 50.
	mr := &MockRules{
		GetTimeFunc: func() int64 {
			return 100
		},
	}

	// Test Data
	senderAddr := codec.Address{0x01}
	marketID_uint64 := uint64(1)
	marketID_ids := ids.ID{byte(marketID_uint64)} // Simple conversion for test
	collateralAssetID_test := ids.ID{0xA1, 0xA2}    // Consistent test asset ID
	yesAssetID_test := ids.ID{0xB1, 0xB2}         // Consistent test asset ID
	initialUserBalance := uint64(1000)
	amountToBuy := uint64(10)
	maxPrice := uint64(50)

	// 1. Initialize user balance
	err := storage.SetBalance(ctx, mu, senderAddr, initialUserBalance)
	require.NoError(err)

	// 2. Create and store the market with EndTime in the past
	market := &storage.Market{
		ID:                marketID_ids,
		Question:          "Test Market EndTime Passed for BuyYes",
		CollateralAssetID: ids.Empty,
		ClosingTime:       50, // EndTime is before txTimestamp (100)
		OracleAddr:        codec.EmptyAddress,
		Status:            storage.MarketStatus_Open, // Market is open but past end time for trading
		Creator:           codec.Address{0x02},
		ResolutionTime:    300,
		YesAssetID:        ids.Empty,
		NoAssetID:         ids.Empty,
	}
	err = storage.SetMarket(ctx, mu, market)
	require.NoError(err)

	// 3. Create BuyYes Action
	buyYesAction := &BuyYes{
		MarketID:          marketID_ids,
		CollateralAssetID: collateralAssetID_test,
		YesAssetID:        yesAssetID_test,
		Amount:            amountToBuy,
		MaxPrice:          maxPrice,
	}

	// 4. Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID
	output, err := buyYesAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)

	// 5. Assertions
	require.Error(err, "Expected an error for market end time passed")
	require.ErrorIs(err, ErrMarketEndTimePassed, "Error should be ErrMarketEndTimePassed")
	require.Nil(output, "Output should be nil on error")

	// Check user's native token balance (should be unchanged)
	finalUserBalance, getBalErr := storage.GetBalance(ctx, mu, senderAddr)
	require.NoError(getBalErr, "Fetching balance should not error")
	require.Equal(initialUserBalance, finalUserBalance, "User balance should remain unchanged")

	// Check user's YES share balance (should be 0)
	userYesShares, getShareErr := storage.GetShareBalance(ctx, mu, marketID_ids, senderAddr, userConsts.YesShareType)
	if getShareErr != nil {
		require.ErrorIs(getShareErr, database.ErrNotFound, "Expected ErrNotFound or 0 shares")
		require.Equal(uint64(0), userYesShares)
	} else {
		require.Equal(uint64(0), userYesShares)
	}

	// Check market's state
	updatedMarket, getMarketErr := storage.GetMarket(ctx, mu, marketID_ids)
	require.NoError(getMarketErr)
	require.NotNil(updatedMarket)
}
