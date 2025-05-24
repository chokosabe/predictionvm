package actions

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	// "github.com/ava-labs/hypersdk/utils" // Removed as RandomID is not in this hypersdk version
	"crypto/rand"
	"encoding/binary"

	"github.com/chokosabe/predictionvm/consts"
	"github.com/chokosabe/predictionvm/storage"
)

const (
	// CreateMarketComputeUnits reflects state reads (checking if market ID exists) and writes (new market, creator balance if fees apply)
	CreateMarketComputeUnits = 2000 // Placeholder
	MaxCreateMarketSize      = 1024 // Placeholder, depends on description length and oracle parameters
)

var (
	ErrUnmarshalEmptyCreateMarket  = errors.New("cannot unmarshal empty bytes as CreateMarket action")
	ErrDescriptionTooLong          = errors.New("market description is too long")
	ErrOracleSourceTooLong         = errors.New("oracle source is too long")
	ErrOracleParametersTooLong     = errors.New("oracle parameters are too long")
	ErrEndTimeInPast               = errors.New("market end time is in the past")
	ErrResolutionTimeBeforeEndTime = errors.New("market resolution time is before or at end time")
	_                              chain.Action = (*CreateMarket)(nil)
)

// CreateMarket represents an action to create a new prediction market.
type CreateMarket struct {
	Description      string `serialize:"true" json:"description"`
	EndTime          int64  `serialize:"true" json:"endTime"`
	ResolutionTime   int64  `serialize:"true" json:"resolutionTime"`
	OracleType       uint8  `serialize:"true" json:"oracleType"`
	OracleSource     string `serialize:"true" json:"oracleSource"`
	OracleParameters []byte `serialize:"true" json:"oracleParameters"`
}

func (*CreateMarket) GetTypeID() uint8 {
	return consts.CreateMarketID // Assuming CreateMarketID is defined in consts package
}

// Bytes serializes the CreateMarket action.
func (cm *CreateMarket) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, MaxCreateMarketSize),
		MaxSize: MaxCreateMarketSize,
	}
	p.PackByte(consts.CreateMarketID)
	if err := codec.LinearCodec.MarshalInto(cm, p); err != nil {
		panic(fmt.Errorf("failed to marshal CreateMarket action: %w", err))
	}
	return p.Bytes
}

// UnmarshalCreateMarket deserializes bytes into a CreateMarket action.
func UnmarshalCreateMarket(bytes []byte) (chain.Action, error) {
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptyCreateMarket
	}
	if bytes[0] != consts.CreateMarketID {
		return nil, fmt.Errorf("unexpected CreateMarket typeID: %d != %d", bytes[0], consts.CreateMarketID)
	}
	t := &CreateMarket{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		t,
	); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CreateMarket action: %w", err)
	}
	return t, nil
}

// StateKeys defines which state keys are read/written by this action.
func (cm *CreateMarket) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	// We will write to a new market key (ID generated in Execute) and potentially actor's balance for fees.
	return state.Keys{
		string(storage.StateKeysBalance(actor)): state.Write, // Assuming a potential fee
		// A generic prefix indicating a write to the market space.
		// The exact key isn't known until Execute.
		string([]byte{storage.MarketPrefix}): state.Write,
	}
}

// Execute performs the logic for the CreateMarket action.
func (cm *CreateMarket) Execute(
	ctx context.Context,
	rules chain.Rules,
	mu state.Mutable,
	timestamp int64, // Current block timestamp
	actor codec.Address,
	actionID ids.ID,
) ([]byte, error) {
	// Validate input
	if len(cm.Description) == 0 {
		return nil, errors.New("market description cannot be empty")
	}
	if len(cm.Description) > 256 { // Example max length
		return nil, ErrDescriptionTooLong
	}
	if len(cm.OracleSource) > 256 { // Example max length
		return nil, ErrOracleSourceTooLong
	}
	if len(cm.OracleParameters) > 512 { // Example max length
		return nil, ErrOracleParametersTooLong
	}
	if cm.EndTime <= timestamp {
		return nil, ErrEndTimeInPast
	}
	if cm.ResolutionTime <= cm.EndTime {
		return nil, ErrResolutionTimeBeforeEndTime
	}

	// TODO: Deduct market creation fee from actor if applicable
	// fee := rules.GetCreateMarketFee() // Assuming such a method exists on rules
	// if err := storage.DeductBalance(ctx, mu, actor, fee); err != nil {
	// 	return nil, fmt.Errorf("failed to deduct creation fee: %w", err)
	// }

	randomBytes := make([]byte, ids.IDLen)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random bytes for market ID: %w", err)
	}
	marketID, err := ids.ToID(randomBytes)
	if err != nil {
		// This should ideally not happen if randomBytes is correct length and not all zeros
		return nil, fmt.Errorf("failed to convert random bytes to market ID: %w", err)
	}
	
	// Convert ids.ID to uint64 for storage.Market.ID
	// Ensure this conversion is appropriate for your ID scheme.
	// If storage.Market.ID should be ids.ID, then this conversion is not needed
	// and storage.MarketKey, etc., would need to handle ids.ID.
	// For now, assuming storage.Market.ID is uint64.
	// We'll use the first 8 bytes of the ids.ID for the uint64 market ID.
	marketIDUint64 := binary.BigEndian.Uint64(marketID[:8])


	market := &storage.Market{
		ID:               marketIDUint64,
		Description:      cm.Description,
		Creator:          actor,
		EndTime:          cm.EndTime,
		ResolutionTime:   cm.ResolutionTime,
		Status:           storage.MarketStatus_Open,
		TotalYesShares:   0,
		TotalNoShares:    0,
		OracleType:       cm.OracleType,
		OracleSource:     cm.OracleSource,
		OracleParameters: cm.OracleParameters,
		ResolvedOutcome:  storage.Outcome_Pending,
	}

	if err := storage.SetMarket(ctx, mu, market); err != nil {
		return nil, fmt.Errorf("failed to set new market %d: %w", market.ID, err)
	}

	resultMsg := fmt.Sprintf("Market %d created successfully by %s", market.ID, actor.String())
	return []byte(resultMsg), nil
}

// ComputeUnits estimates the computational cost of the CreateMarket action.
func (cm *CreateMarket) ComputeUnits(rules chain.Rules) uint64 {
	// Example: baseUnits + cost per byte of description and oracle params
	// baseUnits := rules.GetBaseUnits(consts.CreateMarketID)
	// units += rules.GetCostPerByte(uint64(len(cm.Description)))
	// units += rules.GetCostPerByte(uint64(len(cm.OracleSource)))
	// units += rules.GetCostPerByte(uint64(len(cm.OracleParameters)))
	return CreateMarketComputeUnits // Placeholder
}

// ValidRange defines the time range during which the action is valid.
func (*CreateMarket) ValidRange(rules chain.Rules) (start int64, end int64) {
	return -1, -1 // Always valid unless specific rules apply
}
