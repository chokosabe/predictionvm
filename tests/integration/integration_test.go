// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	// _ "github.com/chokosabe/predictionvm/tests" // Removed problematic import

	"github.com/chokosabe/predictionvm/tests/workload" // Refactored import
	"github.com/ava-labs/hypersdk/tests/registry"
	"github.com/ava-labs/hypersdk/vm/vmtest"

	predictionvm "github.com/chokosabe/predictionvm/vm" // Refactored import and aliased
)

func TestIntegration(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	vmFactory := predictionvm.NewFactory() // Use new alias

	testingNetworkConfig, err := workload.NewTestNetworkConfig(0)
	r.NoError(err)

	testNetwork := vmtest.NewTestNetwork(
		ctx,
		t,
		vmFactory,
		testingNetworkConfig.GenesisAndRuleFactory(),
		2,
		testingNetworkConfig.AuthFactories(),
		testingNetworkConfig.GenesisBytes(),
		nil,
		nil,
	)

	for testRegistry := range registry.GetTestsRegistries() {
		for _, test := range testRegistry.List() {
			t.Run(test.Name, func(t *testing.T) {
				test.Fnc(t, testNetwork)
			})
		}
	}
}
