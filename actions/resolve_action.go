package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/chokosabe/predictionvm/storage"
)

const (
	// MaxResolveResultSize is the maximum size of a ResolveResult in bytes.
	// MarketID (32) + Resolution (1 byte) + TypeID (1 byte) = 34 bytes.
	// Rounded up to 64 for safety and future additions.
	MaxResolveResultSize = 64
)

var (
	ErrInvalidResolutionOutcome = errors.New("invalid resolution outcome")
	ErrUnauthorized             = errors.New("unauthorized")
	// ErrMarketNotFound and ErrMarketInteraction are defined in a shared errors file (e.g., actions/errors.go)
)

// Resolve allows an authorized actor (e.g., oracle) to set the outcome of a market.
type Resolve struct {
	// MarketID is the market to resolve.
	MarketID ids.ID `json:"marketId"`

	// Outcome is the resolution of the market.
	Outcome storage.OutcomeType `json:"outcome"`
}

// GetTypeID implements the codec.Typed interface.
func (r *Resolve) GetTypeID() uint8 {
	return ResolveType
}

// Execute implements the chain.Action interface.
func (r *Resolve) Execute(ctx context.Context, rules chain.Rules, st state.Mutable, blockTime int64, actor codec.Address, txID ids.ID) ([]byte, error) {
	// 1. Validate input resolution
	if r.Outcome != storage.Outcome_Yes && r.Outcome != storage.Outcome_No && r.Outcome != storage.Outcome_Invalid {
		return nil, fmt.Errorf("%w: invalid resolution outcome %d", ErrInvalidResolutionOutcome, r.Outcome)
	}

	// 2. Fetch the market
	market, err := storage.GetMarket(ctx, st, r.MarketID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, fmt.Errorf("%w: market %s not found", ErrMarketNotFound, r.MarketID.String())
		}
		return nil, fmt.Errorf("failed to get market %s: %w", r.MarketID.String(), err)
	}

	// 3. Validate market state
	if market.Status == storage.MarketStatus_Resolved {
		return nil, fmt.Errorf("%w: market %s is already resolved", ErrMarketInteraction, r.MarketID.String())
	}
	// Optional: Add check for MarketStatus_Locked if that's a distinct state before resolution.

	if blockTime <= market.ClosingTime {
		return nil, fmt.Errorf("%w: market %s cannot be resolved before its closing time (current: %d, closing: %d)", ErrMarketInteraction, r.MarketID.String(), blockTime, market.ClosingTime)
	}

	// 4. Validate actor (must be the oracle)
	if actor != market.OracleAddr {
		return nil, fmt.Errorf("%w: actor %s is not the designated oracle %s for market %s", ErrUnauthorized, actor.String(), market.OracleAddr.String(), r.MarketID.String())
	}

	// 5. Update market fields
	market.Status = storage.MarketStatus_Resolved
	market.ResolvedOutcome = r.Outcome
	market.ResolutionTime = blockTime // Record the actual time of resolution

	// 6. Save the updated market to state
	if err := storage.SetMarket(ctx, st, market); err != nil {
		return nil, fmt.Errorf("failed to save resolved market %s: %w", r.MarketID.String(), err)
	}

	// 7. Create and marshal the result
	result := &ResolveResult{
		MarketID:   r.MarketID,
		Resolution: r.Outcome,
	}

	packer := codec.NewWriter(MaxResolveResultSize, MaxResolveResultSize)
	packer.PackByte(result.GetTypeID()) // Prepend the TypeID
	result.MarshalCodec(packer)
	if packer.Err() != nil {
		return nil, fmt.Errorf("failed to marshal ResolveResult for market %s: %w", r.MarketID.String(), packer.Err())
	}

	return packer.Bytes(), nil
}

// ComputeUnits implements the chain.Action interface.
func (r *Resolve) ComputeUnits(rules chain.Rules) uint64 {
	return ResolveComputeUnits // Placeholder, define in costs.go
}

// StateKeys implements the chain.Action interface.
func (r *Resolve) StateKeys(actor codec.Address, txID ids.ID) state.Keys {
	// TODO: Define actual state keys when Execute logic is implemented.
	return state.Keys{}
}

// ValidRange implements the chain.Action interface.
func (r *Resolve) ValidRange(rules chain.Rules) (int64, int64) {
	// This action is valid at any time.
	// Specific timing logic (e.g., after market end time) should be in Execute.
	return 0, -1
}

// Auth implements the chain.Action interface.
func (r *Resolve) Auth() chain.Auth {
	// TODO: Consider more specific auth, e.g., only market creator or designated oracle.
	return &auth.ED25519{}
}

// Bytes implements codec.Marshaller
func (r *Resolve) Bytes() []byte {
	p := codec.NewWriter(ids.IDLen+1, ids.IDLen+1) 	// MarketID (ids.IDLen) + Outcome (1 byte)
	p.PackID(r.MarketID)
	p.PackByte(uint8(r.Outcome)) // Outcome is storage.OutcomeType (uint8)
	return p.Bytes()
}

// MaxPossibleSize implements the chain.Action interface and is used for pre-allocation.
func (r *Resolve) MaxPossibleSize() int {
	return ids.IDLen + 1 // MarketID (ids.IDLen) + Outcome (1 byte)
}

// UnmarshalJSON implements json.Unmarshaler.
// This is kept for potential CLI or direct JSON interaction, but the core VM parsing will use UnmarshalResolve.
func (r *Resolve) UnmarshalJSON(b []byte) error {
	var jsonData struct {
		MarketID ids.ID                `json:"marketId"`
		Outcome  storage.OutcomeType   `json:"outcome"`
	}
	if err := json.Unmarshal(b, &jsonData); err != nil {
		return err
	}
	r.MarketID = jsonData.MarketID
	r.Outcome = jsonData.Outcome
	return nil
}

// Unmarshal is the method used to deserialize bytes into a Resolve action.
// The codecVersion (cv) is currently ignored.
func (r *Resolve) Unmarshal(b []byte, cv uint8) error {
	reader := codec.NewReader(b, r.MaxPossibleSize())
	// codec.LinearCodec will reflectively unpack into the fields of 'r'.
	// Ensure fields in Resolve struct are exportable and correctly tagged if necessary.
	return codec.LinearCodec.UnmarshalFrom(reader.Packer, r)
}

// UnmarshalResolve is the unmarshaler function for Resolve actions,
// suitable for registration with codec.TypeParser.
func UnmarshalResolve(b []byte) (chain.Action, error) {
	action := &Resolve{}
	// The codecVersion (cv) is not used by the Unmarshal method here,
	// so we pass a zero value (or any uint8) for it.
	if err := action.Unmarshal(b, 0); err != nil {
		return nil, err
	}
	return action, nil
}
