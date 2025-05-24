// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	// "encoding/binary" // No longer needed after removing Transfer action
	"errors"
	"fmt" // Added fmt import
	"time"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
	// "github.com/chokosabe/predictionvm/actions" // Refactored import - Removed as Transfer action is being deleted
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/load"
)

var (
	ErrTxGeneratorFundsExhausted = errors.New("tx generator funds exhausted")

	_ load.Issuer[*chain.Transaction] = (*Issuer)(nil)
)

type Issuer struct {
	authFactory chain.AuthFactory
	currBalance uint64
	ruleFactory chain.RuleFactory
	unitPrices  fees.Dimensions

	client  *ws.WebSocketClient
	tracker load.Tracker[ids.ID]
}

func NewIssuer(
	authFactory chain.AuthFactory,
	ruleFactory chain.RuleFactory,
	currBalance uint64,
	unitPrices fees.Dimensions,
	client *ws.WebSocketClient,
	tracker load.Tracker[ids.ID],
) *Issuer {
	return &Issuer{
		authFactory: authFactory,
		ruleFactory: ruleFactory,
		currBalance: currBalance,
		unitPrices:  unitPrices,
		client:      client,
		tracker:     tracker,
	}
}

func (i *Issuer) GenerateTx(_ context.Context) (*chain.Transaction, error) {
	tx, err := chain.GenerateTransaction(
		i.ruleFactory,
		i.unitPrices,
		time.Now().UnixMilli(),
		[]chain.Action{
			// &actions.Transfer{ // Commented out as Transfer action is being removed
			// 	To:    i.authFactory.Address(),
			// 	Value: 1,
			// 	Memo:  binary.BigEndian.AppendUint64(nil, i.currBalance),
			// },
		},
		i.authFactory,
	)
	if err != nil {
		return nil, err
	}
	if tx.MaxFee() > i.currBalance {
		return nil, ErrTxGeneratorFundsExhausted
	}
	i.currBalance -= tx.MaxFee()
	return tx, nil
}

func (i *Issuer) IssueTx(ctx context.Context, tx *chain.Transaction) error {
	if err := i.client.RegisterTx(tx); err != nil {
		return err
	}
	i.tracker.Issue(tx.GetID())
	return nil
}

// Listen for the final status of transactions and notify the tracker.
// Listen stops if the context is done, an error occurs, or if the issuer
// has sent all their transactions.
func (i *Issuer) Listen(ctx context.Context) error {
	// TODO: Implement actual listening logic. This typically involves subscribing
	// to transaction finalization events from the WebSocket client and then
	// calling i.tracker.ObserveConfirmed(txID) or i.tracker.ObserveFailed(txID).
	// For now, we can just block until context is done as a placeholder.
	fmt.Println("[Issuer.Listen] Placeholder: Listening for transaction finalization (blocks until context done).")
	<-ctx.Done()
	return ctx.Err()
}

// Stop notifies the issuer that no further transactions will be issued.
// If a transaction is issued after Stop has been called, the issuer should error.
func (i *Issuer) Stop() {
	// TODO: Implement logic to prevent further transaction issuance if needed.
	// This might involve setting a flag in the Issuer struct.
	fmt.Println("[Issuer.Stop] Placeholder: Issuer stop called.")
	// i.stopped = true (example)
}
