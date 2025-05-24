package asset

import (
	"context"
	"errors" // Added for errors.Is
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state" // Added for state.Immutable
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

// getNextAssetNonce is a helper to read the next asset nonce from state for testing.
// It assumes the nonce is stored as a uint64.
func getNextAssetNonce(ctx context.Context, reader state.Immutable) (uint64, error) {
	nonceBytes, err := reader.GetValue(ctx, []byte{NextAssetNonceKey})
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			// If not found, it means the nonce hasn't been initialized.
			// DefineNewAsset should handle this by starting at an initial value (e.g., 0 or 1).
			return 0, nil 
		}
		return 0, err
	}
	if len(nonceBytes) == 0 {
		// This case should ideally not happen if GetValue returns ErrNotFound for missing keys.
		// An empty byte slice for an existing key is an invalid state for the nonce.
		return 0, fmt.Errorf("nextAssetNonceKey has empty value, expected non-empty byte slice for nonce")
	}
	// Ensure there are enough bytes for a uint64.
	// Note: hypersdk's codec.Uint64Len is not exported, but it's typically 8.
	if len(nonceBytes) < 8 { // Using 8 directly for uint64 size
		return 0, fmt.Errorf("nonce byte slice too short: got %d, want at least %d", len(nonceBytes), 8)
	}

	packer := codec.NewReader(nonceBytes, 8) // Limit read to uint64 size
	val := packer.UnpackUint64(true) // true for required
	if err := packer.Err(); err != nil {
		return 0, fmt.Errorf("failed to unpack nonce: %w", err)
	}
	return val, nil
}

func TestDefineNewAsset_SuccessAndUniqueness(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	creatorAddr1 := codec.Address{0xC1, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13}
	name1 := "Test Asset One"
	symbol1 := "TA1"
	metadata1 := []byte("metadata for asset one")

	// Initial check: nonce should not exist or be 0.
	// The DefineNewAsset function will be responsible for initializing it if it doesn't exist.
	currentNonce, err := getNextAssetNonce(ctx, mu)
	require.NoError(err, "Getting initial nonce should not fail")
	// Depending on implementation, initial nonce might be 0 if not found, or 1 after first use.
	// Let's assume DefineNewAsset initializes to 0, uses it, then stores 1.
	require.Equal(uint64(0), currentNonce, "Initial nonce should be 0 (as read by helper if not found)")

	// 1. Define first asset
	// Note: The current DefineNewAsset is a placeholder and will return an error.
	// This test is written against the expected final behavior.
	assetID1, errDef1 := DefineNewAsset(ctx, mu, creatorAddr1, name1, symbol1, metadata1, 0) // Added timestamp 0 for testing
	
	require.NoError(errDef1, "DefineNewAsset (1) should succeed")
	require.NotEqual(ids.Empty, assetID1, "AssetID1 should not be empty")

	// Verify stored definition for asset 1
	retrievedDef1, errGet1 := GetAssetDefinition(ctx, mu, assetID1)
	require.NoError(errGet1, "GetAssetDefinition (1) should succeed")
	require.NotNil(retrievedDef1, "Retrieved definition 1 should not be nil")
	require.Equal(creatorAddr1, retrievedDef1.Creator, "Creator for asset 1 mismatch")
	require.Equal(name1, retrievedDef1.Name, "Name for asset 1 mismatch")
	require.Equal(symbol1, retrievedDef1.Symbol, "Symbol for asset 1 mismatch")
	require.Equal(metadata1, retrievedDef1.Metadata, "Metadata for asset 1 mismatch")

	// Check nonce after first definition (e.g., if it starts at 0, used, then becomes 1)
	nonceAfter1, errNonce1 := getNextAssetNonce(ctx, mu)
	require.NoError(errNonce1)
	require.Equal(uint64(1), nonceAfter1, "Nonce should be 1 after first asset definition")

	// 2. Define second asset
	creatorAddr2 := codec.Address{0xC2, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13}
	name2 := "Test Asset Two"
	symbol2 := "TA2"
	metadata2 := []byte("metadata for asset two")

	assetID2, errDef2 := DefineNewAsset(ctx, mu, creatorAddr2, name2, symbol2, metadata2, 1) // Added timestamp 1 for testing
	require.NoError(errDef2, "DefineNewAsset (2) should succeed")
	require.NotEqual(ids.Empty, assetID2, "AssetID2 should not be empty")
	require.NotEqual(assetID1, assetID2, "AssetID1 and AssetID2 should be different")

	// Verify stored definition for asset 2
	retrievedDef2, errGet2 := GetAssetDefinition(ctx, mu, assetID2)
	require.NoError(errGet2, "GetAssetDefinition (2) should succeed")
	require.NotNil(retrievedDef2, "Retrieved definition 2 should not be nil")
	require.Equal(creatorAddr2, retrievedDef2.Creator, "Creator for asset 2 mismatch")
	require.Equal(name2, retrievedDef2.Name, "Name for asset 2 mismatch")
	require.Equal(symbol2, retrievedDef2.Symbol, "Symbol for asset 2 mismatch")
	require.Equal(metadata2, retrievedDef2.Metadata, "Metadata for asset 2 mismatch")
	
	// Check nonce after second definition
	nonceAfter2, errNonce2 := getNextAssetNonce(ctx, mu)
	require.NoError(errNonce2)
	require.Equal(uint64(2), nonceAfter2, "Nonce should be 2 after second asset definition")
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
