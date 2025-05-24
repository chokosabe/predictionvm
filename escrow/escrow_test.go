package escrow

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain/chaintest" // Correct import path
	"github.com/ava-labs/hypersdk/codec"
	// "github.com/ava-labs/hypersdk/state" // chaintest.InMemoryStore implements state.Mutable
	"github.com/chokosabe/predictionvm/asset"
	"github.com/stretchr/testify/require"
)

func TestUnlockCollateral_Success_PartialUnlock(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()      // Standard context
	mu := chaintest.NewInMemoryStore() // Correct way to get mutable state for testing

	marketID := ids.GenerateTestID()
	collateralAssetID := ids.GenerateTestID()
	var recipientAddr codec.Address
	copy(recipientAddr[:], []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20})

	initialEscrowAmount := uint64(1000)
	unlockAmount := uint64(400)
	expectedRemainingEscrow := initialEscrowAmount - unlockAmount
	initialRecipientBalance := uint64(50)
	expectedRecipientBalance := initialRecipientBalance + unlockAmount

	// Setup: Put initial funds into escrow
	escrowKey := GetEscrowKey(marketID, collateralAssetID)
	err := mu.Insert(ctx, escrowKey, database.PackUInt64(initialEscrowAmount))
	require.NoError(err)

	// Setup: Set initial recipient balance
	err = asset.SetAssetBalance(ctx, mu, recipientAddr, collateralAssetID, initialRecipientBalance)
	require.NoError(err)

	// Action: Unlock collateral
	err = UnlockCollateral(ctx, mu, marketID, recipientAddr, collateralAssetID, unlockAmount)
	require.NoError(err)

	// Verification: Check remaining escrow amount
	remainingEscrow, err := GetEscrowedAmount(ctx, mu, marketID, collateralAssetID)
	require.NoError(err)
	require.Equal(expectedRemainingEscrow, remainingEscrow)

	// Verification: Check recipient's new balance
	recipientBalance, err := asset.GetAssetBalance(ctx, mu, recipientAddr, collateralAssetID)
	require.NoError(err)
	require.Equal(expectedRecipientBalance, recipientBalance)
}

func TestUnlockCollateral_Success_UnlockAll(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	marketID := ids.GenerateTestID()
	collateralAssetID := ids.GenerateTestID()
	var recipientAddr codec.Address
	copy(recipientAddr[:], []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 21}) // Different recipient

	initialEscrowAmount := uint64(500)
	unlockAmount := initialEscrowAmount // Unlock all
	initialRecipientBalance := uint64(0)
	expectedRecipientBalance := initialRecipientBalance + unlockAmount

	// Setup: Put initial funds into escrow
	escrowKey := GetEscrowKey(marketID, collateralAssetID)
	err := mu.Insert(ctx, escrowKey, database.PackUInt64(initialEscrowAmount))
	require.NoError(err)

	// Setup: Set initial recipient balance (can be 0 or some value)
	err = asset.SetAssetBalance(ctx, mu, recipientAddr, collateralAssetID, initialRecipientBalance)
	require.NoError(err)

	// Action: Unlock all collateral
	err = UnlockCollateral(ctx, mu, marketID, recipientAddr, collateralAssetID, unlockAmount)
	require.NoError(err)

	// Verification: Check remaining escrow amount (should be 0, and key potentially removed)
	remainingEscrow, err := GetEscrowedAmount(ctx, mu, marketID, collateralAssetID)
	require.NoError(err) // GetEscrowedAmount returns 0, nil if not found
	require.Equal(uint64(0), remainingEscrow, "Escrow amount should be zero after unlocking all")

	// Optional: Verify the key is actually removed from state if that's an explicit guarantee
	// val, err := mu.GetValue(ctx, escrowKey)
	// require.ErrorIs(err, database.ErrNotFound, "Escrow key should be removed from state")
	// require.Nil(val)

	// Verification: Check recipient's new balance
	recipientBalanceAfterUnlock, err := asset.GetAssetBalance(ctx, mu, recipientAddr, collateralAssetID)
	require.NoError(err)
	require.Equal(expectedRecipientBalance, recipientBalanceAfterUnlock)
}

func TestUnlockCollateral_Error_AmountIsZero(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	marketID := ids.GenerateTestID()
	collateralAssetID := ids.GenerateTestID()
	var recipientAddr codec.Address
	copy(recipientAddr[:], []byte{1, 2, 3})

	initialEscrowAmount := uint64(100)
	unlockAmount := uint64(0) // Attempt to unlock zero
	initialRecipientBalance := uint64(10)

	// Setup: Put initial funds into escrow
	escrowKey := GetEscrowKey(marketID, collateralAssetID)
	err := mu.Insert(ctx, escrowKey, database.PackUInt64(initialEscrowAmount))
	require.NoError(err)

	// Setup: Set initial recipient balance
	err = asset.SetAssetBalance(ctx, mu, recipientAddr, collateralAssetID, initialRecipientBalance)
	require.NoError(err)

	// Action: Attempt to unlock zero collateral
	err = UnlockCollateral(ctx, mu, marketID, recipientAddr, collateralAssetID, unlockAmount)

	// Verification: Check for expected error
	require.Error(err)
	require.ErrorIs(err, ErrAmountCannotBeZero)

	// Verification: Ensure escrow amount is unchanged
	escrowAmountAfter, err := GetEscrowedAmount(ctx, mu, marketID, collateralAssetID)
	require.NoError(err)
	require.Equal(initialEscrowAmount, escrowAmountAfter, "Escrow amount should not change")

	// Verification: Ensure recipient balance is unchanged
	recipientBalanceAfter, err := asset.GetAssetBalance(ctx, mu, recipientAddr, collateralAssetID)
	require.NoError(err)
	require.Equal(initialRecipientBalance, recipientBalanceAfter, "Recipient balance should not change")
}

func TestUnlockCollateral_Error_InsufficientFunds(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	marketID := ids.GenerateTestID()
	collateralAssetID := ids.GenerateTestID()
	var recipientAddr codec.Address
	copy(recipientAddr[:], []byte{1, 2, 3, 4})

	initialEscrowAmount := uint64(100)
	unlockAmount := initialEscrowAmount + 1 // Attempt to unlock more than available
	initialRecipientBalance := uint64(20)

	// Setup: Put initial funds into escrow
	escrowKey := GetEscrowKey(marketID, collateralAssetID)
	err := mu.Insert(ctx, escrowKey, database.PackUInt64(initialEscrowAmount))
	require.NoError(err)

	// Setup: Set initial recipient balance
	err = asset.SetAssetBalance(ctx, mu, recipientAddr, collateralAssetID, initialRecipientBalance)
	require.NoError(err)

	// Action: Attempt to unlock collateral
	err = UnlockCollateral(ctx, mu, marketID, recipientAddr, collateralAssetID, unlockAmount)

	// Verification: Check for expected error
	require.Error(err)
	require.ErrorIs(err, ErrInsufficientFundsInEscrow)

	// Verification: Ensure escrow amount is unchanged
	escrowAmountAfter, err := GetEscrowedAmount(ctx, mu, marketID, collateralAssetID)
	require.NoError(err)
	require.Equal(initialEscrowAmount, escrowAmountAfter, "Escrow amount should not change")

	// Verification: Ensure recipient balance is unchanged
	recipientBalanceAfterUnlock, err := asset.GetAssetBalance(ctx, mu, recipientAddr, collateralAssetID)
	require.NoError(err)
	require.Equal(initialRecipientBalance, recipientBalanceAfterUnlock, "Recipient balance should not change")
}

func TestLockCollateral_Success_InitialLock(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	marketID := ids.GenerateTestID()
	collateralAssetID := ids.GenerateTestID()
	var actorAddr codec.Address
	copy(actorAddr[:], []byte{5, 6, 7, 8})

	initialActorBalance := uint64(1000)
	lockAmount := uint64(300)
	expectedActorBalance := initialActorBalance - lockAmount
	expectedEscrowAmount := lockAmount

	// Setup: Set initial actor balance
	err := asset.SetAssetBalance(ctx, mu, actorAddr, collateralAssetID, initialActorBalance)
	require.NoError(err)

	// Action: Lock collateral
	err = LockCollateral(ctx, mu, marketID, actorAddr, collateralAssetID, lockAmount)
	require.NoError(err)

	// Verification: Check actor's new balance
	actorBalanceAfter, err := asset.GetAssetBalance(ctx, mu, actorAddr, collateralAssetID)
	require.NoError(err)
	require.Equal(expectedActorBalance, actorBalanceAfter, "Actor balance incorrect after lock")

	// Verification: Check escrowed amount
	escrowAmount, err := GetEscrowedAmount(ctx, mu, marketID, collateralAssetID)
	require.NoError(err)
	require.Equal(expectedEscrowAmount, escrowAmount, "Escrow amount incorrect after lock")
}

func TestLockCollateral_Success_LockAdditional(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	marketID := ids.GenerateTestID()
	collateralAssetID := ids.GenerateTestID()
	var actorAddr codec.Address
	copy(actorAddr[:], []byte{9, 10, 11, 12})

	initialActorBalance := uint64(1000)
	firstLockAmount := uint64(200)
	secondLockAmount := uint64(150)
	totalLockedAmount := firstLockAmount + secondLockAmount
	expectedActorBalance := initialActorBalance - totalLockedAmount

	// Setup: Set initial actor balance
	err := asset.SetAssetBalance(ctx, mu, actorAddr, collateralAssetID, initialActorBalance)
	require.NoError(err)

	// Action: First lock
	err = LockCollateral(ctx, mu, marketID, actorAddr, collateralAssetID, firstLockAmount)
	require.NoError(err)

	// Action: Second lock (additional)
	err = LockCollateral(ctx, mu, marketID, actorAddr, collateralAssetID, secondLockAmount)
	require.NoError(err)

	// Verification: Check actor's new balance
	actorBalanceAfter, err := asset.GetAssetBalance(ctx, mu, actorAddr, collateralAssetID)
	require.NoError(err)
	require.Equal(expectedActorBalance, actorBalanceAfter, "Actor balance incorrect after second lock")

	// Verification: Check total escrowed amount
	escrowAmount, err := GetEscrowedAmount(ctx, mu, marketID, collateralAssetID)
	require.NoError(err)
	require.Equal(totalLockedAmount, escrowAmount, "Total escrow amount incorrect after second lock")
}

func TestLockCollateral_Error_AmountIsZero(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	marketID := ids.GenerateTestID()
	collateralAssetID := ids.GenerateTestID()
	var actorAddr codec.Address
	copy(actorAddr[:], []byte{13, 14, 15, 16})

	initialActorBalance := uint64(500)
	lockAmount := uint64(0) // Attempt to lock zero
	initialEscrowAmount := uint64(0) // Assuming no prior escrow for this test

	// Setup: Set initial actor balance
	err := asset.SetAssetBalance(ctx, mu, actorAddr, collateralAssetID, initialActorBalance)
	require.NoError(err)

	// Setup: Ensure initial escrow is 0 (or not set, GetEscrowedAmount handles this)
	// No explicit setup needed if we expect it to be 0 initially.

	// Action: Attempt to lock zero collateral
	err = LockCollateral(ctx, mu, marketID, actorAddr, collateralAssetID, lockAmount)

	// Verification: Check for expected error
	require.Error(err)
	require.ErrorIs(err, ErrAmountCannotBeZero)

	// Verification: Ensure actor balance is unchanged
	actorBalanceAfter, err := asset.GetAssetBalance(ctx, mu, actorAddr, collateralAssetID)
	require.NoError(err)
	require.Equal(initialActorBalance, actorBalanceAfter, "Actor balance should not change")

	// Verification: Ensure escrow amount is unchanged
	escrowAmountAfter, err := GetEscrowedAmount(ctx, mu, marketID, collateralAssetID)
	require.NoError(err)
	require.Equal(initialEscrowAmount, escrowAmountAfter, "Escrow amount should not change")
}

func TestLockCollateral_Error_InsufficientBalance(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	mu := chaintest.NewInMemoryStore()

	marketID := ids.GenerateTestID()
	collateralAssetID := ids.GenerateTestID()
	var actorAddr codec.Address
	copy(actorAddr[:], []byte{17, 18, 19, 20})

	initialActorBalance := uint64(100)
	lockAmount := initialActorBalance + 1 // Attempt to lock more than available balance
	initialEscrowAmount := uint64(0)      // Assuming no prior escrow

	// Setup: Set initial actor balance
	err := asset.SetAssetBalance(ctx, mu, actorAddr, collateralAssetID, initialActorBalance)
	require.NoError(err)

	// Action: Attempt to lock collateral with insufficient balance
	err = LockCollateral(ctx, mu, marketID, actorAddr, collateralAssetID, lockAmount)

	// Verification: Check for expected error (asset.ErrInsufficientBalance)
	require.Error(err)
	require.ErrorIs(err, asset.ErrInsufficientBalance) // LockCollateral should propagate this error

	// Verification: Ensure actor balance is unchanged
	actorBalanceAfter, err := asset.GetAssetBalance(ctx, mu, actorAddr, collateralAssetID)
	require.NoError(err)
	require.Equal(initialActorBalance, actorBalanceAfter, "Actor balance should not change")

	// Verification: Ensure escrow amount is unchanged
	escrowAmountAfterLock, err := GetEscrowedAmount(ctx, mu, marketID, collateralAssetID)
	require.NoError(err)
	require.Equal(initialEscrowAmount, escrowAmountAfterLock, "Escrow amount should not change")
}
