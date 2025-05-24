package actions

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"

	userConsts "github.com/chokosabe/predictionvm/consts"
	"github.com/chokosabe/predictionvm/storage"
)

var _ chain.Action = (*BuyNo)(nil)

// BuyNo represents an action where a user buys NO shares for a specific market.
type BuyNo struct {
	MarketID uint64 `serialize:"true" json:"marketId"`
	Amount   uint64 `serialize:"true" json:"amount"`
	MaxPrice uint64 `serialize:"true" json:"maxPrice"`
}

// GetTypeID implements chain.Action
func (b *BuyNo) GetTypeID() uint8 {
	return userConsts.BuyNoID
}

// StateKeys implements chain.Action
func (b *BuyNo) StateKeys(actor codec.Address, chainID ids.ID) state.Keys {
	return state.Keys{
		string(storage.BalanceKey(actor)): state.Read | state.Write,
		string(storage.MarketKey(b.MarketID)): state.Read | state.Write,
		string(storage.ShareBalanceKey(b.MarketID, actor, userConsts.NoShareType)): state.Read | state.Write,
	}
}

// Execute performs the action of buying NO shares for a given market.
func (b *BuyNo) Execute(
	ctx context.Context,
	rules chain.Rules,
	mu state.Mutable,
	txTimestamp int64,
	actor codec.Address,
	txID ids.ID,
) ([]byte, error) {
	// Basic validation
	if b.Amount == 0 {
		return nil, ErrAmountCannotBeZero
	}
	if b.MaxPrice == 0 {
		return nil, ErrMaxPriceCannotBeZero
	}

	// 1. Check if market exists and is active
	market, err := storage.GetMarket(ctx, mu, b.MarketID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) { // Corrected to database.ErrNotFound
			return nil, fmt.Errorf("%w: market %d not found when fetching", ErrMarketNotFound, b.MarketID)
		}
		return nil, fmt.Errorf("failed to get market %d: %w", b.MarketID, err)
	}
	if market.Status == storage.MarketStatus_ResolvedYes || market.Status == storage.MarketStatus_ResolvedNo {
		return nil, fmt.Errorf("%w: market %d is already resolved (status: %s)", ErrMarketInteraction, b.MarketID, market.Status.String())
	}
	if market.Status == storage.MarketStatus_TradingClosed {
		return nil, fmt.Errorf("%w: market %d trading is closed (status: %s)", ErrMarketInteraction, b.MarketID, market.Status.String())
	}
	if txTimestamp > market.EndTime {
		return nil, fmt.Errorf("%w: market %d has ended (current: %d, end: %d)", ErrMarketInteraction, b.MarketID, txTimestamp, market.EndTime)
	}

	// 2. Check actor's balance
	balanceKey := storage.BalanceKey(actor)
	currentBalanceBytes, err := mu.GetValue(ctx, balanceKey)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) { // Corrected to database.ErrNotFound
			cost := b.Amount * b.MaxPrice // Calculate cost for error message
			return nil, fmt.Errorf("%w: actor %s has no balance record, cost is %d", ErrInsufficientFunds, actor.String(), cost)
		}
		return nil, fmt.Errorf("failed to get actor's balance for %s: %w", actor.String(), err)
	}

	currentBalance, err := database.ParseUInt64(currentBalanceBytes) // Corrected to database.ParseUInt64
	if err != nil {
		return nil, fmt.Errorf("failed to parse actor's balance for %s: %w", actor.String(), err)
	}

	// 3. Calculate cost and ensure actor has enough funds
	cost := b.Amount * b.MaxPrice
	if currentBalance < cost {
		return nil, fmt.Errorf("%w: actor balance %d, cost %d for %d shares at max price %d for market %d", ErrInsufficientFunds, currentBalance, cost, b.Amount, b.MaxPrice, b.MarketID)
	}

	// 4. Deduct funds
	newBalance := currentBalance - cost
	if err := mu.Insert(ctx, balanceKey, database.PackUInt64(newBalance)); err != nil { // Corrected to database.PackUInt64
		return nil, fmt.Errorf("failed to set new balance %d for actor %s: %w", newBalance, actor.String(), err)
	}

	// 5. Credit NO shares to actor
	currentNoShares, err := storage.GetShareBalance(ctx, mu, b.MarketID, actor, userConsts.NoShareType)
	if err != nil && !errors.Is(err, database.ErrNotFound) { // Corrected to database.ErrNotFound, treat as 0 shares
		// Consider reverting native token balance change here or using a more transactional approach
		return nil, fmt.Errorf("failed to get current NO share balance for actor %s, market %d: %w", actor.String(), b.MarketID, err)
	}

	newShareBalance := currentNoShares + b.Amount
	if err := storage.SetShareBalance(ctx, mu, b.MarketID, actor, userConsts.NoShareType, newShareBalance); err != nil {
		// Consider reverting native token balance change here
		return nil, fmt.Errorf("failed to set new NO share balance %d for actor %s, market %d: %w", newShareBalance, actor.String(), b.MarketID, err)
	}

	// 6. Update market's total NO shares
	market.TotalNoShares += b.Amount
	if err := storage.SetMarket(ctx, mu, market); err != nil {
		// Consider reverting previous state changes (actor balance, share balance)
		return nil, fmt.Errorf("failed to update market %d with new total NO shares: %w", b.MarketID, err)
	}

	// TODO: Emit event for BuyNo action

	return nil, nil // Success
}

// ComputeUnits implements chain.Action
func (b *BuyNo) ComputeUnits(rules chain.Rules) uint64 {
	return 0 // Placeholder, to align with test expectation until proper fee/unit calculation
}

// ValidRange implements chain.Action
func (b *BuyNo) ValidRange(rules chain.Rules) (int64, int64) {
	return -1, -1 // Placeholder, means valid at any time
}

// MaxGas implements chain.Action
func (b *BuyNo) MaxGas(rules chain.Rules) uint64 {
	return 0 // Placeholder
}

// Bytes implements chain.Action
func (b *BuyNo) Bytes() []byte {
	packer := codec.NewWriter(0, userConsts.MaxActionSize)
	if err := codec.LinearCodec.MarshalInto(b, packer.Packer); err != nil {
		// This should ideally not happen with a well-defined struct
		// and could panic or log fatally in a real scenario.
		// Consider returning an error or panicking if appropriate for the VM's error handling strategy.
		// For now, following BuyYes pattern which prints and returns nil, though panicking might be better.
		fmt.Printf("Error marshalling BuyNo action with MarshalInto: %v\n", err)
		return nil
	}
	return packer.Bytes()
}

// Unmarshal is a helper function to deserialize bytes into a BuyNo action.
// The codecVersion parameter is currently ignored as LinearCodec.UnmarshalFrom does not use it.
func (b *BuyNo) Unmarshal(d []byte, _ uint8) error {
	packer := codec.NewReader(d, userConsts.MaxActionSize)
	return codec.LinearCodec.UnmarshalFrom(packer.Packer, b)
}
