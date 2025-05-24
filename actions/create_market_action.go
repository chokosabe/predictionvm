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
	ErrUnmarshalEmptyCreateMarket      = errors.New("cannot unmarshal empty bytes as CreateMarket action")
	ErrQuestionTooLong                 = errors.New("market question is too long") // Renamed
	ErrClosingTimeInPast               = errors.New("market closing time is in the past") // Renamed
	ErrResolutionTimeBeforeClosingTime = errors.New("market resolution time is before or at closing time") // Renamed
	ErrInvalidCollateralAssetID        = errors.New("collateral asset ID is invalid") // Added
	ErrInvalidOracleAddress            = errors.New("oracle address is invalid")    // Added
	_                                  chain.Action = (*CreateMarket)(nil)
)

// CreateMarket represents an action to create a new prediction market.
// Aligned with Spec 3.1 for Market definition inputs
type CreateMarket struct {
	Question          string        `serialize:"true" json:"question"`           // Renamed from Description
	CollateralAssetID ids.ID        `serialize:"true" json:"collateralAssetID"`  // Added
	ClosingTime       int64         `serialize:"true" json:"closingTime"`        // Renamed from EndTime
	ResolutionTime    int64         `serialize:"true" json:"resolutionTime"`
	OracleAddr        codec.Address `serialize:"true" json:"oracleAddr"`         // Added, replaces OracleType/Source/Parameters
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
	if len(cm.Question) == 0 {
		return nil, errors.New("market question cannot be empty")
	}
	if len(cm.Question) > 256 { // Example max length for Question
		return nil, ErrQuestionTooLong
	}
	if cm.CollateralAssetID == ids.Empty {
		return nil, ErrInvalidCollateralAssetID
	}
	if len(cm.OracleAddr) == 0 { // Basic check for empty OracleAddr
		return nil, ErrInvalidOracleAddress
	}
	if cm.ClosingTime <= timestamp {
		return nil, ErrClosingTimeInPast
	}
	if cm.ResolutionTime <= cm.ClosingTime {
		return nil, ErrResolutionTimeBeforeClosingTime
	}

	// TODO: Deduct market creation fee from actor if applicable
	// fee := rules.GetCreateMarketFee() // Assuming such a method exists on rules
	// if err := storage.DeductBalance(ctx, mu, actor, fee); err != nil {
	// 	return nil, fmt.Errorf("failed to deduct creation fee: %w", err)
	// }

	// Generate a unique ID for the market itself (pvm stands for predictionvm market)
	randomBytesPvmID := make([]byte, ids.IDLen)
	_, err := rand.Read(randomBytesPvmID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random bytes for market PVM ID: %w", err)
	}
	marketPvmID, err := ids.ToID(randomBytesPvmID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert random bytes to market PVM ID: %w", err)
	}
	marketIDUint64 := binary.BigEndian.Uint64(marketPvmID[:8]) // Used for storage.Market.ID (uint64)

	// Generate Yes and No Asset IDs for the market's shares
	yesAssetRandomBytes := make([]byte, ids.IDLen)
	if _, err := rand.Read(yesAssetRandomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes for yes asset ID: %w", err)
	}
	yesAssetID, err := ids.ToID(yesAssetRandomBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to convert random bytes to yes asset ID: %w", err)
	}

	noAssetRandomBytes := make([]byte, ids.IDLen)
	if _, err := rand.Read(noAssetRandomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes for no asset ID: %w", err)
	}
	noAssetID, err := ids.ToID(noAssetRandomBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to convert random bytes to no asset ID: %w", err)
	}

	market := &storage.Market{
		ID:                marketIDUint64,
		Question:          cm.Question,
		CollateralAssetID: cm.CollateralAssetID,
		ClosingTime:       cm.ClosingTime,
		OracleAddr:        cm.OracleAddr,
		Status:            storage.MarketStatus_Open,
		ResolvedOutcome:   storage.Outcome_Pending,
		YesAssetID:        yesAssetID, // Generated
		NoAssetID:         noAssetID,  // Generated
		Creator:           actor,
		ResolutionTime:    cm.ResolutionTime,
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
