package asset

import (
	"context"
	"crypto/sha256" // Added for SHA256 hashing
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	// "github.com/chokosabe/predictionvm/storage" // May need for share type or other consts
)

// ShareType defines the type of prediction market share.
type ShareType int

const (
	YesShare ShareType = 0
	NoShare  ShareType = 1
)

// String returns a string representation of the ShareType.
func (st ShareType) String() string {
	switch st {
	case YesShare:
		return "YesShare"
	case NoShare:
		return "NoShare"
	default:
		return fmt.Sprintf("UnknownShareType(%d)", st)
	}
}

// MintShares creates new shares for a given market and actor.
// It returns the asset ID of the minted shares and an error if any.
func MintShares(
	ctx context.Context,
	mu state.Mutable,
	marketID uint64,
	actor codec.Address,
	shareType ShareType,
	amount uint64,
) (ids.ID, error) {
	if amount == 0 {
		return ids.Empty, fmt.Errorf("cannot mint zero amount of shares")
	}
	assetID, err := GetShareAssetID(marketID, shareType)
	if err != nil {
		return ids.Empty, fmt.Errorf("failed to get share asset ID: %w", err)
	}

	currentBalance, err := GetAssetBalance(ctx, mu, actor, assetID)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return ids.Empty, fmt.Errorf("failed to get current asset balance for actor %s, asset %s: %w", actor, assetID, err)
	}

	newBalance := currentBalance + amount
	if err := SetAssetBalance(ctx, mu, actor, assetID, newBalance); err != nil {
		return ids.Empty, fmt.Errorf("failed to set new asset balance for actor %s, asset %s: %w", actor, assetID, err)
	}

	// TODO: Potentially register the new asset if it's the first mint (e.g., in a separate asset registry).
	// For now, the existence of a balance implies the asset's conceptual existence.

	return assetID, nil
}

// GetShareAssetID derives or retrieves the asset ID for a given market and share type.
// This needs a robust and deterministic way to generate unique asset IDs.
// Current implementation is a placeholder and needs to be made robust.
func GetShareAssetID(marketID uint64, shareType ShareType) (ids.ID, error) {
	if shareType != YesShare && shareType != NoShare {
		return ids.Empty, fmt.Errorf("unknown share type: %s", shareType.String())
	}

	// Create a unique seed string for the hash
	seedString := fmt.Sprintf("marketID:%d_shareType:%s", marketID, shareType.String())

	// Compute SHA256 hash
	hash := sha256.Sum256([]byte(seedString))

	// Convert the hash to ids.ID
	// ids.ID is [32]byte, which is the same size as sha256.Sum256 output
	return ids.ID(hash), nil
}

// GetAssetBalance retrieves the balance of a specific asset for an actor.
// The key is constructed as: actorAddress + assetID
func GetAssetBalance(
	ctx context.Context,
	reader state.Mutable,
	actor codec.Address,
	assetID ids.ID,
) (uint64, error) {
	key := make([]byte, 0, codec.AddressLen+ids.IDLen)
	key = append(key, actor[:]...)
	key = append(key, assetID[:]...)
	valBytes, err := reader.GetValue(ctx, key)
	if err != nil {
		// If ErrNotFound is returned by GetValue, we want to propagate it so GetAssetBalance
		// can correctly indicate that the balance doesn't exist (which implies 0 for our logic).
		// database.ParseUInt64 would fail on nil bytes from ErrNotFound anyway.
		return 0, err 
	}
	return database.ParseUInt64(valBytes)
}

// SetAssetBalance sets the balance of a specific asset for an actor.
// The key is constructed as: actorAddress + assetID

// BurnShares decreases the shares for a given market and actor.
// It returns an error if any.
func BurnShares(
	ctx context.Context,
	mu state.Mutable,
	marketID uint64,
	actor codec.Address,
	shareType ShareType,
	amount uint64,
) error {
	// Placeholder: In a real implementation, this would:
	if amount == 0 {
		return fmt.Errorf("cannot burn zero amount of shares")
	}

	assetID, err := GetShareAssetID(marketID, shareType)
	if err != nil {
		return fmt.Errorf("failed to get share asset ID for burn: %w", err)
	}

	currentBalance, err := GetAssetBalance(ctx, mu, actor, assetID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("actor %s has no %s shares (asset %s) for market %d to burn: balance not found", actor, shareType.String(), assetID, marketID)
		}
		return fmt.Errorf("failed to get current asset balance for burn (actor %s, asset %s): %w", actor, assetID, err)
	}

	if currentBalance < amount {
		return fmt.Errorf("insufficient balance to burn %d %s shares for actor %s, asset %s: current balance %d", amount, shareType.String(), actor, assetID, currentBalance)
	}

	newBalance := currentBalance - amount

	if newBalance == 0 {
		// If balance is zero, remove the state entry
		key := make([]byte, 0, codec.AddressLen+ids.IDLen)
		key = append(key, actor[:]...)
		key = append(key, assetID[:]...)
		if err := mu.Remove(ctx, key); err != nil {
			return fmt.Errorf("failed to delete zero balance entry for actor %s, asset %s: %w", actor, assetID, err)
		}
	} else {
		// Otherwise, set the new balance
		if err := SetAssetBalance(ctx, mu, actor, assetID, newBalance); err != nil {
			return fmt.Errorf("failed to set new asset balance after burn for actor %s, asset %s: %w", actor, assetID, err)
		}
	}

	return nil
}

// SetAssetBalance sets the balance of a specific asset for an actor.
// The key is constructed as: actorAddress + assetID
func SetAssetBalance(
	ctx context.Context,
	mu state.Mutable,
	actor codec.Address,
	assetID ids.ID,
	balance uint64,
) error {
	key := make([]byte, 0, codec.AddressLen+ids.IDLen)
	key = append(key, actor[:]...)
	key = append(key, assetID[:]...)
	return mu.Insert(ctx, key, database.PackUInt64(balance))
}
