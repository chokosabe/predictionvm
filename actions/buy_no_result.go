package actions

import (
	"github.com/ava-labs/hypersdk/codec"
	pvmconsts "github.com/chokosabe/predictionvm/consts"
)

// Ensure BuyNoResult implements codec.Typed
var _ codec.Typed = (*BuyNoResult)(nil)

// BuyNoResult is the output of a successful BuyNo action.
type BuyNoResult struct {
	SharesBought uint64 `serialize:"true" json:"sharesBought"`
	CostPaid     uint64 `serialize:"true" json:"costPaid"`
}

// GetTypeID returns the type ID of the BuyNoResult.
func (br *BuyNoResult) GetTypeID() uint8 {
	return pvmconsts.BuyNoID // Reusing BuyNoID for its result type
}

// MarshalCodec serializes the BuyNoResult into bytes using the provided packer.
func (br *BuyNoResult) MarshalCodec(p *codec.Packer) error {
	p.PackUint64(br.SharesBought)
	p.PackUint64(br.CostPaid)
	return p.Err()
}

// UnmarshalCodec deserializes bytes into a BuyNoResult using the provided unpacker.
func (br *BuyNoResult) UnmarshalCodec(p *codec.Packer) error {
	br.SharesBought = p.UnpackUint64(true)
	br.CostPaid = p.UnpackUint64(true)
	return p.Err()
}
