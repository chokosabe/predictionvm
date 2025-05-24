package actions

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	pvmconsts "github.com/chokosabe/predictionvm/consts"
	"github.com/chokosabe/predictionvm/storage"
)

// Ensure ResolveResult implements codec.Typed
var _ codec.Typed = (*ResolveResult)(nil)

// ResolveResult is the output of a successful Resolve action.
type ResolveResult struct {
	MarketID   ids.ID                `serialize:"true" json:"marketId"`
	Resolution storage.OutcomeType   `serialize:"true" json:"resolution"` // Changed to OutcomeType
}

// GetTypeID returns the type ID of the ResolveResult.
func (rr *ResolveResult) GetTypeID() uint8 {
	return pvmconsts.ResolveID // Assuming ResolveID is defined in pvmconsts for the Resolve action type
}

// MarshalCodec serializes the ResolveResult into bytes using the provided packer.
func (rr *ResolveResult) MarshalCodec(p *codec.Packer) error {
	p.PackID(rr.MarketID)
	p.PackByte(uint8(rr.Resolution)) // MarketResolution is uint8
	return p.Err()
}

// UnmarshalCodec deserializes bytes into a ResolveResult using the provided unpacker.
func (rr *ResolveResult) UnmarshalCodec(p *codec.Packer) error {
	p.UnpackID(true, &rr.MarketID) // require=true, dst=*ids.ID
	rr.Resolution = storage.OutcomeType(p.UnpackByte())
	return p.Err()
}
