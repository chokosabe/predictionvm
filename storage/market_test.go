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
	st := chaintest.NewInMemoryStore()

	// Generate a valid HyperSDK address for testing
	testOracleAddr := codec.CreateAddress(0, ids.GenerateTestID())
	testOracleAddrStr := testOracleAddr.String()

	// Define a sample market according to Spec 3.1
	originalMarket := &Market{
		ID:                1,
		Question:          "Will it rain tomorrow?", // Assuming 'Description' becomes 'Question'
		CollateralAssetID: ids.GenerateTestID(),
		ClosingTime:       1700000000,
		OracleAddr:        oracleAddrFromString(t, testOracleAddrStr), // Use generated valid address string
		Status:            MarketStatus_Open,
		YesAssetID:        ids.GenerateTestID(),
		NoAssetID:         ids.GenerateTestID(),
		// Creator, ResolutionTime, TotalYesShares, TotalNoShares, ResolvedOutcome would also be here
		// For this initial test, focusing on the fields needing alignment.
	}

	// Attempt to set the market (this will likely fail until Market struct is updated)
	err := SetMarket(ctx, st, originalMarket)
	require.NoError(err, "SetMarket should not error initially (pending struct update)")

	// Attempt to get the market
	retrievedMarket, err := GetMarket(ctx, st, originalMarket.ID)
	require.NoError(err)
	require.NotNil(retrievedMarket)

	// Assertions - these will guide the changes to the Market struct
	require.Equal(originalMarket.ID, retrievedMarket.ID)
	// Assuming Description field in current Market struct will be renamed/mapped to Question
	// If Market struct has 'Description', this will fail until it's 'Question' or handled.
	// For now, let's assume we'll add 'Question' and remove/replace 'Description'.
	require.Equal(originalMarket.Question, retrievedMarket.Question)

	// These fields are definitely new and will cause failures until added to Market struct
	require.Equal(originalMarket.CollateralAssetID, retrievedMarket.CollateralAssetID)
	require.Equal(originalMarket.YesAssetID, retrievedMarket.YesAssetID)
	require.Equal(originalMarket.NoAssetID, retrievedMarket.NoAssetID)
	require.Equal(originalMarket.OracleAddr, retrievedMarket.OracleAddr)
	
	// To make the test pass initially with the current Market struct, we might need to comment out
	// setting/getting fields that don't exist yet in originalMarket or in assertions.
	// The goal is to first get this test file in place, then iteratively update Market struct and this test.
}

// oracleAddrFromString is a helper to parse an address string and fail the test on error.
func oracleAddrFromString(t *testing.T, addrStr string) codec.Address {
	addr, err := codec.StringToAddress(addrStr)
	require.NoError(t, err, "Failed to parse address string: %s", addrStr)
	return addr
}
