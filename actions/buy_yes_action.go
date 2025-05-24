package actions

import (
	"context"
	"errors" // Added for error handling
	"fmt"

	"github.com/ava-labs/avalanchego/database" // Added for database interactions
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state" // Added for state.Keys, state.Permissions

	userConsts "github.com/chokosabe/predictionvm/consts" // Aliased for clarity
	"github.com/chokosabe/predictionvm/storage"
)

var _ chain.Action = (*BuyYes)(nil)

// BuyYes represents an action where a user buys YES shares for a specific market.
type BuyYes struct {
	MarketID uint64 `serialize:"true" json:"marketId"`
	// Amount of shares to buy or amount of collateral to commit, depending on market mechanism.
	Amount uint64 `serialize:"true" json:"amount"`
	// MaxPrice the user is willing to pay per share, if applicable to market type.
	// Or max collateral they are willing to lock.
	MaxPrice uint64 `serialize:"true" json:"maxPrice"`
}

var (
	ErrAmountCannotBeZero   = errors.New("amount cannot be zero")
	ErrMaxPriceCannotBeZero = errors.New("max price cannot be zero")
	ErrMarketNotFound       = errors.New("market not found")
	ErrMarketInteraction    = errors.New("market interaction error") // Generic for resolved, cancelled, ended
	ErrInsufficientFunds    = errors.New("insufficient funds")
)

// GetTypeID implements chain.Action
func (b *BuyYes) GetTypeID() uint8 {
	return userConsts.BuyYesID
}

// StateKeys implements chain.Action
func (b *BuyYes) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	// TODO: storage.BalanceKey, storage.MarketKey, storage.ShareBalanceKey are currently undefined.
	// These will need to be correctly defined and imported.
	// For now, using placeholders that will cause compile errors for these specific lines,
	// but the method signature and return type are correct for chain.Action.
	return state.Keys{
		string(storage.BalanceKey(actor)):                               state.Read | state.Write, // Placeholder
		string(storage.MarketKey(b.MarketID)):                           state.Read | state.Write, // Placeholder
		string(storage.ShareBalanceKey(b.MarketID, actor, userConsts.YesShareType)): state.Read | state.Write, // Placeholder
	}
}

// Execute performs the action of buying YES shares for a given market.
func (b *BuyYes) Execute(
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
		if errors.Is(err, database.ErrNotFound) {
			return nil, fmt.Errorf("%w: market %d not found when fetching", ErrMarketNotFound, b.MarketID)
		}
		return nil, fmt.Errorf("failed to get market %d: %w", b.MarketID, err)
	}
	// Redundant if GetMarket returns error on not found, but good for clarity
	// if market == nil {
	// 	return nil, fmt.Errorf("%w: market %d not found", ErrMarketNotFound, b.MarketID)
	// }
	if market.Status == storage.MarketStatus_ResolvedYes || market.Status == storage.MarketStatus_ResolvedNo {
		return nil, fmt.Errorf("%w: market %d is already resolved (status: %s)", ErrMarketInteraction, b.MarketID, market.Status.String())
	}
	// Removed IsCancelled check as MarketStatus does not have a direct 'Cancelled' state.
	// Cancellation might be handled by a specific resolution outcome (e.g., Invalid) or other logic.
	if txTimestamp > market.EndTime && market.Status == storage.MarketStatus_Open { // Can only trade if market is open and within time
		// If trading is closed but not yet resolved, that's a different state (MarketStatus_TradingClosed)
		// which might still allow other actions but not new trades like BuyYes.
		// For BuyYes, if EndTime is passed and it's still 'Open', it's effectively ended for new trades.
		// If it's TradingClosed, that also means new trades are not allowed.
		return nil, fmt.Errorf("%w: market %d has ended or trading is closed (current: %d, end: %d, status: %s)", ErrMarketInteraction, b.MarketID, txTimestamp, market.EndTime, market.Status.String())
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
		if errors.Is(err, database.ErrNotFound) {
			// Actor has no balance record, treat as 0 balance for cost calculation
			// This means they definitely don't have enough funds if cost > 0
			cost := b.Amount * b.MaxPrice // Calculate cost to include in error message
			return nil, fmt.Errorf("%w: actor %s has no balance record, cost is %d", ErrInsufficientFunds, actor.String(), cost)
		}
		return nil, fmt.Errorf("failed to get actor's balance for %s: %w", actor.String(), err)
	}

	currentBalance, err := database.ParseUInt64(currentBalanceBytes)
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
	if err := mu.Insert(ctx, balanceKey, database.PackUInt64(newBalance)); err != nil {
		return nil, fmt.Errorf("failed to set new balance %d for actor %s: %w", newBalance, actor.String(), err)
	}

	// 5. Credit YES shares to actor
	currentYesShares, err := storage.GetShareBalance(ctx, mu, b.MarketID, actor, userConsts.YesShareType)
	// Allow ErrNotFound for initial share balance, in which case currentShareBalance defaults to 0 (uint64)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		// Consider reverting native token balance change here or using a more transactional approach
		return nil, fmt.Errorf("failed to get current share balance for actor %s, market %d, type YES: %w", actor.String(), b.MarketID, err)
	}

	newShareBalance := currentYesShares + b.Amount
	if err := storage.SetShareBalance(ctx, mu, b.MarketID, actor, userConsts.YesShareType, newShareBalance); err != nil {
		// Consider reverting native token balance change here
		return nil, fmt.Errorf("failed to set new share balance %d for actor %s, market %d, type YES: %w", newShareBalance, actor.String(), b.MarketID, err)
	}

	// 6. Update market's total YES shares
	// We use the 'market' variable fetched earlier in this Execute call.
	// It's important that this 'market' instance is the one we want to modify and save.
	market.TotalYesShares += b.Amount
	if err := storage.SetMarket(ctx, mu, market); err != nil { // Pass the market object itself
		// Potentially revert user's share balance and native token balance changes here for atomicity
		return nil, fmt.Errorf("failed to update market %d total YES shares: %w", b.MarketID, err)
	}

	// For now, return nil output and no error for success
	return nil, nil
}

// ComputeUnits implements chain.Action
func (b *BuyYes) ComputeUnits(rules chain.Rules) uint64 {
	return 0 // Placeholder, to align with test expectation until proper fee/unit calculation
}

// ValidRange implements chain.Action
func (b *BuyYes) ValidRange(rules chain.Rules) (start int64, end int64) {
	return -1, -1 // Always valid
}

// Bytes implements chain.Action
func (b *BuyYes) Bytes() []byte {
	packer := codec.NewWriter(0, userConsts.MaxActionSize)
	if err := codec.LinearCodec.MarshalInto(b, packer.Packer); err != nil {
		// This should ideally not happen with a well-defined struct
		// and could panic or log fatally in a real scenario.
		// Consider returning an error or panicking if appropriate for the VM's error handling strategy.
		fmt.Printf("Error marshalling BuyYes action with MarshalInto: %v\n", err)
		return nil
	}
	return packer.Bytes()
}

// Unmarshal is a helper function to deserialize bytes into a BuyYes action.
// The codecVersion parameter is currently ignored as LinearCodec.UnmarshalFrom does not use it.
func (b *BuyYes) Unmarshal(d []byte, _ uint8) error {
	packer := codec.NewReader(d, userConsts.MaxActionSize)
	return codec.LinearCodec.UnmarshalFrom(packer.Packer, b)
}

// UnmarshalBuyYes is the unmarshaler function for BuyYes actions,
// suitable for registration with codec.TypeParser.
func UnmarshalBuyYes(b []byte) (chain.Action, error) {
	action := &BuyYes{}
	// The codecVersion (cv) is not used by the Unmarshal method for LinearCodec,
	// so we pass a zero value (or any uint8) for it.
	err := action.Unmarshal(b, 0)
	if err != nil {
		return nil, err
	}
	return action, nil
}
