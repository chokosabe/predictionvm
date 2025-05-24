package controller

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"

	"github.com/chokosabe/predictionvm/storage"
)

var _ chain.BalanceHandler = (*Controller)(nil)

type Controller struct {
	// state state.Mutable // Example: a way to access mutable state, provided by VM during init
	// rules chain.Rules   // Rules are not directly passed to BalanceHandler methods, VM handles fee calculation based on rules
}

func New() *Controller {
	// TODO: Initialize with necessary state access mechanisms
	return &Controller{}
}

func (c *Controller) SponsorStateKeys(addr codec.Address) state.Keys {
	key := storage.StateKeysBalance(addr)
	return state.Keys{string(key): state.Read}
}

func (c *Controller) CanDeduct(ctx context.Context, addr codec.Address, im state.Immutable, amount uint64) error {
	bal, err := storage.GetBalance(ctx, im, addr)
	if err != nil {
		return err
	}
	if bal < amount {
		return storage.ErrInsufficientBalance
	}
	return nil
}

func (c *Controller) Deduct(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64) error {
	return storage.DeductBalance(ctx, mu, addr, amount)
}

func (c *Controller) AddBalance(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64) error {
	return storage.AddBalance(ctx, mu, addr, amount)
}

func (c *Controller) GetBalance(ctx context.Context, addr codec.Address, im state.Immutable) (uint64, error) {
	return storage.GetBalance(ctx, im, addr)
}
