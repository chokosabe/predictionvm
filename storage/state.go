package storage

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database" // Used for ErrNotFound
	// "github.com/ava-labs/avalanchego/x/merkledb" // No longer needed directly for ErrNotFound
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

const (
	// BalancePrefix is the prefix for storing native token balances for accounts.
	// Format: BalancePrefix | Address -> uint64
	BalancePrefix byte = 0x0

	// MarketPrefix is the prefix for storing market data.
	// Format: MarketPrefix | MarketID (uint64) -> Market (struct)
	MarketPrefix byte = 0x1

	// ShareBalancePrefix is the prefix for storing user share balances.
	// Format: ShareBalancePrefix | MarketID (uint64) | UserAddress (codec.Address) | ShareType (uint8) -> uint64 (amount)
	ShareBalancePrefix byte = 0x2
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
)

// GetBalance retrieves the native token balance for a given address.
func GetBalance(ctx context.Context, im state.Immutable, addr codec.Address) (uint64, error) {
	key := BalanceKey(addr) // Use canonical BalanceKey function
	valBytes, err := im.GetValue(ctx, key) // Pass ctx
	if errors.Is(err, database.ErrNotFound) { // Use database.ErrNotFound
		return 0, nil // Address has no balance, treat as 0
	}
	if err != nil {
		return 0, err
	}
	if len(valBytes) == 0 {
		return 0, nil
	}
	reader := codec.NewReader(valBytes, len(valBytes))
	balance := reader.UnpackUint64(true) // true for required
	if errs := reader.Err(); errs != nil {
		return 0, errs
	}
	return balance, nil
}

// SetBalance sets the native token balance for a given address.
func SetBalance(ctx context.Context, mu state.Mutable, addr codec.Address, amount uint64) error {
	key := BalanceKey(addr) // Use canonical BalanceKey function
	writer := codec.NewWriter(8, 8) // Use literal 8 for uint64 length (8 bytes)
	writer.PackUint64(amount)
	if errs := writer.Err(); errs != nil {
		return errs
	}
	return mu.Insert(ctx, key, writer.Bytes()) // Pass ctx
}

// DeductBalance subtracts an amount from an address's native token balance.
// It returns ErrInsufficientBalance if the deduction is not possible.
func DeductBalance(ctx context.Context, mu state.Mutable, addr codec.Address, amount uint64) error {
	currentBalance, err := GetBalance(ctx, mu, addr) // Pass ctx
	if err != nil {
		return err
	}
	if currentBalance < amount {
		return ErrInsufficientBalance
	}
	newBalance := currentBalance - amount
	return SetBalance(ctx, mu, addr, newBalance) // Pass ctx
}

// AddBalance adds an amount to an address's native token balance.
func AddBalance(ctx context.Context, mu state.Mutable, addr codec.Address, amount uint64) error {
	currentBalance, err := GetBalance(ctx, mu, addr) // Pass ctx
	if err != nil {
		return err
	}
	newBalance := currentBalance + amount
	// TODO: Check for overflow if balances can become extremely large, though uint64 is quite big.
	return SetBalance(ctx, mu, addr, newBalance) // Pass ctx
}

// StateKeysBalance returns the state key for an address's native token balance.
func StateKeysBalance(addr codec.Address) []byte {
	key := make([]byte, 1+codec.AddressLen)
	key[0] = BalancePrefix
	copy(key[1:], addr[:])
	return key
}

// AddressFromKey extracts an address from a balance state key.
// Returns an error if the key is not a valid balance key.
func AddressFromKey(key []byte) (codec.Address, error) {
	if len(key) != 1+codec.AddressLen {
		return codec.Address{}, errors.New("invalid key length")
	}
	if key[0] != BalancePrefix {
		return codec.Address{}, errors.New("invalid prefix")
	}
	var addr codec.Address
	copy(addr[:], key[1:])
	return addr, nil
}

// EnsureActorHasBalance checks if an actor has at least a certain amount.
// This is useful for pre-transaction checks if not covered by MaxCost.
func EnsureActorHasBalance(ctx context.Context, im state.Immutable, actor codec.Address, required uint64) error {
	bal, err := GetBalance(ctx, im, actor) // Pass ctx
	if err != nil {
		return err
	}
	if bal < required {
		return ErrInsufficientBalance
	}
	return nil
}
