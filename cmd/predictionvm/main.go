// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/ulimit"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/chain"
	// "github.com/chokosabe/predictionvm/cmd/predictionvm/version" // TODO: Create this version package
	"github.com/ava-labs/hypersdk/snow"

	pvm "github.com/chokosabe/predictionvm/vm"
)

var rootCmd = &cobra.Command{
	Use:        "predictionvm",
	Short:      "BaseVM agent",
	SuggestFor: []string{"predictionvm"},
	RunE:       runFunc,
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCmd.AddCommand(
		// version.NewCommand(), // TODO: Re-enable when version package is created
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "predictionvm failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runFunc(*cobra.Command, []string) error {
	if err := ulimit.Set(ulimit.DefaultFDLimit, logging.NoLog{}); err != nil {
		return fmt.Errorf("%w: failed to set fd limit correctly", err)
	}

	v, err := pvm.New()
	if err != nil {
		return err
	}

	return rpcchainvm.Serve(context.TODO(), snow.NewSnowVM[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]("v0.0.1", v))
}
