package actions

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/fees"
)

// MockRules provides a mock implementation of chain.Rules for testing.
// Ensure this is defined only once for the 'actions' package tests.
var _ chain.Rules = (*MockRules)(nil)

type MockRules struct {
	GetTimeFunc           func() int64
	MaxActionGasFunc      func(action chain.Action) uint64
	MaxBlockGasFunc       func() uint64
	FetchCustomFunc       func(key string) (any, bool)
	GetBaseComputeUnitsFunc func() uint64 // Corrected: No arguments
	GetChainIDFunc        func() ids.ID
	GetMaxActionsPerTxFunc func() uint8
	GetMaxBlockUnitsFunc  func() fees.Dimensions
	GetMinBlockGapFunc    func() int64
	GetMinEmptyBlockGapFunc func() int64
	GetMinUnitPriceFunc   func() fees.Dimensions
	GetNetworkIDFunc      func() uint32
	GetSponsorStateKeysMaxChunksFunc func() []uint16
	GetStorageKeyAllocateUnitsFunc func() uint64
	GetStorageKeyReadUnitsFunc func() uint64
	GetStorageKeyWriteUnitsFunc func() uint64
	GetStorageValueAllocateUnitsFunc func() uint64
	GetStorageValueReadUnitsFunc func() uint64
	GetStorageValueWriteUnitsFunc func() uint64
	GetUnitPriceChangeDenominatorFunc func() fees.Dimensions
	GetValidityWindowFunc func() int64
	GetWindowTargetUnitsFunc func() fees.Dimensions
}

func (m *MockRules) GetTime() int64 {
	if m.GetTimeFunc != nil {
		return m.GetTimeFunc()
	}
	return 0
}

func (m *MockRules) MaxActionGas(action chain.Action) uint64 {
	if m.MaxActionGasFunc != nil {
		return m.MaxActionGasFunc(action)
	}
	return 0
}

func (m *MockRules) MaxBlockGas() uint64 {
	if m.MaxBlockGasFunc != nil {
		return m.MaxBlockGasFunc()
	}
	return 0
}

// FetchCustom implements chain.Rules
func (m *MockRules) FetchCustom(key string) (any, bool) {
	if m.FetchCustomFunc != nil {
		return m.FetchCustomFunc(key)
	}
	// Default mock behavior: return nil and false (not found)
	return nil, false
}

// GetBaseComputeUnits implements chain.Rules
func (m *MockRules) GetBaseComputeUnits() uint64 { // Corrected: No arguments
	if m.GetBaseComputeUnitsFunc != nil {
		return m.GetBaseComputeUnitsFunc()
	}
	return 0 // Default base compute units
}

// GetChainID implements chain.Rules
func (m *MockRules) GetChainID() ids.ID {
	if m.GetChainIDFunc != nil {
		return m.GetChainIDFunc()
	}
	return ids.Empty // Default chain ID
}

// GetMaxActionsPerTx implements chain.Rules
func (m *MockRules) GetMaxActionsPerTx() uint8 {
	if m.GetMaxActionsPerTxFunc != nil {
		return m.GetMaxActionsPerTxFunc()
	}
	return 0 // Default max actions
}

// GetMaxBlockUnits implements chain.Rules
func (m *MockRules) GetMaxBlockUnits() fees.Dimensions {
	if m.GetMaxBlockUnitsFunc != nil {
		return m.GetMaxBlockUnitsFunc()
	}
	return fees.Dimensions{} // Default max block units
}

// GetMinBlockGap implements chain.Rules
func (m *MockRules) GetMinBlockGap() int64 {
	if m.GetMinBlockGapFunc != nil {
		return m.GetMinBlockGapFunc()
	}
	return 0 // Default min block gap
}

// GetMinEmptyBlockGap implements chain.Rules
func (m *MockRules) GetMinEmptyBlockGap() int64 {
	if m.GetMinEmptyBlockGapFunc != nil {
		return m.GetMinEmptyBlockGapFunc()
	}
	return 0 // Default min empty block gap
}

// GetMinUnitPrice implements chain.Rules
func (m *MockRules) GetMinUnitPrice() fees.Dimensions {
	if m.GetMinUnitPriceFunc != nil {
		return m.GetMinUnitPriceFunc()
	}
	return fees.Dimensions{} // Default min unit price
}

// GetNetworkID implements chain.Rules
func (m *MockRules) GetNetworkID() uint32 {
	if m.GetNetworkIDFunc != nil {
		return m.GetNetworkIDFunc()
	}
	return 0 // Default network ID
}

// GetSponsorStateKeysMaxChunks implements chain.Rules
func (m *MockRules) GetSponsorStateKeysMaxChunks() []uint16 {
	if m.GetSponsorStateKeysMaxChunksFunc != nil {
		return m.GetSponsorStateKeysMaxChunksFunc()
	}
	return nil // Default sponsor state keys max chunks
}

// GetStorageKeyAllocateUnits implements chain.Rules
func (m *MockRules) GetStorageKeyAllocateUnits() uint64 {
	if m.GetStorageKeyAllocateUnitsFunc != nil {
		return m.GetStorageKeyAllocateUnitsFunc()
	}
	return 0 // Default storage key allocate units
}

// GetStorageKeyReadUnits implements chain.Rules
func (m *MockRules) GetStorageKeyReadUnits() uint64 {
	if m.GetStorageKeyReadUnitsFunc != nil {
		return m.GetStorageKeyReadUnitsFunc()
	}
	return 0 // Default storage key read units
}

// GetStorageKeyWriteUnits implements chain.Rules
func (m *MockRules) GetStorageKeyWriteUnits() uint64 {
	if m.GetStorageKeyWriteUnitsFunc != nil {
		return m.GetStorageKeyWriteUnitsFunc()
	}
	return 0 // Default storage key write units
}

// GetStorageValueAllocateUnits implements chain.Rules
func (m *MockRules) GetStorageValueAllocateUnits() uint64 {
	if m.GetStorageValueAllocateUnitsFunc != nil {
		return m.GetStorageValueAllocateUnitsFunc()
	}
	return 0 // Default storage value allocate units
}

// GetStorageValueReadUnits implements chain.Rules
func (m *MockRules) GetStorageValueReadUnits() uint64 {
	if m.GetStorageValueReadUnitsFunc != nil {
		return m.GetStorageValueReadUnitsFunc()
	}
	return 0 // Default storage value read units
}

// GetStorageValueWriteUnits implements chain.Rules
func (m *MockRules) GetStorageValueWriteUnits() uint64 {
	if m.GetStorageValueWriteUnitsFunc != nil {
		return m.GetStorageValueWriteUnitsFunc()
	}
	return 0 // Default storage value write units
}

// GetUnitPriceChangeDenominator implements chain.Rules
func (m *MockRules) GetUnitPriceChangeDenominator() fees.Dimensions {
	if m.GetUnitPriceChangeDenominatorFunc != nil {
		return m.GetUnitPriceChangeDenominatorFunc()
	}
	return fees.Dimensions{} // Default unit price change denominator
}

// GetValidityWindow implements chain.Rules
func (m *MockRules) GetValidityWindow() int64 {
	if m.GetValidityWindowFunc != nil {
		return m.GetValidityWindowFunc()
	}
	return 0 // Default validity window
}

// GetWindowTargetUnits implements chain.Rules
func (m *MockRules) GetWindowTargetUnits() fees.Dimensions {
	if m.GetWindowTargetUnitsFunc != nil {
		return m.GetWindowTargetUnitsFunc()
	}
	return fees.Dimensions{} // Default window target units
}
