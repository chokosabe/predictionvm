package actions

import (
	"github.com/ava-labs/hypersdk/chain"
)

// Action types
const (
	CreateMarketType byte = iota // 0
	BuyYesType                   // 1
	BuyNoType                    // 2
	ClaimType                    // 3
	_                            // Placeholder to shift iota, making next value 4
	ResolveType                  // This will now be 5
)

// ActionRegistry maps action types to their respective structs.
var ActionRegistry = map[byte]chain.Action{
	CreateMarketType: &CreateMarket{},
	BuyYesType:       &BuyYes{},
	BuyNoType:        &BuyNo{},
	ClaimType:        &Claim{},
	// ResolveType will now be 5 due to the placeholder in iota
	ResolveType:      &Resolve{},
}
