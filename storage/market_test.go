package storage

import (
	"context" // Added for context.Background()
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/stretchr/testify/require"
)

func TestSetGetMarket_WithSpecFields(t *testing.T) {
	require := require.New(t)

	ctx := context.Background()

	// Generate a valid HyperSDK address for testing
	testOracleAddr := codec.CreateAddress(0, ids.GenerateTestID())
	testCreatorAddr := codec.CreateAddress(1, ids.GenerateTestID())

	baseMarket := &Market{
		ID:                ids.ID{1},
		Question:          "Will it rain tomorrow?",
		CollateralAssetID: ids.GenerateTestID(),
		ClosingTime:       1700000000,
		OracleAddr:        testOracleAddr,
		YesAssetID:        ids.GenerateTestID(),
		NoAssetID:         ids.GenerateTestID(),
		Creator:           testCreatorAddr,
		ResolutionTime:    1700000000 + 3600, // 1 hour after closing
	}

	testCases := []struct {
		name            string
		status          MarketStatus
		outcome         OutcomeType
		expectedOutcome OutcomeType // For assertion, as some statuses might force a pending outcome
	}{
		{"Open_Pending", MarketStatus_Open, Outcome_Pending, Outcome_Pending},
		{"Locked_Pending", MarketStatus_Locked, Outcome_Pending, Outcome_Pending},
		{"Resolved_Yes", MarketStatus_Resolved, Outcome_Yes, Outcome_Yes},
		{"Resolved_No", MarketStatus_Resolved, Outcome_No, Outcome_No},
		{"Resolved_Invalid", MarketStatus_Resolved, Outcome_Invalid, Outcome_Invalid},
		// Edge case: Market is Open but an outcome is set (should ideally be Pending)
		// Depending on SetMarket logic, this might be overridden or stored as is.
		// For now, assume it's stored as is, but GetMarket should reflect it.
		{"Open_Yes_EdgeCase", MarketStatus_Open, Outcome_Yes, Outcome_Yes},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			st := chaintest.NewInMemoryStore() // Fresh store for each test case

			originalMarket := *baseMarket // Copy base
			originalMarket.Status = tc.status
			originalMarket.ResolvedOutcome = tc.outcome
			// If market ID needs to be unique per sub-test, adjust baseMarket.ID or originalMarket.ID here
			// For this test, using the same ID is fine as store is fresh.

			err := SetMarket(ctx, st, &originalMarket)
			require.NoError(err, "SetMarket should not error")

			retrievedMarket, err := GetMarket(ctx, st, originalMarket.ID)
			require.NoError(err)
			require.NotNil(retrievedMarket)

			require.Equal(originalMarket.ID, retrievedMarket.ID)
			require.Equal(originalMarket.Question, retrievedMarket.Question)
			require.Equal(originalMarket.CollateralAssetID, retrievedMarket.CollateralAssetID)
			require.Equal(originalMarket.ClosingTime, retrievedMarket.ClosingTime)
			require.Equal(originalMarket.OracleAddr, retrievedMarket.OracleAddr)
			require.Equal(originalMarket.Status, retrievedMarket.Status)
			require.Equal(tc.expectedOutcome, retrievedMarket.ResolvedOutcome) // Assert against expectedOutcome
			require.Equal(originalMarket.YesAssetID, retrievedMarket.YesAssetID)
			require.Equal(originalMarket.NoAssetID, retrievedMarket.NoAssetID)
			require.Equal(originalMarket.Creator, retrievedMarket.Creator)
			require.Equal(originalMarket.ResolutionTime, retrievedMarket.ResolutionTime)
		})
	}
}


func oracleAddrFromString_unused(t *testing.T, addrStr string) codec.Address {
	addr, err := codec.StringToAddress(addrStr)
	require.NoError(t, err, "Failed to parse address string: %s", addrStr)
	return addr
}
