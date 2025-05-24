package asset

import (
	"context"
	"crypto/sha256" // Used by hashing.ComputeHash256 indirectly
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing" // Added for ComputeHash256
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
)

// ShareType defines the type of prediction market share.
type ShareType int

const (
	YesShare ShareType = 0
	NoShare  ShareType = 1
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
)

const (
	// Key for the next available nonce to generate unique asset IDs.
	NextAssetNonceKey byte = 0x01 // Simple prefix, actual key will be this byte itself

	// Prefix for storing asset definitions.
	// Key format: assetDefinitionPrefix | assetID[:]
	assetDefinitionPrefix byte = 0x02

	// Estimated buffer sizes for codec operations
	initialAssetDefBufferSize = 256
	// defaultMaxSliceLen is a common default for max slice/array lengths (256 KiB).
	defaultMaxSliceLen = 1 << 18
)

// AssetDefinition stores metadata about a defined asset.
// The AssetID itself is the key in the database.
type AssetDefinition struct {
	Creator  codec.Address `json:"creator"`
	Created  uint64        `json:"created"`
	Name     string        `json:"name"`
	Symbol   string        `json:"symbol"`
	Metadata []byte        `json:"metadata"`
}

// MarshalCodec serializes AssetDefinition into a Packer.
func (ad *AssetDefinition) MarshalCodec(p *codec.Packer) error {
	p.PackAddress(ad.Creator)
	p.PackLong(ad.Created)
	p.PackString(ad.Name)
	p.PackString(ad.Symbol)
	p.PackBytes(ad.Metadata)
	return p.Err()
}

// UnmarshalCodec deserializes bytes from a Packer into an AssetDefinition.
func (ad *AssetDefinition) UnmarshalCodec(p *codec.Packer) error {
	p.UnpackAddress(&ad.Creator)
	ad.Created = p.UnpackLong() // Corrected: UnpackLong takes no arguments
	ad.Name = p.UnpackString(true)
	ad.Symbol = p.UnpackString(true)
	p.UnpackBytes(defaultMaxSliceLen, true, &ad.Metadata) // Corrected: Use defaultMaxSliceLen
	return p.Err()
}

// GetAssetDefinitionKey returns the state key for a given assetID's definition.
func GetAssetDefinitionKey(assetID ids.ID) []byte {
	key := make([]byte, 1+ids.IDLen)
	key[0] = assetDefinitionPrefix
	copy(key[1:], assetID[:])
	return key
}

// GetAssetDefinition retrieves an asset's definition from state.
func GetAssetDefinition(ctx context.Context, reader state.Immutable, assetID ids.ID) (*AssetDefinition, error) {
	key := GetAssetDefinitionKey(assetID)
	valBytes, err := reader.GetValue(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get asset definition for %s: %w", assetID, err)
	}
	if len(valBytes) == 0 {
		return nil, fmt.Errorf("asset definition not found for %s (empty value)", assetID)
	}

	ad := &AssetDefinition{}
	unpacker := codec.NewReader(valBytes, (1 << 18))
	if err := ad.UnmarshalCodec(unpacker); err != nil {
		return nil, fmt.Errorf("failed to unmarshal asset definition for %s: %w", assetID, err)
	}
	return ad, nil
}

// DefineNewAsset creates a definition for a new asset type in the state.
// It generates a unique assetID for the new asset.
func DefineNewAsset(
	ctx context.Context,
	mu state.Mutable,
	creator codec.Address,
	name string,
	symbol string,
	metadata []byte,
	timestamp uint64, // Added timestamp parameter
) (ids.ID, error) {
	nonceBytes, err := mu.GetValue(ctx, []byte{NextAssetNonceKey})
	var currentNonce uint64
	if err != nil {
		if err == database.ErrNotFound {
			currentNonce = 0
		} else {
			return ids.Empty, fmt.Errorf("failed to get next asset nonce: %w", err)
		}
	} else {
		if len(nonceBytes) == 0 {
			currentNonce = 0
		} else {
			packer := codec.NewReader(nonceBytes, consts.Uint64Len)
			currentNonce = packer.UnpackUint64(true)
			if err := packer.Err(); err != nil {
				return ids.Empty, fmt.Errorf("failed to unpack current asset nonce: %w", err)
			}
		}
	}

	assetIDHasher := wrappers.Packer{Bytes: make([]byte, consts.Uint64Len)} // Initialize with length for PackLong
	assetIDHasher.PackLong(currentNonce)
	if assetIDHasher.Err != nil {
		return ids.Empty, fmt.Errorf("failed to pack nonce for asset ID generation: %w", assetIDHasher.Err)
	}
	assetID := ids.ID(hashing.ComputeHash256(assetIDHasher.Bytes)) // Corrected: Use hashing.ComputeHash256

	definition := &AssetDefinition{
		Creator:  creator,
		Created:  timestamp, // Corrected: Use passed timestamp
		Name:     name,
		Symbol:   symbol,
		Metadata: metadata,
	}

	defKey := GetAssetDefinitionKey(assetID)
	// Correctly marshal AssetDefinition to bytes
	// Assuming a max capacity, e.g., twice the initial, or a defined constant like defaultMaxSliceLen if appropriate.
	packer := codec.NewWriter(initialAssetDefBufferSize, initialAssetDefBufferSize*2) 
	if err := definition.MarshalCodec(packer); err != nil { // Pass packer directly
		return ids.Empty, fmt.Errorf("failed to marshal asset definition: %w", err)
	}
	defBytes := packer.Bytes() // Call Bytes() method

	if err := mu.Insert(ctx, defKey, defBytes); err != nil {
		return ids.Empty, fmt.Errorf("failed to store asset definition: %w", err)
	}

	nextNonce := currentNonce + 1
	noncePacker := wrappers.Packer{Bytes: make([]byte, consts.Uint64Len)} // Initialize with length for PackLong
	noncePacker.PackLong(nextNonce)
	if noncePacker.Err != nil {
		return ids.Empty, fmt.Errorf("failed to pack next asset nonce: %w", noncePacker.Err)
	}
	if err := mu.Insert(ctx, []byte{NextAssetNonceKey}, noncePacker.Bytes); err != nil {
		return ids.Empty, fmt.Errorf("failed to store next asset nonce: %w", err)
	}

	return assetID, nil
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

// GetBalanceKey returns the state key for an actor's balance of a specific asset.
// Key format: actorAddress | assetID
func GetBalanceKey(actor codec.Address, assetID ids.ID) []byte {
	key := make([]byte, 0, codec.AddressLen+ids.IDLen)
	key = append(key, actor[:]...)
	key = append(key, assetID[:]...)
	return key
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
	key := GetBalanceKey(actor, assetID)
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
		return fmt.Errorf("%w to burn %d %s shares for actor %s, asset %s: current balance %d", ErrInsufficientBalance, amount, shareType.String(), actor, assetID, currentBalance)
	}

	newBalance := currentBalance - amount

	if newBalance == 0 {
		// If balance is zero, remove the state entry
		key := GetBalanceKey(actor, assetID)
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
	key := GetBalanceKey(actor, assetID)
	return mu.Insert(ctx, key, database.PackUInt64(balance))
}

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
