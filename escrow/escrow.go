package escrow

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/chokosabe/predictionvm/asset" // For asset.GetAssetBalance, asset.SetAssetBalance
	// "github.com/chokosabe/predictionvm/consts" // If any constants are needed
)

const (
	// EscrowPrefix is the state prefix for escrowed funds.
	// Key format: EscrowPrefix | marketID | collateralAssetID -> escrowedAmount (uint64)
	EscrowPrefix byte = 0x03
)

var (
	ErrInsufficientFundsInEscrow = errors.New("insufficient funds in escrow")
	// ErrInsufficientActorBalance is now sourced from the asset package
	ErrAmountCannotBeZero        = errors.New("amount cannot be zero")
)

// GetEscrowKey generates the state key for an escrow entry.
// Key: EscrowPrefix | marketID | collateralAssetID
func GetEscrowKey(marketID ids.ID, collateralAssetID ids.ID) []byte {
	key := make([]byte, 1+ids.IDLen+ids.IDLen)
	key[0] = EscrowPrefix
	copy(key[1:], marketID[:])
	copy(key[1+ids.IDLen:], collateralAssetID[:])
	return key
}

// LockCollateral locks the specified amount of collateral from the actor into the market's escrow.
func LockCollateral(
	ctx context.Context,
	mu state.Mutable,
	marketID ids.ID,
	actor codec.Address,
	collateralAssetID ids.ID,
	amount uint64,
) error {
	if amount == 0 {
		return ErrAmountCannotBeZero
	}

	// 1. Get actor's current balance
	actorBalance, err := asset.GetAssetBalance(ctx, mu, actor, collateralAssetID)
	if err != nil {
		// If ErrNotFound, it means balance is 0, which is handled by the check below.
		// For other errors, return them.
		if !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("failed to get actor %s balance for asset %s: %w", actor, collateralAssetID, err)
		}
		// If ErrNotFound, actorBalance will be 0, which is fine, the next check handles it.
	}

	// 2. Check if actor has sufficient balance
	if actorBalance < amount {
		return fmt.Errorf("%w: actor %s has %d, needs %d of asset %s",
			asset.ErrInsufficientBalance, actor, actorBalance, amount, collateralAssetID)
	}

	// 3. Update actor's balance
	newActorBalance := actorBalance - amount
	if err := asset.SetAssetBalance(ctx, mu, actor, collateralAssetID, newActorBalance); err != nil {
		return fmt.Errorf("failed to set new actor balance for %s after locking collateral: %w", actor, err)
	}

	// 4. Update escrowed amount
	currentEscrowedAmount, err := GetEscrowedAmount(ctx, mu, marketID, collateralAssetID)
	if err != nil {
		// This error comes from GetEscrowedAmount's own GetValue or ParseUInt64, so it's already descriptive.
		return fmt.Errorf("failed to get current escrowed amount before locking: %w", err)
	}

	newEscrowedAmount := currentEscrowedAmount + amount
	packedNewEscrowedAmount := database.PackUInt64(newEscrowedAmount)
	escrowKey := GetEscrowKey(marketID, collateralAssetID)

	if err := mu.Insert(ctx, escrowKey, packedNewEscrowedAmount); err != nil {
		// Attempt to revert actor's balance if escrow update fails. This is a best-effort.
		// A more robust solution might involve a multi-operation transaction abstraction if available.
		if revertErr := asset.SetAssetBalance(ctx, mu, actor, collateralAssetID, actorBalance); revertErr != nil {
			return fmt.Errorf("failed to insert new escrow amount (and failed to revert actor balance %s): %w (revert error: %v)", actor, err, revertErr)
		}
		return fmt.Errorf("failed to insert new escrow amount for market %s, asset %s (actor balance reverted): %w", marketID, collateralAssetID, err)
	}

	return nil
}

// UnlockCollateral unlocks the specified amount of collateral from the market's escrow to the recipient.
func UnlockCollateral(
	ctx context.Context,
	mu state.Mutable,
	marketID ids.ID,
	recipient codec.Address,
	collateralAssetID ids.ID,
	amount uint64,
) error {
	if amount == 0 {
		return ErrAmountCannotBeZero
	}

	// 1. Get current escrowed amount
	currentEscrowedAmount, err := GetEscrowedAmount(ctx, mu, marketID, collateralAssetID)
	if err != nil {
		return fmt.Errorf("failed to get current escrowed amount for market %s, asset %s before unlocking: %w", marketID, collateralAssetID, err)
	}

	// 2. Check if escrow has sufficient funds
	if currentEscrowedAmount < amount {
		return fmt.Errorf("%w: market %s, asset %s has %d, needs to unlock %d",
			ErrInsufficientFundsInEscrow, marketID, collateralAssetID, currentEscrowedAmount, amount)
	}

	// 3. Update escrowed amount
	newEscrowedAmount := currentEscrowedAmount - amount
	escrowKey := GetEscrowKey(marketID, collateralAssetID)

	if newEscrowedAmount == 0 {
		if err := mu.Remove(ctx, escrowKey); err != nil {
			return fmt.Errorf("failed to remove zeroed escrow entry for market %s, asset %s: %w", marketID, collateralAssetID, err)
		}
	} else {
		packedNewEscrowedAmount := database.PackUInt64(newEscrowedAmount)
		if err := mu.Insert(ctx, escrowKey, packedNewEscrowedAmount); err != nil {
			return fmt.Errorf("failed to update escrow amount for market %s, asset %s: %w", marketID, collateralAssetID, err)
		}
	}

	// 4. Update recipient's balance
	recipientBalance, err := asset.GetAssetBalance(ctx, mu, recipient, collateralAssetID)
	if err != nil {
		if !errors.Is(err, database.ErrNotFound) {
			// If error is not NotFound, it's an unexpected issue. Attempt to revert escrow state.
			// This is a best-effort. A more robust solution might involve a multi-operation transaction abstraction.
			originalEscrowPacked := database.PackUInt64(currentEscrowedAmount)
			if revertErr := mu.Insert(ctx, escrowKey, originalEscrowPacked); revertErr != nil {
				return fmt.Errorf("failed to get recipient %s balance (and failed to revert escrow state): %w (revert error: %v)", recipient, err, revertErr)
			}
			return fmt.Errorf("failed to get recipient %s balance (escrow state reverted): %w", recipient, err)
		}
		// If ErrNotFound, recipientBalance is 0, which is fine.
	}

	newRecipientBalance := recipientBalance + amount
	if err := asset.SetAssetBalance(ctx, mu, recipient, collateralAssetID, newRecipientBalance); err != nil {
		// Attempt to revert escrow state if recipient balance update fails.
		originalEscrowPacked := database.PackUInt64(currentEscrowedAmount)
		if revertErr := mu.Insert(ctx, escrowKey, originalEscrowPacked); revertErr != nil {
			return fmt.Errorf("failed to set new recipient %s balance (and failed to revert escrow state): %w (revert error: %v)", recipient, err, revertErr)
		}
		return fmt.Errorf("failed to set new recipient %s balance (escrow state reverted): %w", recipient, err)
	}

	return nil
}

// GetEscrowedAmount returns the amount of collateral escrowed for a given market and asset.
func GetEscrowedAmount(
	ctx context.Context,
	immu state.Immutable,
	marketID ids.ID,
	collateralAssetID ids.ID,
) (uint64, error) {
	key := GetEscrowKey(marketID, collateralAssetID)
	val, err := immu.GetValue(ctx, key)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			// No escrow entry found, means 0 collateral
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get escrowed amount for market %s, asset %s: %w", marketID, collateralAssetID, err)
	}
	amount, err := database.ParseUInt64(val)
	if err != nil {
		return 0, fmt.Errorf("failed to parse escrowed amount for market %s, asset %s: %w", marketID, collateralAssetID, err)
	}
	return amount, nil
}
