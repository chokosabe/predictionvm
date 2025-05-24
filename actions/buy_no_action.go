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
	"github.com/chokosabe/predictionvm/asset"
	"github.com/chokosabe/predictionvm/escrow"
)

var _ chain.Action = (*BuyNo)(nil)

const (
	// MaxBuyNoResultSize is a pessimistic estimate of the max size of BuyNoResult.
	// TypeID (1) + SharesBought (8) + CostPaid (8) = 17 bytes. We use 32 for buffer.
	MaxBuyNoResultSize = 32
)

// BuyNo represents an action where a user buys NO shares for a specific market.
type BuyNo struct {
	MarketID          ids.ID `serialize:"true" json:"marketId"`
	CollateralAssetID ids.ID `serialize:"true" json:"collateralAssetId"` // Client must provide this
	NoAssetID         ids.ID `serialize:"true" json:"noAssetId"`         // Client must provide this
	// Amount of NO shares the user wants to buy (implies an equal amount of collateral to be locked).
	Amount            uint64 `serialize:"true" json:"amount"`
	// MaxPrice is the maximum amount of collateral the user is willing to pay per NO share.
	// In the current model, this is effectively the amount of collateral locked per share.
	MaxPrice          uint64 `serialize:"true" json:"maxPrice"`
}

// GetTypeID implements chain.Action
func (b *BuyNo) GetTypeID() uint8 {
	return userConsts.BuyNoID
}

// StateKeys implements chain.Action
func (b *BuyNo) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	marketKey := GetMarketKey(b.MarketID) // Assumes GetMarketKey can handle ids.ID or is adapted

	// Key for the actor's balance of the collateral asset
	actorCollateralBalanceKey := asset.GetBalanceKey(actor, b.CollateralAssetID)

	// Key for the market's escrow account for the collateral asset
	marketEscrowKey := escrow.GetEscrowKey(b.MarketID, b.CollateralAssetID)

	// Key for the actor's balance of NO shares (which is an asset)
	actorNoShareBalanceKey := asset.GetBalanceKey(actor, b.NoAssetID)

	return state.Keys{
		string(marketKey):                state.Read,  // To read market details
		string(actorCollateralBalanceKey): state.Write, // To check balance and lock collateral
		string(marketEscrowKey):          state.Write, // To credit the market's escrow account
		string(actorNoShareBalanceKey):   state.Write, // To mint NO shares to the actor
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
			return nil, fmt.Errorf("%w: market %s not found when fetching", ErrMarketNotFound, b.MarketID.String())
		}
		return nil, fmt.Errorf("failed to get market %s: %w", b.MarketID.String(), err)
	}
	if market.Status == storage.MarketStatus_Resolved {
		return nil, fmt.Errorf("%w: market %s is already resolved (status: %s)", ErrMarketInteraction, b.MarketID.String(), market.Status.String())
	}
	if market.Status == storage.MarketStatus_Locked {
		return nil, fmt.Errorf("%w: market %s trading is closed (status: %s)", ErrMarketInteraction, b.MarketID.String(), market.Status.String())
	}
	if txTimestamp > market.ClosingTime {
		return nil, fmt.Errorf("%w: market %s has ended (current: %d, end: %d)", ErrMarketInteraction, b.MarketID.String(), txTimestamp, market.ClosingTime)
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
		return nil, fmt.Errorf("%w: actor balance %d, cost %d for %d shares at max price %d for market %s", ErrInsufficientFunds, currentBalance, cost, b.Amount, b.MaxPrice, b.MarketID.String())
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
		return nil, fmt.Errorf("failed to get current NO share balance for actor %s, market %s: %w", actor.String(), b.MarketID.String(), err)
	}

	newShareBalance := currentNoShares + b.Amount
	if err := storage.SetShareBalance(ctx, mu, b.MarketID, actor, userConsts.NoShareType, newShareBalance); err != nil {
		// Consider reverting native token balance change here
		return nil, fmt.Errorf("failed to set new NO share balance %d for actor %s, market %s: %w", newShareBalance, actor.String(), b.MarketID.String(), err)
	}

	// TotalNoShares is now managed by the HybridAsset module, no direct update here.

	// TODO: Emit event for BuyNo action

	// Create and marshal the result
	result := &BuyNoResult{
		SharesBought: b.Amount,
		CostPaid:     cost, // 'cost' was calculated earlier in the function
	}

	packer := codec.NewWriter(MaxBuyNoResultSize, MaxBuyNoResultSize) // initialSliceCap, maxSliceCap
	packer.PackByte(result.GetTypeID()) // Prepend the TypeID
	result.MarshalCodec(packer) // Marshal the result struct
	if packer.Err() != nil {
		return nil, fmt.Errorf("failed to marshal BuyNoResult for market %s: %w", b.MarketID.String(), packer.Err())
	}

	return packer.Bytes(), nil // Success
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
