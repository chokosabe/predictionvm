package storage

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids" // Added this import
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"

	pvmConsts "github.com/chokosabe/predictionvm/consts"
)

// MarketStatus defines the possible states of a prediction market.
type MarketStatus uint8

const (
	MarketStatus_Open          MarketStatus = 0 // Market is open for trading
	MarketStatus_TradingClosed MarketStatus = 1 // Trading is closed, awaiting resolution
	MarketStatus_ResolvedYes   MarketStatus = 2 // Market resolved as YES
	MarketStatus_ResolvedNo    MarketStatus = 3 // Market resolved as NO
)

func (ms MarketStatus) String() string {
	switch ms {
	case MarketStatus_Open:
		return "Open"
	case MarketStatus_TradingClosed:
		return "TradingClosed"
	case MarketStatus_ResolvedYes:
		return "ResolvedYes"
	case MarketStatus_ResolvedNo:
		return "ResolvedNo"
	default:
		return fmt.Sprintf("UnknownMarketStatus:%d", ms)
	}
}

// OutcomeType defines the possible resolved outcomes of a prediction market.
type OutcomeType uint8

const (
	Outcome_Pending OutcomeType = 0 // Market outcome is not yet determined
	Outcome_Yes     OutcomeType = 1 // Market resolved as YES
	Outcome_No      OutcomeType = 2 // Market resolved as NO
	Outcome_Invalid OutcomeType = 3 // Market resolved as Invalid (e.g., ambiguous question, event didn't occur)
)

func (ot OutcomeType) String() string {
	switch ot {
	case Outcome_Pending:
		return "Pending"
	case Outcome_Yes:
		return "Yes"
	case Outcome_No:
		return "No"
	case Outcome_Invalid:
		return "Invalid"
	default:
		return fmt.Sprintf("UnknownOutcomeType:%d", ot)
	}
}

// Market defines the structure for a prediction market.
// Aligned with Spec 3.1
// Key: market/{id} -> Market
/*
  id            uint64
  question      string
  collateral    assetID   // USDC or native token (represented by ids.ID)
  closingTime   uint64    // unix (represented by int64)
  oracleAddr    address   // BLS pubkey (represented by codec.Address)
  status        enum{OPEN, LOCKED, RESOLVED} (represented by MarketStatus)
  outcome       bool?     // nil until RESOLVED (represented by ResolvedOutcome OutcomeType)
  yesAssetID    assetID   // ERC404 YES token (represented by ids.ID)
  noAssetID     assetID   // ERC404 NO token (represented by ids.ID)
*/
type Market struct {
	ID                uint64        `serialize:"true" json:"id"`
	Question          string        `serialize:"true" json:"question"`                 // Renamed from Description
	CollateralAssetID ids.ID        `serialize:"true" json:"collateralAssetID"`      // Added
	ClosingTime       int64         `serialize:"true" json:"closingTime"`              // Spec says uint64, using int64 for consistency with other time fields
	OracleAddr        codec.Address `serialize:"true" json:"oracleAddr"`               // Added, replaces OracleType/Source/Parameters
	Status            MarketStatus  `serialize:"true" json:"status"`
	ResolvedOutcome   OutcomeType   `serialize:"true" json:"resolvedOutcome"`
	YesAssetID        ids.ID        `serialize:"true" json:"yesAssetID"`               // Added
	NoAssetID         ids.ID        `serialize:"true" json:"noAssetID"`                // Added

	// Fields from previous version, kept for now or to be reviewed against full spec integration
	Creator         codec.Address `serialize:"true" json:"creator"`
	ResolutionTime  int64         `serialize:"true" json:"resolutionTime"` // Spec doesn't explicitly list this, but useful
	TotalYesShares  uint64        `serialize:"true" json:"totalYesShares"`   // Will be derived from HybridAsset module later
	TotalNoShares   uint64        `serialize:"true" json:"totalNoShares"`    // Will be derived from HybridAsset module later
}

// MarketKey generates the state key for a given market ID.
// Format: MarketPrefix | MarketID (uint64)
func MarketKey(marketID uint64) []byte {
	key := make([]byte, 1+8) // Use literal 8 for Uint64Len
	key[0] = MarketPrefix
	binary.BigEndian.PutUint64(key[1:], marketID)
	return key
}

// GetMarket retrieves a market by its ID from the state.
func GetMarket(ctx context.Context, im state.Immutable, marketID uint64) (*Market, error) {
	key := MarketKey(marketID)
	valBytes, err := im.GetValue(ctx, key)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, fmt.Errorf("market %d not found: %w", marketID, err)
		}
		return nil, err
	}
	if len(valBytes) == 0 {
		// Should not happen if ErrNotFound is handled, but good practice
		return nil, fmt.Errorf("market %d found but value is empty", marketID)
	}

	reader := codec.NewReader(valBytes, pvmConsts.MaxMarketDataSize)
	market := &Market{}
	if err := codec.LinearCodec.UnmarshalFrom(reader.Packer, market); err != nil {
		return nil, fmt.Errorf("failed to unmarshal market %d using LinearCodec: %w", marketID, err)
	}
	if err := reader.Err(); err != nil {
		return nil, fmt.Errorf("reader error after unmarshaling market %d: %w", marketID, err)
	}

	return market, nil
}

// SetMarket stores a market into the state.
func SetMarket(ctx context.Context, mu state.Mutable, market *Market) error {
	key := MarketKey(market.ID)
	writer := codec.NewWriter(0, pvmConsts.MaxMarketDataSize)
	if err := codec.LinearCodec.MarshalInto(market, writer.Packer); err != nil {
		return fmt.Errorf("failed to marshal market %d using LinearCodec: %w", market.ID, err)
	}
	if err := writer.Err(); err != nil {
		return fmt.Errorf("writer error after marshaling market %d: %w", market.ID, err)
	}
	return mu.Insert(ctx, key, writer.Bytes())
}

// ShareBalanceKey generates the state key for a user's share balance in a market.
// Format: ShareBalancePrefix | MarketID (uint64) | UserAddress (codec.Address) | ShareType (uint8)
func ShareBalanceKey(marketID uint64, user codec.Address, shareType uint8) []byte {
	key := make([]byte, 1+8+codec.AddressLen+1) // Use literal 8 for Uint64Len and 1 for Uint8Len
	key[0] = ShareBalancePrefix
	offset := 1
	binary.BigEndian.PutUint64(key[offset:], marketID)
	offset += 8 // Use literal 8 for Uint64Len
	copy(key[offset:], user[:])
	offset += codec.AddressLen
	key[offset] = shareType
	return key
}

// GetShareBalance retrieves a user's share balance for a specific market and share type.
func GetShareBalance(ctx context.Context, im state.Immutable, marketID uint64, user codec.Address, shareType uint8) (uint64, error) {
	key := ShareBalanceKey(marketID, user, shareType)
	valBytes, err := im.GetValue(ctx, key) // Use GetValue and pass ctx
	if errors.Is(err, database.ErrNotFound) {
		return 0, nil // No shares of this type for this user/market, treat as 0
	}
	if err != nil {
		return 0, err
	}
	if len(valBytes) == 0 {
		return 0, nil // Key exists but empty value, treat as 0
	}
	reader := codec.NewReader(valBytes, len(valBytes))
	balance := reader.UnpackUint64(true) // true for required
	if errs := reader.Err(); errs != nil {
		return 0, fmt.Errorf("failed to unpack share balance for market %d, user %s, type %d: %w", marketID, user, shareType, errs)
	}
	return balance, nil
}

// SetShareBalance sets a user's share balance for a specific market and share type.
func SetShareBalance(ctx context.Context, mu state.Mutable, marketID uint64, user codec.Address, shareType uint8, amount uint64) error {
	key := ShareBalanceKey(marketID, user, shareType)
	writer := codec.NewWriter(8, 8) // Use literal 8, 8 for Uint64Len
	writer.PackUint64(amount)
	if errs := writer.Err(); errs != nil {
		return fmt.Errorf("failed to pack share balance for market %d, user %s, type %d: %w", marketID, user, shareType, errs)
	}
	return mu.Insert(ctx, key, writer.Bytes()) // Use Insert and pass ctx
}

// AddShares adds a specified amount of shares to a user's balance for a market and share type.
func AddShares(ctx context.Context, mu state.Mutable, marketID uint64, user codec.Address, shareType uint8, amountToAdd uint64) error {
	currentShares, err := GetShareBalance(ctx, mu, marketID, user, shareType) // Pass ctx
	if err != nil {
		return fmt.Errorf("failed to get current share balance for user %s, market %d, type %d: %w", user, marketID, shareType, err)
	}
	newShares := currentShares + amountToAdd
	// TODO: Check for overflow if share amounts can become extremely large
	return SetShareBalance(ctx, mu, marketID, user, shareType, newShares) // Pass ctx
}

// DeductShares subtracts a specified amount of shares from a user's balance.
// Returns an error if the user does not have enough shares.
func DeductShares(ctx context.Context, mu state.Mutable, marketID uint64, user codec.Address, shareType uint8, amountToDeduct uint64) error {
	currentShares, err := GetShareBalance(ctx, mu, marketID, user, shareType) // Pass ctx
	if err != nil {
		return fmt.Errorf("failed to get current share balance for user %s, market %d, type %d: %w", user, marketID, shareType, err)
	}
	if currentShares < amountToDeduct {
		return fmt.Errorf("insufficient %s shares for user %s in market %d (has %d, needs %d)", pvmConsts.ShareTypeToString(shareType), user, marketID, currentShares, amountToDeduct)
	}
	newShares := currentShares - amountToDeduct
	return SetShareBalance(ctx, mu, marketID, user, shareType, newShares) // Pass ctx
}
