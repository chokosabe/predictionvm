package asset

import (
	"context"
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/stretchr/testify/require"
)

func TestMintShares_Success_NewAsset(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	actorAddr := codec.Address{0xA1, 0x01, 0x02, 0x03}
	marketID := uint64(1)
	amountToMint := uint64(100)

	// Call MintShares for YES shares
	mintedYesAssetID, err := MintShares(ctx, mu, marketID, actorAddr, YesShare, amountToMint)
	require.NoError(err)
	require.NotEqual(ids.Empty, mintedYesAssetID, "Minted YES asset ID should not be empty")

	// Check actor's balance of the new YES shares
	yesBalance, err := GetAssetBalance(ctx, mu, actorAddr, mintedYesAssetID)
	require.NoError(err, "Getting YES balance should succeed after minting")
	require.Equal(amountToMint, yesBalance, "Actor should have the minted amount of YES shares")

	// Verify the derived asset ID
	expectedYesAssetID, err := GetShareAssetID(marketID, YesShare)
	require.NoError(err)
	require.Equal(expectedYesAssetID, mintedYesAssetID, "Minted YES asset ID should match derived ID")

	// Call MintShares for NO shares for the same market and actor
	noAmountToMint := uint64(150)
	mintedNoAssetID, err := MintShares(ctx, mu, marketID, actorAddr, NoShare, noAmountToMint)
	require.NoError(err)
	require.NotEqual(ids.Empty, mintedNoAssetID, "Minted NO asset ID should not be empty")
	require.NotEqual(mintedYesAssetID, mintedNoAssetID, "YES and NO asset IDs should be different for the same market")

	// Check actor's balance of the new NO shares
	noBalance, err := GetAssetBalance(ctx, mu, actorAddr, mintedNoAssetID)
	require.NoError(err, "Getting NO balance should succeed after minting")
	require.Equal(noAmountToMint, noBalance, "Actor should have the minted amount of NO shares")

	// Verify the derived asset ID for NO shares
	expectedNoAssetID, err := GetShareAssetID(marketID, NoShare)
	require.NoError(err)
	require.Equal(expectedNoAssetID, mintedNoAssetID, "Minted NO asset ID should match derived ID")
}

func TestMintShares_Success_AddToExistingAsset(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	actorAddr := codec.Address{0xA2, 0x01, 0x02, 0x03}
	marketID := uint64(2)
	initialAmount := uint64(100)
	additionalAmount := uint64(50)
	expectedTotalAmount := initialAmount + additionalAmount

	// First mint
	assetID, err := MintShares(ctx, mu, marketID, actorAddr, YesShare, initialAmount)
	require.NoError(err)
	require.NotEqual(ids.Empty, assetID)

	balance1, err := GetAssetBalance(ctx, mu, actorAddr, assetID)
	require.NoError(err)
	require.Equal(initialAmount, balance1, "Balance after first mint should be initialAmount")

	// Second mint of the same asset to the same actor
	assetID2, err := MintShares(ctx, mu, marketID, actorAddr, YesShare, additionalAmount)
	require.NoError(err)
	require.Equal(assetID, assetID2, "Asset ID should be the same for subsequent mints of the same share type")

	// Check final balance
	finalBalance, err := GetAssetBalance(ctx, mu, actorAddr, assetID)
	require.NoError(err, "Getting final balance should succeed")
	require.Equal(expectedTotalAmount, finalBalance, "Actor's balance should be the sum of both mints")
}

func TestMintShares_Error_ZeroAmount(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	actorAddr := codec.Address{0xA4, 0x01, 0x02, 0x03}
	marketID := uint64(3)

	_, err := MintShares(ctx, mu, marketID, actorAddr, YesShare, 0)
	require.Error(err, "Expected error when minting zero amount")
	require.Contains(err.Error(), "cannot mint zero amount of shares")
}

func TestGetAssetBalance_NotFound(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	actorAddr := codec.Address{0xA3, 0x01, 0x02, 0x03}
	// Use a deterministically generated asset ID that is unlikely to exist by chance
	nonExistentAssetID, err := GetShareAssetID(999, YesShare) 
	require.NoError(err)

	balance, err := GetAssetBalance(ctx, mu, actorAddr, nonExistentAssetID)
	require.ErrorIs(err, database.ErrNotFound, "Expected ErrNotFound for non-existent asset balance")
	require.Equal(uint64(0), balance, "Balance should be 0 if not found")
}

func TestGetShareAssetID_DeterminismAndUniqueness(t *testing.T) {
	require := require.New(t)

	marketID1 := uint64(1)
	marketID2 := uint64(2)

	// YES shares for market 1
	yes1_a, err := GetShareAssetID(marketID1, YesShare)
	require.NoError(err)
	yes1_b, err := GetShareAssetID(marketID1, YesShare)
	require.NoError(err)
	require.Equal(yes1_a, yes1_b, "Asset ID for same market and type should be deterministic")

	// NO shares for market 1
	no1, err := GetShareAssetID(marketID1, NoShare)
	require.NoError(err)
	require.NotEqual(yes1_a, no1, "YES and NO share IDs for the same market should be different")

	// YES shares for market 2
	yes2, err := GetShareAssetID(marketID2, YesShare)
	require.NoError(err)
	require.NotEqual(yes1_a, yes2, "YES share IDs for different markets should be different")

	// NO shares for market 2
	no2, err := GetShareAssetID(marketID2, NoShare)
	require.NoError(err)
	require.NotEqual(no1, no2, "NO share IDs for different markets should be different")
	require.NotEqual(yes2, no2, "YES and NO share IDs for market 2 should be different")

	// Test unknown share type
	_, err = GetShareAssetID(marketID1, ShareType(99))
	require.Error(err, "Expected error for unknown share type")
	require.Contains(err.Error(), "unknown share type")
}

func TestBurnShares_Success(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	actorAddr := codec.Address{0xB1, 0x01, 0x02, 0x03}
	marketID := uint64(10)
	initialMintAmount := uint64(100)
	burnAmount := uint64(30)
	expectedRemainingAmount := initialMintAmount - burnAmount

	// Mint initial shares
	assetID, err := MintShares(ctx, mu, marketID, actorAddr, YesShare, initialMintAmount)
	require.NoError(err)

	// Burn some shares
	err = BurnShares(ctx, mu, marketID, actorAddr, YesShare, burnAmount)
	require.NoError(err, "Burning shares should succeed") // This will fail until BurnShares is implemented

	// Check balance after burning
	balance, err := GetAssetBalance(ctx, mu, actorAddr, assetID)
	require.NoError(err)
	require.Equal(expectedRemainingAmount, balance, "Balance after burning should be correctly reduced")
}

func TestBurnShares_Success_BurnAll(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	actorAddr := codec.Address{0xB2, 0x01, 0x02, 0x03}
	marketID := uint64(11)
	initialMintAmount := uint64(100)

	// Mint initial shares
	assetID, err := MintShares(ctx, mu, marketID, actorAddr, NoShare, initialMintAmount)
	require.NoError(err)

	// Burn all shares
	err = BurnShares(ctx, mu, marketID, actorAddr, NoShare, initialMintAmount)
	require.NoError(err, "Burning all shares should succeed") // This will fail

	// Check balance after burning all
	balance, err := GetAssetBalance(ctx, mu, actorAddr, assetID)
	// Depending on implementation, balance might be 0 or ErrNotFound if entry is deleted.
	// For now, assume it's 0 if found, or ErrNotFound.
	if err != nil {
		require.ErrorIs(err, database.ErrNotFound, "Balance should be not found if all shares are burned and entry deleted")
		require.Equal(uint64(0), balance) // GetAssetBalance returns 0 on ErrNotFound
	} else {
		require.Equal(uint64(0), balance, "Balance should be 0 after burning all shares")
	}
}

func TestBurnShares_Error_InsufficientBalance(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	actorAddr := codec.Address{0xB3, 0x01, 0x02, 0x03}
	marketID := uint64(12)
	initialMintAmount := uint64(50)
	burnAmount := uint64(100) // More than minted

	// Mint initial shares
	_, err := MintShares(ctx, mu, marketID, actorAddr, YesShare, initialMintAmount)
	require.NoError(err)

	// Attempt to burn more shares than available
	err = BurnShares(ctx, mu, marketID, actorAddr, YesShare, burnAmount)
	require.Error(err, "Expected error when burning more shares than available")
	require.ErrorContains(err, fmt.Sprintf("insufficient balance to burn %d", burnAmount))
}

func TestBurnShares_Error_BurnZeroAmount(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	actorAddr := codec.Address{0xB4, 0x01, 0x02, 0x03}
	marketID := uint64(13)

	// Mint some shares to ensure the asset exists for the actor
	_, err := MintShares(ctx, mu, marketID, actorAddr, YesShare, 10)
	require.NoError(err)

	// Attempt to burn zero shares
	err = BurnShares(ctx, mu, marketID, actorAddr, YesShare, 0)
	require.Error(err, "Expected error when burning zero amount")
	require.ErrorContains(err, "cannot burn zero amount of shares")
}

func TestBurnShares_Error_AssetNotFound_Or_ZeroBalance(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	actorAddr := codec.Address{0xB5, 0x01, 0x02, 0x03}
	marketID := uint64(14)
	burnAmount := uint64(10)

	// Attempt to burn shares for an asset the actor doesn't hold (or has zero balance)
	// No prior MintShares for this specific actor/marketID/shareType combination
	err := BurnShares(ctx, mu, marketID, actorAddr, NoShare, burnAmount)
	require.Error(err, "Expected error when burning from non-existent or zero balance asset")
	// This error message comes from when GetAssetBalance returns database.ErrNotFound inside BurnShares
	require.ErrorContains(err, "balance not found")
}
