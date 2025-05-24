// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"errors"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/genesis" // Added hypersdk/genesis
	"github.com/ava-labs/hypersdk/state/metadata"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/vm/defaultvm"

	"github.com/chokosabe/predictionvm/actions"    // Local actions
	"github.com/chokosabe/predictionvm/controller" // Local controller
)

var (
	ActionParser *codec.TypeParser[chain.Action]
	AuthParser   *codec.TypeParser[chain.Auth]
	OutputParser *codec.TypeParser[codec.Typed]

	AuthProvider *auth.AuthProvider

	Parser *chain.TxTypeParser
)

// Setup types
func init() {
	ActionParser = codec.NewTypeParser[chain.Action]()
	AuthParser = codec.NewTypeParser[chain.Auth]()
	OutputParser = codec.NewTypeParser[codec.Typed]()
	AuthProvider = auth.NewAuthProvider()

	if err := auth.WithDefaultPrivateKeyFactories(AuthProvider); err != nil {
		panic(err)
	}

	if err := errors.Join(
		// PredictionVM Actions
		ActionParser.Register(&actions.CreateMarket{}, nil), // Uses CreateMarket.Unmarshal method
		ActionParser.Register(&actions.BuyYes{}, actions.UnmarshalBuyYes),
		ActionParser.Register(&actions.BuyNo{}, nil),    // Uses BuyNo.Unmarshal method
		ActionParser.Register(&actions.Claim{}, actions.UnmarshalClaim),
		ActionParser.Register(&actions.Resolve{}, actions.UnmarshalResolve),

		// Standard Auth Types
		AuthParser.Register(&auth.ED25519{}, auth.UnmarshalED25519),
		AuthParser.Register(&auth.SECP256R1{}, auth.UnmarshalSECP256R1),
		AuthParser.Register(&auth.BLS{}, auth.UnmarshalBLS),

		OutputParser.Register(&actions.CreateMarketResult{}, nil), // Register CreateMarketResult
		OutputParser.Register(&actions.BuyYesResult{}, nil),       // Register BuyYesResult
		OutputParser.Register(&actions.BuyNoResult{}, nil),        // Register BuyNoResult
		OutputParser.Register(&actions.ResolveResult{}, nil),      // Register ResolveResult
		OutputParser.Register(&actions.ClaimResult{}, nil),        // Register ClaimResult
	); err != nil {
		panic(err)
	}

	Parser = chain.NewTxTypeParser(ActionParser, AuthParser)
}

// New returns a VM with the specified options
func New(options ...vm.Option) (*vm.VM, error) {
	factory := NewFactory()
	return factory.New(options...)
}

func NewFactory() *vm.Factory {
	options := defaultvm.NewDefaultOptions() // Start with default options
	return vm.NewFactory(
		&genesis.DefaultGenesisFactory{},
		controller.New(),
		metadata.NewDefaultManager(),
		ActionParser,
		AuthParser,
		OutputParser,
		auth.DefaultEngines(),
		options...,
	)
}
