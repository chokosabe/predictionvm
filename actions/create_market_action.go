package actions

import (
	"github.com/chokosabe/predictionvm/escrow"
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database" // For database.ErrNotFound
	"github.com/ava-labs/avalanchego/ids"
	// "github.com/ava-labs/avalanchego/x/merkledb" // No longer directly needed
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"

	"github.com/chokosabe/predictionvm/asset"
	pvmconsts "github.com/chokosabe/predictionvm/consts" // Aliased
)

const (
	defaultMaxSize                = uint64(1 << 18) // 256 KiB
	initialMarketBufferSize       = 256             // Estimate
	initialCreateMarketBufferSize = 512             // Estimate, Question can be long

	// CreateMarketComputeUnits reflects state reads (checking if market ID exists) and writes (new market, creator balance if fees apply)
	CreateMarketComputeUnits = 2000 // Placeholder
	MaxCreateMarketSize      = 1024 // Placeholder, depends on description length and oracle parameters
	MaxQuestionLength        = 256
	PrefixMarket             = 0x00
)

var (
	ErrUnmarshalEmptyCreateMarket      = errors.New("cannot unmarshal empty bytes as CreateMarket action")
	ErrQuestionTooLong                 = errors.New("question too long")
	ErrClosingTimeInPast               = errors.New("closing time must be in the future")
	ErrResolutionTimeBeforeClosingTime = errors.New("resolution time must be after closing time")
	ErrInvalidCollateralAssetID        = errors.New("invalid collateral asset ID")
	ErrInvalidOracleAddress            = errors.New("invalid oracle address")
	ErrMarketExists                    = errors.New("market already exists")
)

// GetMarketKey generates the state key for a given market ID.
func GetMarketKey(marketID ids.ID) []byte {
	return append([]byte{PrefixMarket}, marketID[:]...)
}

// MarketStatus defines the state of a prediction market.
type MarketStatus byte

const (
	MarketStatusOpen MarketStatus = iota
	MarketStatusClosed
	MarketStatusResolved
)

// Market represents a prediction market.
type Market struct {
	ID                ids.ID        `serialize:"true" json:"id"`                 // Unique identifier for the market (derived from actionID of CreateMarket)
	Question          string        `serialize:"true" json:"question"`
	ClosingTime       int64         `serialize:"true" json:"closingTime"`
	ResolutionTime    int64         `serialize:"true" json:"resolutionTime"`
	CollateralAssetID ids.ID        `serialize:"true" json:"collateralAssetID"`
	OracleAddress     codec.Address `serialize:"true" json:"oracleAddress"`
	YesAssetID        ids.ID        `serialize:"true" json:"yesAssetID"`
	NoAssetID         ids.ID        `serialize:"true" json:"noAssetID"`
	Status            MarketStatus  `serialize:"true" json:"status"`
}

// MarshalCodec serializes the Market into a Packer.
func (m *Market) MarshalCodec(p *codec.Packer) error {
	p.PackID(m.ID)
	p.PackString(m.Question)
	p.PackInt64(m.ClosingTime)
	p.PackInt64(m.ResolutionTime)
	p.PackID(m.CollateralAssetID)
	p.PackAddress(m.OracleAddress)
	p.PackID(m.YesAssetID)
	p.PackID(m.NoAssetID)
	p.PackByte(byte(m.Status))
	return p.Err()
}

// UnmarshalCodec deserializes bytes from a Packer into a Market.
func (m *Market) UnmarshalCodec(p *codec.Packer) error {
	p.UnpackID(true, &m.ID) // Assuming true for allowNilOrEmpty/checkEOF
	m.Question = p.UnpackString(true) // Assuming true for checkEOF
	m.ClosingTime = p.UnpackInt64(true) // Assuming true for checkEOF
	m.ResolutionTime = p.UnpackInt64(true) // Assuming true for checkEOF
	p.UnpackID(true, &m.CollateralAssetID)
	p.UnpackAddress(&m.OracleAddress)
	p.UnpackID(true, &m.YesAssetID)
	p.UnpackID(true, &m.NoAssetID)
	statusByte := p.UnpackByte() // Corrected: UnpackByte takes no arguments
	m.Status = MarketStatus(statusByte)
	return p.Err()
}

// Bytes serializes the Market.
func (m *Market) Bytes() []byte {
	packer := codec.NewWriter(initialMarketBufferSize, int(defaultMaxSize))
	if err := m.MarshalCodec(packer); err != nil {
		panic(fmt.Errorf("failed to marshal Market: %w", err))
	}
	return packer.Bytes()
}

// UnmarshalMarket deserializes bytes into a Market.
func UnmarshalMarket(b []byte, m *Market) error {
	unpacker := codec.NewReader(b, int(defaultMaxSize))
	if err := m.UnmarshalCodec(unpacker); err != nil {
		return fmt.Errorf("failed to unmarshal Market: %w", err)
	}
	return nil
}

var _ chain.Action = (*CreateMarket)(nil)

// CreateMarket represents an action to create a new prediction market.
// Aligned with Spec 3.1 for Market definition inputs
type CreateMarket struct {
	Question          string        `serialize:"true" json:"question"`           // Renamed from Description
	CollateralAssetID ids.ID        `serialize:"true" json:"collateralAssetID"`  // Added
	InitialLiquidity  uint64        `serialize:"true" json:"initialLiquidity"` // Added
	ClosingTime       int64         `serialize:"true" json:"closingTime"`        // Renamed from EndTime
	ResolutionTime    int64         `serialize:"true" json:"resolutionTime"`
	OracleAddr        codec.Address `serialize:"true" json:"oracleAddr"`         // Added, replaces OracleType/Source/Parameters
}

// MarshalCodec serializes the CreateMarket into a Packer.
func (cm *CreateMarket) MarshalCodec(p *codec.Packer) error {
	p.PackString(cm.Question)
	p.PackID(cm.CollateralAssetID)
	p.PackUint64(cm.InitialLiquidity)
	p.PackInt64(cm.ClosingTime)
	p.PackInt64(cm.ResolutionTime)
	p.PackAddress(cm.OracleAddr)
	return p.Err()
}

// UnmarshalCodec deserializes bytes from a Packer into a CreateMarket.
func (cm *CreateMarket) UnmarshalCodec(p *codec.Packer) error {
	cm.Question = p.UnpackString(true) // Assuming true for checkEOF
	p.UnpackID(true, &cm.CollateralAssetID) // Assuming true for allowNilOrEmpty/checkEOF
	cm.InitialLiquidity = p.UnpackUint64(true) // Assuming true for checkEOF
	cm.ClosingTime = p.UnpackInt64(true) // Assuming true for checkEOF
	cm.ResolutionTime = p.UnpackInt64(true) // Assuming true for checkEOF
	p.UnpackAddress(&cm.OracleAddr)
	return p.Err()
}

func (*CreateMarket) GetTypeID() uint8 {
	return pvmconsts.CreateMarketID // Assuming CreateMarketID is defined in consts package
}

// Bytes serializes the CreateMarket action.
func (cm *CreateMarket) Bytes() []byte {
	packer := codec.NewWriter(initialCreateMarketBufferSize, int(defaultMaxSize))
	packer.PackByte(pvmconsts.CreateMarketID) // Prefix with TypeID
	if err := cm.MarshalCodec(packer); err != nil {
		panic(fmt.Errorf("failed to marshal CreateMarket action: %w", err))
	}
	return packer.Bytes()
}

// UnmarshalCreateMarket deserializes bytes into a CreateMarket action.
func UnmarshalCreateMarket(data []byte) (chain.Action, error) {
	if len(data) == 0 {
		return nil, ErrUnmarshalEmptyCreateMarket
	}
	unpacker := codec.NewReader(data, int(defaultMaxSize))
	actionType := unpacker.UnpackByte()

	if unpacker.Err() != nil { // Check for errors after UnpackByte, like EOF. Call Err() as a method.
		return nil, fmt.Errorf("failed to unpack CreateMarket typeID: %w", unpacker.Err())
	}
	if actionType != pvmconsts.CreateMarketID {
		return nil, fmt.Errorf("unexpected CreateMarket typeID: %d != %d", actionType, pvmconsts.CreateMarketID)
	}

	t := &CreateMarket{}
	if err := t.UnmarshalCodec(unpacker); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CreateMarket action: %w", err)
	}
	return t, nil
}

// ComputeUnits returns the compute units this action consumes.
func (cm *CreateMarket) ComputeUnits(rules chain.Rules) uint64 {
	return CreateMarketComputeUnits
}

// StateKeys defines which state keys are read/written by this action.
func (cm *CreateMarket) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	marketKeyBytes := GetMarketKey(actionID)
	marketKeyString := string(marketKeyBytes) // state.Keys is map[string]Permissions
	return state.Keys{
		marketKeyString: state.Write, // Market key is read (to check existence) and then written
		string(asset.NextAssetNonceKey): state.Write, // For DefineNewAsset
		// Asset definition keys are dynamic and written by DefineNewAsset
		string(escrow.GetEscrowKey(actionID, cm.CollateralAssetID)): state.Write, // For LockCollateral
		string(asset.GetBalanceKey(actor, cm.CollateralAssetID)):    state.Write, // For LockCollateral (actor's balance)
	}
}

// Execute performs the logic for the CreateMarket action.
func (cm *CreateMarket) Execute(
	ctx context.Context, // Standard Go context
	rules chain.Rules, // Chain rules, might be unused here
	mu state.Mutable, // Mutable state access
	timestamp int64, // Current block timestamp
	actor codec.Address, // Address of the actor performing the action, might be unused here
	actionID ids.ID, // This actionID becomes the Market.ID
) ([]byte, error) {
	_ = rules // Explicitly ignore if not used
	_ = actor // Explicitly ignore if not used

	marketKey := GetMarketKey(actionID)
	_, err := mu.GetValue(ctx, marketKey) // Check for existence using GetValue
	if err == nil {
		// Value exists, so market already exists
		return nil, ErrMarketExists
	} else if !errors.Is(err, database.ErrNotFound) { // Check for database.ErrNotFound
		// Another error occurred during GetValue
		return nil, fmt.Errorf("failed to check if market exists: %w", err)
	}
	// If err is merkledb.ErrNotFound, market does not exist, proceed.

	// Validations
	if len(cm.Question) == 0 || len(cm.Question) > MaxQuestionLength {
		return nil, ErrQuestionTooLong
	}
	if cm.ClosingTime <= timestamp {
		return nil, ErrClosingTimeInPast
	}
	if cm.ResolutionTime <= cm.ClosingTime {
		return nil, ErrResolutionTimeBeforeClosingTime
	}
	if cm.CollateralAssetID == ids.Empty {
		return nil, ErrInvalidCollateralAssetID
	}
	if cm.OracleAddr == codec.EmptyAddress {
		return nil, ErrInvalidOracleAddress
	}

	// Define YES and NO assets for the market
	yesAssetName := cm.Question + " - YES"
	yesAssetSymbol := fmt.Sprintf("M%.8sY", actionID.String())
	yesAssetMetadata := []byte("YES share for market " + actionID.String())
	yesAssetID, err := asset.DefineNewAsset(ctx, mu, actor, yesAssetName, yesAssetSymbol, yesAssetMetadata, uint64(timestamp))
	if err != nil {
		return nil, fmt.Errorf("failed to define YES asset: %w", err)
	}

	noAssetName := cm.Question + " - NO"
	noAssetSymbol := fmt.Sprintf("M%.8sN", actionID.String())
	noAssetMetadata := []byte("NO share for market " + actionID.String())
	noAssetID, err := asset.DefineNewAsset(ctx, mu, actor, noAssetName, noAssetSymbol, noAssetMetadata, uint64(timestamp))
	if err != nil {
		return nil, fmt.Errorf("failed to define NO asset: %w", err)
	}

	market := &Market{ // Using the Market struct defined in this file
		ID:                actionID,
		Question:          cm.Question,
		ClosingTime:       cm.ClosingTime,
		ResolutionTime:    cm.ResolutionTime,
		CollateralAssetID: cm.CollateralAssetID,
		OracleAddress:     cm.OracleAddr,
		YesAssetID:        yesAssetID,
		NoAssetID:         noAssetID,
		Status:            MarketStatusOpen,
	}

	marketBytes := market.Bytes()
	if err := mu.Insert(ctx, marketKey, marketBytes); err != nil { // Use Insert as per hypersdk v0.0.18
		return nil, fmt.Errorf("failed to save market %s: %w", market.ID, err)
	}

	// Lock initial liquidity from the creator into escrow
	if err := escrow.LockCollateral(ctx, mu, market.ID, actor, cm.CollateralAssetID, cm.InitialLiquidity); err != nil {
		// Attempt to remove the market if locking collateral fails. This is a best-effort cleanup.
		// A more robust solution might involve transactional atomicity if the underlying state supports it.
		if removeErr := mu.Remove(ctx, marketKey); removeErr != nil {
			return nil, fmt.Errorf("failed to lock initial liquidity for market %s (and failed to remove market): %w (remove error: %v)", market.ID, err, removeErr)
		}
		return nil, fmt.Errorf("failed to lock initial liquidity for market %s (market removed): %w", market.ID, err)
	}

	return actionID[:], nil // Return actionID as success result ID (actionID) as a byte slice
}

// ValidRange defines the time range during which the action is valid.
// It returns (0,0) to indicate that the action is generally valid at any time.
// Specific timing validations (e.g., market closing time in future) are handled in Execute.
func (*CreateMarket) ValidRange(rules chain.Rules) (start int64, end int64) {
	return 0, 0
}
