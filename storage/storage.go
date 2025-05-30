// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"encoding/binary" // Added for BalanceKey
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/hypersdk/codec"
	userConsts "github.com/chokosabe/predictionvm/consts" // Added for BalanceKey
)

type ReadState func(context.Context, [][]byte) ([][]byte, []error)

// State
// 0x0/ (hypersdk-height)
// 0x1/ (hypersdk-timestamp)
// 0x2/ (hypersdk-fee)
//
// 0x3/ (balance)
//   -> [owner] => balance

// [balancePrefix] + [address] + [chunkInfo]
func BalanceKey(addr codec.Address) (k []byte) {
	k = make([]byte, 1+codec.AddressLen+userConsts.Uint16Len)
	k[0] = userConsts.BalancePrefix
	copy(k[1:], addr[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen:], userConsts.BalanceChunks)
	return
}

/*
// If locked is 0, then account does not exist
func GetBalance(
	ctx context.Context,
	im state.Immutable,
	addr codec.Address,
) (uint64, error) {
	_, bal, _, err := getBalance(ctx, im, addr)
	return bal, err
}

func getBalance(
	ctx context.Context,
	im state.Immutable,
	addr codec.Address,
) ([]byte, uint64, bool, error) {
	k := BalanceKey(addr)
	bal, exists, err := innerGetBalance(im.GetValue(ctx, k))
	return k, bal, exists, err
}
*/

// Used to serve RPC queries
func GetBalanceFromState(
	ctx context.Context,
	f ReadState,
	addr codec.Address,
) (uint64, error) {
	k := StateKeysBalance(addr) // Replaced BalanceKey with StateKeysBalance
	values, errs := f(ctx, [][]byte{k})
	bal, _, err := innerGetBalance(values[0], errs[0])
	return bal, err
}

func innerGetBalance(
	v []byte,
	err error,
) (uint64, bool, error) {
	if errors.Is(err, database.ErrNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	val, err := database.ParseUInt64(v)
	if err != nil {
		return 0, false, err
	}
	return val, true, nil
}

/* Commented out: Replaced by logic in storage/state.go and controller/controller.go
func SetBalance(
	ctx context.Context,
	mu state.Mutable,
	addr codec.Address,
	balance uint64,
) error {
	k := BalanceKey(addr)
	return setBalance(ctx, mu, k, balance)
}

func setBalance(
	ctx context.Context,
	mu state.Mutable,
	key []byte,
	balance uint64,
) error {
	return mu.Insert(ctx, key, binary.BigEndian.AppendUint64(nil, balance))
}

func AddBalance(
	ctx context.Context,
	mu state.Mutable,
	addr codec.Address,
	amount uint64,
) (uint64, error) {
	key, bal, _, err := getBalance(ctx, mu, addr)
	if err != nil {
		return 0, err
	}
	nbal, err := smath.Add(bal, amount)
	if err != nil {
		return 0, fmt.Errorf(
			"%w: could not add balance (bal=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			bal,
			addr,
			amount,
		)
	}
	return nbal, setBalance(ctx, mu, key, nbal)
}

func SubBalance(
	ctx context.Context,
	mu state.Mutable,
	addr codec.Address,
	amount uint64,
) (uint64, error) {
	key, bal, ok, err := getBalance(ctx, mu, addr)
	if !ok {
		return 0, fmt.Errorf("%w: could not subtract (bal=%d, addr=%v, amount=%d",
			ErrInvalidBalance,
			0,
			addr,
			amount,
		)
	}
	if err != nil {
		return 0, err
	}
	nbal, err := smath.Sub(bal, amount)
	if err != nil {
		return 0, fmt.Errorf(
			"%w: could not subtract balance (bal=%d < amount=%d, gap=%d, addr=%v)",
			ErrInvalidBalance,
			bal,
			amount,
			amount-bal,
			addr,
		)
	}
	if nbal == 0 {
		// If there is no balance left, we should delete the record instead of
		// setting it to 0.
		return 0, mu.Remove(ctx, key)
	}
	return nbal, setBalance(ctx, mu, key, nbal)
}
*/
