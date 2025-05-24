package actions

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	pvmconsts "github.com/chokosabe/predictionvm/consts" // Corrected import path
)

// Ensure CreateMarketResult implements codec.Typed
var _ codec.Typed = (*CreateMarketResult)(nil)

// CreateMarketResult is the output of a successful CreateMarket action.
type CreateMarketResult struct {
	MarketID ids.ID `serialize:"true" json:"marketId"`
}

// GetTypeID returns the type ID of the CreateMarketResult.
// It uses the same ID as the CreateMarket action, following the pattern
// observed in hypersdk-starter-kit (e.g., Greeting and GreetingResult).
func (cmr *CreateMarketResult) GetTypeID() uint8 {
	return pvmconsts.CreateMarketID
}

// MarshalCodec serializes the CreateMarketResult into bytes using the provided packer.
func (cmr *CreateMarketResult) MarshalCodec(p *codec.Packer) error {
	p.PackID(cmr.MarketID)
	return p.Err()
}
