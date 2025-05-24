package actions

import (
	"github.com/ava-labs/hypersdk/codec"
	pvmconsts "github.com/chokosabe/predictionvm/consts"
)

// Ensure BuyYesResult implements codec.Typed
var _ codec.Typed = (*BuyYesResult)(nil)

// BuyYesResult is the output of a successful BuyYes action.
type BuyYesResult struct {
	SharesBought uint64 `serialize:"true" json:"sharesBought"`
	CostPaid     uint64 `serialize:"true" json:"costPaid"`
}

// GetTypeID returns the type ID of the BuyYesResult.
func (br *BuyYesResult) GetTypeID() uint8 {
	return pvmconsts.BuyYesID // Reusing BuyYesID for its result type
}

// MarshalCodec serializes the BuyYesResult into bytes using the provided packer.
func (br *BuyYesResult) MarshalCodec(p *codec.Packer) error {
	p.PackUint64(br.SharesBought)
	p.PackUint64(br.CostPaid)
	return p.Err()
}

// UnmarshalCodec deserializes bytes into a BuyYesResult using the provided unpacker.
func (br *BuyYesResult) UnmarshalCodec(p *codec.Packer) error {
	br.SharesBought = p.UnpackUint64(true)
	br.CostPaid = p.UnpackUint64(true)
	return p.Err()
}
