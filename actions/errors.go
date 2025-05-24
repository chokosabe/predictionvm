package actions

import "errors"

var (
	ErrNotImplemented         = errors.New("not implemented")
	ErrMarketNotFound         = errors.New("market not found")
	ErrMarketNotResolved      = errors.New("market is not resolved")
	ErrMarketOutcomePending   = errors.New("market outcome is pending")
	ErrMarketInvalidNoPayout  = errors.New("market resolved as invalid, no payout")
	ErrNoWinningShares        = errors.New("no winning shares to claim")
)
