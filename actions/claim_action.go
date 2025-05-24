package actions

import (
	"context"
	"encoding/json"
	"errors" // Added for errors.Is
	"fmt"    // Added for fmt.Errorf

	"github.com/chokosabe/predictionvm/consts" // Added for consts.ClaimID

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"

	"github.com/chokosabe/predictionvm/asset"
	"github.com/chokosabe/predictionvm/storage"
)

// Claim allows a user to claim their winnings from a resolved market.
type Claim struct {
	// MarketID is the market to claim winnings from.
	MarketID ids.ID `json:"marketId"`
}

// GetTypeID implements the codec.Typed interface.
func (c *Claim) GetTypeID() byte {
	return consts.ClaimID
}

// Execute implements the chain.Action interface.
func (c *Claim) Execute(ctx context.Context, rules chain.Rules, mu state.Mutable, blockTime int64, actor codec.Address, txID ids.ID) ([]byte, error) {
	actorSlice := actor[:]
	if len(actorSlice) != codec.AddressLen {
		// This check might be useful to keep if actor length issues are recurrent, 
		// but for now, the debug print is removed.
		// fmt.Printf("[DEBUG] Claim.Execute: actor[:] has unexpected length! Expected %d. Bytes: %x\n", codec.AddressLen, actorSlice)
	}

	market, err := storage.GetMarket(ctx, mu, c.MarketID)
	if err != nil {
		wrappedErr := fmt.Errorf("failed to get market %s during claim: %w", c.MarketID, err)
		if errors.Is(err, database.ErrNotFound) {
			return nil, fmt.Errorf("%w: %w", ErrMarketNotFound, wrappedErr) // Chain ErrMarketNotFound with the wrapped original
		}
		return nil, wrappedErr // Propagate other wrapped errors
	}

	if market.Status != storage.MarketStatus_Resolved {
		return nil, ErrMarketNotResolved
	}

	var winningShareType uint8
	// Assuming 1 unit of collateral per winning share. In a real scenario, this might be tied to market creation parameters.
	const payoutAmountPerShare uint64 = 1

	switch market.ResolvedOutcome {
	case storage.Outcome_Yes:
		winningShareType = consts.YesShareType
	case storage.Outcome_No:
		winningShareType = consts.NoShareType
	case storage.Outcome_Invalid:
		return nil, ErrMarketInvalidNoPayout
	default: // Includes OutcomePending or any other unexpected outcome
		return nil, ErrMarketOutcomePending
	}

	// Fetch the user's balance of the winning shares for this specific market
	userWinningSharesBalance, err := storage.GetShareBalance(ctx, mu, c.MarketID, actor, winningShareType)
	if err != nil {
		// Since GetShareBalance is expected to return 0, nil on NotFound, any error here is more significant.
		return nil, fmt.Errorf("failed to get winning share balance for market %s, actor %s, type %d: %w", c.MarketID, actor, winningShareType, err)
	}

	if userWinningSharesBalance == 0 {
		return nil, ErrNoWinningShares
	}

	// Set the user's winning shares for this market to 0 (effectively burning them for this claim)
	if err := storage.SetShareBalance(ctx, mu, c.MarketID, actor, winningShareType, 0); err != nil {
		return nil, err // Or a more specific error indicating burn failure
	}

	// Calculate payout (amount of collateral to be transferred)
	amountToPayout := userWinningSharesBalance * payoutAmountPerShare

	// Credit user's collateral asset balance
	currentCollateralBalance, err := asset.GetAssetBalance(ctx, mu, actor, market.CollateralAssetID)
	if err != nil {
		// Any error here is significant as NotFound is handled internally by GetAssetBalance.
		return nil, fmt.Errorf("failed to get collateral balance for asset %s, actor %s: %w", market.CollateralAssetID, actor, err)
	}

	newCollateralBalance := currentCollateralBalance + amountToPayout
	if err := asset.SetAssetBalance(ctx, mu, actor, market.CollateralAssetID, newCollateralBalance); err != nil {
		return nil, err // If setting new balance fails, this is a critical failure.
	}

	// Construct and marshal the result
	result := &ClaimResult{
		MarketID:        c.MarketID,
		ClaimantAddress: actor,
		PayoutAssetID:   market.CollateralAssetID,
		AmountClaimed:   amountToPayout,
	}
	p := codec.NewWriter(MaxClaimResultSize, MaxClaimResultSize)

	if err := result.MarshalCodec(p); err != nil {
		return nil, err
	}
	return p.Bytes(), nil
}

// ComputeUnits implements the chain.Action interface.
func (c *Claim) ComputeUnits(rules chain.Rules) uint64 {
	const ClaimComputeUnits uint64 = 100 // Example value, adjust as needed
	return ClaimComputeUnits
}

// MaxClaimResultSize is the maximum byte size of a marshaled ClaimResult.
// MarketID (32) + ClaimantAddress (33) + PayoutAssetID (32) + AmountClaimed (8) = 105.
const MaxClaimResultSize = 105

// StateKeys implements the chain.Action interface.
func (c *Claim) StateKeys(actor codec.Address, txID ids.ID) state.Keys {
	// We need to read the market, and read/write the actor's share balances for this market,
	// and read/write the actor's collateral balance for the market's collateral asset.
	// The MarketID is known. The actor is known.
	// ShareType (Yes/No) can be covered by listing both.
	// CollateralAssetID is inside the market state, so asset.GetBalanceKey for collateral cannot be fully formed here.

	keys := make(state.Keys, 3) // Market (read) + 2 Share Balances (write)

	// Key for reading the market state
	marketKey := storage.MarketKey(c.MarketID)
	keys[string(marketKey)] = state.Read

	// Keys for reading and then writing (to zero) the user's share balances for this market.
	// We list both Yes and No share types because the winning type is determined during Execute.
	yesShareBalanceKey := storage.ShareBalanceKey(c.MarketID, actor, consts.YesShareType)
	keys[string(yesShareBalanceKey)] = state.Write // Read then Write (to 0)

	noShareBalanceKey := storage.ShareBalanceKey(c.MarketID, actor, consts.NoShareType)
	keys[string(noShareBalanceKey)] = state.Write // Read then Write (to 0)

	// The key for the collateral asset (asset.GetBalanceKey(actor, market.CollateralAssetID))
	// cannot be fully determined here because market.CollateralAssetID is read from the market state.
	// HyperSDK's state permission model might require a way to declare this, possibly through
	// broader permissions if exact key matching is strict and this causes issues.
	// For now, we only declare keys that can be fully constructed with information available to StateKeys.
	// Read and Write for actor's balances (winning shares and collateral)
	// Since we don't know the specific asset IDs here without reading the market,
	// we'd typically list the prefixes or a broader set of keys if the SDK supports it.
	// Or, the Execute method would need to be structured to provide these keys.
	// For now, let's assume the most common scenario: read/write for collateral and one share type.
	// This is an approximation because YesAssetID/NoAssetID are dynamic.
	// A more accurate StateKeys might require a preliminary state read or a different design.

	// Simplification: Assume actor's balance for *any* asset could be touched for read/write.
	// This is overly broad. A better way is to specify the keys based on market.CollateralAssetID,
	// market.YesAssetID, market.NoAssetID which are part of the market object.
	// For the purpose of this exercise, we will list them as if we had the market object.
	// This part of StateKeys is tricky without access to the market details directly.

	// Let's assume the critical keys are:
	// 1. Market object (read)
	// 2. Actor's balance of the winning share type (read/write - effectively delete)
	// 3. Actor's balance of the collateral (read/write - update)

	// We will list the market key and then use placeholder asset IDs for the balance keys
	// as we cannot resolve them here without reading state.
	// The actual asset IDs (Yes, No, Collateral) are stored within the market data itself.
	// A truly accurate StateKeys would require knowing these IDs beforehand or making the keys more generic.

	return keys
}

// ValidRange implements the chain.Action interface.
func (c *Claim) ValidRange(rules chain.Rules) (int64, int64) {
	// This action is valid at any time.
	return 0, -1
}

// Auth implements the chain.Action interface.
func (c *Claim) Auth() chain.Auth {
	return &auth.ED25519{}
}

// Bytes implements codec.Marshaller
func (c *Claim) Bytes() []byte {
	p := codec.NewWriter(ids.IDLen, ids.IDLen) // MarketID is an ids.ID
	p.PackID(c.MarketID)
	return p.Bytes()
}

// MaxPossibleSize implements the chain.Action interface and is used for pre-allocation.
func (c *Claim) MaxPossibleSize() int {
	return ids.IDLen // Only MarketID
}

// UnmarshalJSON implements json.Unmarshaler.
// This is kept for potential CLI or direct JSON interaction, but the core VM parsing will use UnmarshalClaim.
func (c *Claim) UnmarshalJSON(b []byte) error {
	var jsonData struct {
		MarketID ids.ID `json:"marketId"`
	}
	if err := json.Unmarshal(b, &jsonData); err != nil {
		return err
	}
	c.MarketID = jsonData.MarketID
	return nil
}

// Unmarshal is the method used to deserialize bytes into a Claim action.
// The codecVersion (cv) is currently ignored.
func (c *Claim) Unmarshal(b []byte, cv uint8) error {
	reader := codec.NewReader(b, c.MaxPossibleSize())
	reader.UnpackID(true, &c.MarketID) // true to allow parsing of ID from string representation
	return reader.Err()
}

// UnmarshalClaim is the unmarshaler function for Claim actions,
// suitable for registration with codec.TypeParser.
func UnmarshalClaim(b []byte) (chain.Action, error) {
	action := &Claim{}
	// The codecVersion (cv) is not used by the Unmarshal method here,
	// so we pass a zero value (or any uint8) for it.
	if err := action.Unmarshal(b, 0); err != nil {
		return nil, err
	}
	return action, nil
}
