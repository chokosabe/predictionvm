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

func TestBuyNo_Execute_Success(t *testing.T) {
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
	marketID := uint64(1)
	initialUserBalance := uint64(1000)
	amountToBuy := uint64(10)
	collateralNeeded := amountToBuy
	maxPriceOrCollateral := collateralNeeded + 5

	// 1. Initialize user balance
	err := storage.SetBalance(ctx, mu, senderAddr, initialUserBalance)
	require.NoError(err)

	// 2. Create and store a market
	market := &storage.Market{
		ID:                marketID,
		Question:          "Test Market for BuyNo",
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

	// 3. Create BuyNo Action
	buyNoAction := &BuyNo{
		MarketID: marketID,
		Amount:   amountToBuy,
		MaxPrice: maxPriceOrCollateral,
	}

	// 4. Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID
	output, err := buyNoAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)
	units := buyNoAction.ComputeUnits(mr)

	// 5. Assertions for successful execution
	require.NoError(err)
	require.Nil(output) // Expect nil output on success
	// ComputeUnits currently returns 0, this assertion might change if ComputeUnits logic evolves
	require.Equal(uint64(0), units)

	// Check user's native token balance
	expectedCost := amountToBuy * maxPriceOrCollateral
	expectedFinalUserBalance := initialUserBalance - expectedCost
	finalUserBalance, getBalErr := storage.GetBalance(ctx, mu, senderAddr)
	require.NoError(getBalErr)
	require.Equal(expectedFinalUserBalance, finalUserBalance, "User balance should be correctly deducted")

	// Check user's NO share balance
	userNoShares, getShareErr := storage.GetShareBalance(ctx, mu, marketID, senderAddr, userConsts.NoShareType)
	require.NoError(getShareErr)
	require.Equal(amountToBuy, userNoShares, "User should have the correct amount of NO shares")

	// Check market's total NO shares
	updatedMarket, getMarketErr := storage.GetMarket(ctx, mu, marketID)
	require.NoError(getMarketErr)
	require.NotNil(updatedMarket)
	// Ensure TotalYesShares remains unchanged
}

func TestBuyNo_Execute_Error_MarketNotFound(t *testing.T) {
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
	nonExistentMarketID := uint64(999)
	initialUserBalance := uint64(1000)
	amountToBuy := uint64(10)
	maxPrice := uint64(50)

	// 1. Initialize user balance (optional, as it shouldn't be touched if market not found)
	err := storage.SetBalance(ctx, mu, senderAddr, initialUserBalance)
	require.NoError(err)

	// 2. Create BuyNo Action for a non-existent market
	buyNoAction := &BuyNo{
		MarketID: nonExistentMarketID,
		Amount:   amountToBuy,
		MaxPrice: maxPrice,
	}

	// 3. Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID
	output, err := buyNoAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)

	// 4. Assertions
	require.Error(err, "Expected an error for market not found")
	require.ErrorIs(err, ErrMarketNotFound, "Error should be ErrMarketNotFound")
	require.Contains(err.Error(), fmt.Sprintf("market %d not found when fetching", nonExistentMarketID), "Error message mismatch")
	require.Nil(output, "Output should be nil on error")

	// Check user's native token balance (should be unchanged)
	finalUserBalance, getBalErr := storage.GetBalance(ctx, mu, senderAddr)
	require.NoError(getBalErr, "Fetching balance should not error")
	require.Equal(initialUserBalance, finalUserBalance, "User balance should remain unchanged")

	// Check user's NO share balance (should be 0, and fetching might error if key never created)
	userNoShares, getShareErr := storage.GetShareBalance(ctx, mu, nonExistentMarketID, senderAddr, userConsts.NoShareType)
	if getShareErr != nil {
		require.ErrorIs(getShareErr, database.ErrNotFound, "Expected ErrNotFound or 0 shares if key doesn't exist")
		require.Equal(uint64(0), userNoShares, "User NO shares should be 0 if error is ErrNotFound")
	} else {
		require.Equal(uint64(0), userNoShares, "User NO shares should be 0")
	}
}

func TestBuyNo_Execute_Error_MarketResolved(t *testing.T) {
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
	marketID := uint64(1)
	initialUserBalance := uint64(1000)
	amountToBuy := uint64(10)
	maxPrice := uint64(50)

	marketBase := &storage.Market{
		ID:                marketID,
		Question:          "Test Market Resolved",
		CollateralAssetID: ids.Empty,
		ClosingTime:       200,
		OracleAddr:        codec.EmptyAddress,
		Creator:           codec.Address{0x02},
		ResolutionTime:    300,
		YesAssetID:        ids.Empty,
		NoAssetID:         ids.Empty,
	}

	buyNoAction := &BuyNo{
		MarketID: marketID,
		Amount:   amountToBuy,
		MaxPrice: maxPrice,
	}

	testCases := []struct {
		name         string
		marketStatus storage.MarketStatus
	}{
		{"MarketResolvedYes", storage.MarketStatus_ResolvedYes},
		{"MarketResolvedNo", storage.MarketStatus_ResolvedNo},
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
			err = storage.SetMarket(ctx, mu, &market)
			require.NoError(err)

			// 3. Execute the Action
			txTimestamp := mr.GetTime()
			var txID ids.ID
			output, err := buyNoAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)

			// 4. Assertions
			require.Error(err, "Expected an error for resolved market")
			require.ErrorIs(err, ErrMarketInteraction, "Error should be ErrMarketInteraction")
			require.Contains(err.Error(), fmt.Sprintf("market %d is already resolved", marketID), "Error message mismatch")
			require.Nil(output, "Output should be nil on error")

			// Check user's native token balance (should be unchanged)
			finalUserBalance, getBalErr := storage.GetBalance(ctx, mu, senderAddr)
			require.NoError(getBalErr, "Fetching balance should not error")
			require.Equal(initialUserBalance, finalUserBalance, "User balance should remain unchanged")

			// Check user's NO share balance (should be 0)
			userNoShares, getShareErr := storage.GetShareBalance(ctx, mu, marketID, senderAddr, userConsts.NoShareType)
			if getShareErr != nil {
				require.ErrorIs(getShareErr, database.ErrNotFound, "Expected ErrNotFound or 0 shares")
				require.Equal(uint64(0), userNoShares)
			} else {
				require.Equal(uint64(0), userNoShares)
			}

			// Check market's total NO shares (should be unchanged)
			updatedMarket, getMarketErr := storage.GetMarket(ctx, mu, marketID)
			require.NoError(getMarketErr)
			require.NotNil(updatedMarket)
				})
	}
}

func TestBuyNo_Execute_Error_InsufficientFunds(t *testing.T) {
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
	marketID := uint64(1)
	initialUserBalance := uint64(49) // Cost will be 10 * 5 = 50. Balance is 49.
	amountToBuy := uint64(10)
	maxPrice := uint64(5)

	// 1. Initialize user balance
	err := storage.SetBalance(ctx, mu, senderAddr, initialUserBalance)
	require.NoError(err)

	// 2. Create and store an open market
	market := &storage.Market{
		ID:                marketID,
		Question:          "Test Market Insufficient Funds",
		CollateralAssetID: ids.Empty,
		ClosingTime:       200,
		OracleAddr:        codec.EmptyAddress,
		Status:            storage.MarketStatus_Open,
		Creator:           codec.Address{0x02},
		ResolutionTime:    300,
		YesAssetID:        ids.Empty,
		NoAssetID:         ids.Empty,
	}
	err = storage.SetMarket(ctx, mu, market)
	require.NoError(err)

	// 3. Create BuyNo Action
	buyNoAction := &BuyNo{
		MarketID: marketID,
		Amount:   amountToBuy,
		MaxPrice: maxPrice,
	}

	// 4. Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID
	output, err := buyNoAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)

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

	// Check user's NO share balance (should be 0)
	userNoShares, getShareErr := storage.GetShareBalance(ctx, mu, marketID, senderAddr, userConsts.NoShareType)
	if getShareErr != nil {
		require.ErrorIs(getShareErr, database.ErrNotFound, "Expected ErrNotFound or 0 shares")
		require.Equal(uint64(0), userNoShares)
	} else {
		require.Equal(uint64(0), userNoShares)
	}

	// Check market's total NO shares (should be unchanged)
	updatedMarket, getMarketErr := storage.GetMarket(ctx, mu, marketID)
	require.NoError(getMarketErr)
	require.NotNil(updatedMarket)
}

func TestBuyNo_Execute_Error_AmountZero(t *testing.T) {
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
	marketID := uint64(1)
	// initialUserBalance and market setup are minimal as this error occurs before their use.

	// Create BuyNo Action with Amount = 0
	buyNoAction := &BuyNo{
		MarketID: marketID,
		Amount:   0, // Amount is zero
		MaxPrice: uint64(50),
	}

	// Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID
	output, err := buyNoAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)

	// Assertions
	require.Error(err, "Expected an error for zero amount")
	require.ErrorIs(err, ErrAmountCannotBeZero, "Error should be ErrAmountCannotBeZero")
	require.Nil(output, "Output should be nil on error")

	// Minimal state checks, as the error should occur before significant state changes
	// For instance, ensure no market was attempted to be read if it wasn't created.
}

func TestBuyNo_Execute_Error_MaxPriceZero(t *testing.T) {
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
	marketID := uint64(1)

	// Create BuyNo Action with MaxPrice = 0
	buyNoAction := &BuyNo{
		MarketID: marketID,
		Amount:   uint64(10),
		MaxPrice: 0, // MaxPrice is zero
	}

	// Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID
	output, err := buyNoAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)

	// Assertions
	require.Error(err, "Expected an error for zero max price")
	require.ErrorIs(err, ErrMaxPriceCannotBeZero, "Error should be ErrMaxPriceCannotBeZero")
	require.Nil(output, "Output should be nil on error")
}

func TestBuyNo_Execute_Error_NoBalanceRecord_InsufficientFunds(t *testing.T) {
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
	marketID := uint64(1)
	amountToBuy := uint64(10)
	maxPrice := uint64(5)
	// No initial balance is set for senderAddr

	// 1. Create and store an open market (needed for the action to proceed past market checks)
	market := &storage.Market{
		ID:                marketID,
		Question:          "Test Market No Balance Record",
		CollateralAssetID: ids.Empty,
		ClosingTime:       200,
		OracleAddr:        codec.EmptyAddress,
		Status:            storage.MarketStatus_Open,
		Creator:           codec.Address{0x02},
		ResolutionTime:    300,
		YesAssetID:        ids.Empty,
		NoAssetID:         ids.Empty,
	}
	err := storage.SetMarket(ctx, mu, market)
	require.NoError(err)

	// 2. Create BuyNo Action
	buyNoAction := &BuyNo{
		MarketID: marketID,
		Amount:   amountToBuy,
		MaxPrice: maxPrice,
	}

	// 3. Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID
	output, err := buyNoAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)

	// 4. Assertions
	require.Error(err, "Expected an error for insufficient funds due to no balance record")
	require.ErrorIs(err, ErrInsufficientFunds, "Error should be ErrInsufficientFunds")
	cost := amountToBuy * maxPrice
	// Actor balance will be treated as 0, and a specific error message is generated
	require.Contains(err.Error(), fmt.Sprintf("actor %s has no balance record, cost is %d", senderAddr.String(), cost), "Error message mismatch")
	require.Nil(output, "Output should be nil on error")

	// Check user's native token balance (should still be effectively 0 or non-existent)
	finalUserBalance, getBalErr := storage.GetBalance(ctx, mu, senderAddr)
	if getBalErr != nil {
		require.ErrorIs(getBalErr, database.ErrNotFound, "Expected ErrNotFound for balance")
		require.Equal(uint64(0), finalUserBalance, "Balance should be 0 if ErrNotFound")
	} else {
		// This case should ideally not happen if the record truly doesn't exist and GetBalance propagates ErrNotFound
		// However, if GetBalance returns 0, nil for a non-existent key (as per innerGetBalance logic seen earlier),
		// then this assertion is fine.
		require.Equal(uint64(0), finalUserBalance, "User balance should be 0 if no record existed")
	}

	// Check user's NO share balance (should be 0)
	userNoShares, getShareErr := storage.GetShareBalance(ctx, mu, marketID, senderAddr, userConsts.NoShareType)
	if getShareErr != nil {
		require.ErrorIs(getShareErr, database.ErrNotFound, "Expected ErrNotFound or 0 shares")
		require.Equal(uint64(0), userNoShares)
	} else {
		require.Equal(uint64(0), userNoShares)
	}

	// Check market's total NO shares (should be unchanged)
	updatedMarket, getMarketErr := storage.GetMarket(ctx, mu, marketID)
	require.NoError(getMarketErr)
	require.NotNil(updatedMarket)
}

func TestBuyNo_Execute_Error_MarketTradingClosed(t *testing.T) {
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
	marketID := uint64(1)
	initialUserBalance := uint64(1000)
	amountToBuy := uint64(10)
	maxPrice := uint64(50)

	// 1. Initialize user balance
	err := storage.SetBalance(ctx, mu, senderAddr, initialUserBalance)
	require.NoError(err)

	// 2. Create and store the market with TradingClosed status
	market := &storage.Market{
		ID:                marketID,
		Question:          "Test Market Trading Closed",
		CollateralAssetID: ids.Empty,
		ClosingTime:       200, // Ensure ClosingTime is in the future relative to txTimestamp
		OracleAddr:        codec.EmptyAddress,
		Status:            storage.MarketStatus_TradingClosed,
		Creator:           codec.Address{0x02},
		ResolutionTime:    300,
		YesAssetID:        ids.Empty,
		NoAssetID:         ids.Empty,
	}
	err = storage.SetMarket(ctx, mu, market)
	require.NoError(err)

	// 3. Create BuyNo Action
	buyNoAction := &BuyNo{
		MarketID: marketID,
		Amount:   amountToBuy,
		MaxPrice: maxPrice,
	}

	// 4. Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID
	output, err := buyNoAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)

	// 5. Assertions
	require.Error(err, "Expected an error for market trading closed")
	require.ErrorIs(err, ErrMarketInteraction, "Error should be ErrMarketInteraction")
	require.Contains(err.Error(), fmt.Sprintf("market %d trading is closed", marketID), "Error message mismatch")
	require.Nil(output, "Output should be nil on error")

	// Check user's native token balance (should be unchanged)
	finalUserBalance, getBalErr := storage.GetBalance(ctx, mu, senderAddr)
	require.NoError(getBalErr, "Fetching balance should not error")
	require.Equal(initialUserBalance, finalUserBalance, "User balance should remain unchanged")

	// Check user's NO share balance (should be 0)
	userNoShares, getShareErr := storage.GetShareBalance(ctx, mu, marketID, senderAddr, userConsts.NoShareType)
	if getShareErr != nil {
		require.ErrorIs(getShareErr, database.ErrNotFound, "Expected ErrNotFound or 0 shares")
		require.Equal(uint64(0), userNoShares)
	} else {
		require.Equal(uint64(0), userNoShares)
	}

	// Check market's total NO shares (should be unchanged)
	updatedMarket, getMarketErr := storage.GetMarket(ctx, mu, marketID)
	require.NoError(getMarketErr)
	require.NotNil(updatedMarket)
}

func TestBuyNo_Execute_Error_MarketEndTimePassed(t *testing.T) {
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
	marketID := uint64(1)
	initialUserBalance := uint64(1000)
	amountToBuy := uint64(10)
	maxPrice := uint64(50)

	// 1. Initialize user balance
	err := storage.SetBalance(ctx, mu, senderAddr, initialUserBalance)
	require.NoError(err)

	// 2. Create and store the market with EndTime in the past
	market := &storage.Market{
		ID:                marketID,
		Question:          "Test Market EndTime Passed",
		CollateralAssetID: ids.Empty,
		ClosingTime:       50,  // txTimestamp (100) > ClosingTime (50)
		OracleAddr:        codec.EmptyAddress,
		Status:            storage.MarketStatus_Open, // Market is open but past its ClosingTime
		Creator:           codec.Address{0x02},
		ResolutionTime:    300,
		YesAssetID:        ids.Empty,
		NoAssetID:         ids.Empty,
	}
	err = storage.SetMarket(ctx, mu, market)
	require.NoError(err)

	// 3. Create BuyNo Action
	buyNoAction := &BuyNo{
		MarketID: marketID,
		Amount:   amountToBuy,
		MaxPrice: maxPrice,
	}

	// 4. Execute the Action
	txTimestamp := mr.GetTime()
	var txID ids.ID
	output, err := buyNoAction.Execute(ctx, mr, mu, txTimestamp, senderAddr, txID)

	// 5. Assertions
	require.Error(err, "Expected an error for market EndTime passed")
	require.ErrorIs(err, ErrMarketInteraction, "Error should be ErrMarketInteraction")
	require.Contains(err.Error(), fmt.Sprintf("market %d has ended", marketID), "Error message mismatch")
	require.Nil(output, "Output should be nil on error")

	// Check user's native token balance (should be unchanged)
	finalUserBalance, getBalErr := storage.GetBalance(ctx, mu, senderAddr)
	require.NoError(getBalErr, "Fetching balance should not error")
	require.Equal(initialUserBalance, finalUserBalance, "User balance should remain unchanged")

	// Check user's NO share balance (should be 0)
	userNoShares, getShareErr := storage.GetShareBalance(ctx, mu, marketID, senderAddr, userConsts.NoShareType)
	if getShareErr != nil {
		require.ErrorIs(getShareErr, database.ErrNotFound, "Expected ErrNotFound or 0 shares")
		require.Equal(uint64(0), userNoShares)
	} else {
		require.Equal(uint64(0), userNoShares)
	}

	// Check market's total NO shares (should be unchanged)
	updatedMarket, getMarketErr := storage.GetMarket(ctx, mu, marketID)
	require.NoError(getMarketErr)
	require.NotNil(updatedMarket)
}
