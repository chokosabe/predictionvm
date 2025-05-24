package actions

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"

	"github.com/chokosabe/predictionvm/consts"
)

var _ codec.Typed = (*ClaimResult)(nil)

// ClaimResult is the output of a successful Claim action.
// It indicates the market, claimant, asset, and amount paid out.
type ClaimResult struct {
	MarketID        ids.ID        `json:"marketId"`
	ClaimantAddress codec.Address `json:"claimantAddress"`
	PayoutAssetID   ids.ID        `json:"payoutAssetId"`
	AmountClaimed   uint64        `json:"amountClaimed"`
}

// GetTypeID returns the type ID of the ClaimResult.
func (cr *ClaimResult) GetTypeID() byte {
	return consts.ClaimOutput
}

// MarshalCodec serializes the ClaimResult into bytes using the provided packer.
func (cr *ClaimResult) MarshalCodec(p *codec.Packer) error {
	p.PackID(cr.MarketID)
	if err := p.Err(); err != nil {
		return err
	}

	p.PackAddress(cr.ClaimantAddress)
	if err := p.Err(); err != nil {
		return err
	}

	p.PackID(cr.PayoutAssetID)
	if err := p.Err(); err != nil {
		return err
	}

	p.PackUint64(cr.AmountClaimed)
	return p.Err()
}

// UnmarshalCodec deserializes bytes into a ClaimResult using the provided reader.
func (cr *ClaimResult) UnmarshalCodec(p *codec.Packer) error {
	// p is of type *codec.Packer (from github.com/ava-labs/hypersdk/codec), configured for reading.
	// The methods UnpackID, UnpackAddress, UnpackUint64 are available on this type.
	p.UnpackID(true, &cr.MarketID) // UnpackID(required bool, dest *ids.ID)
	p.UnpackAddress(&cr.ClaimantAddress)  // UnpackAddress(dest *Address)
	p.UnpackID(true, &cr.PayoutAssetID) // UnpackID(required bool, dest *ids.ID)
	cr.AmountClaimed = p.UnpackUint64(true) // UnpackUint64(required bool) uint64
	return p.Err()
}
