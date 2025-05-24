// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	// "context" // No longer needed after removing transferCmd

	"github.com/spf13/cobra"

	// "github.com/ava-labs/hypersdk/chain" // No longer needed after removing transferCmd
	// "github.com/ava-labs/hypersdk/cli/prompt" // No longer needed after removing transferCmd
	// "github.com/chokosabe/predictionvm/actions" // Removed as Transfer action is being deleted
)

var actionCmd = &cobra.Command{
	Use: "action",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}
