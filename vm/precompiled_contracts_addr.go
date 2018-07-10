// Copyright 2018 combchain Foundation Ltd

package vm

import (
	"github.com/combchain/go-combchain/common"
	"github.com/combchain/go-combchain/core/types"
	"math/big"
)

// Precompiled contracts address or
// Reserved contracts address.
// Should prevent overwriting to them.
var (
	ecrecoverPrecompileAddr      = common.BytesToAddress([]byte{1})
	sha256hashPrecompileAddr     = common.BytesToAddress([]byte{2})
	ripemd160hashPrecompileAddr  = common.BytesToAddress([]byte{3})
	dataCopyPrecompileAddr       = common.BytesToAddress([]byte{4})
	bigModExpPrecompileAddr      = common.BytesToAddress([]byte{5})
	bn256AddPrecompileAddr       = common.BytesToAddress([]byte{6})
	bn256ScalarMulPrecompileAddr = common.BytesToAddress([]byte{7})
	bn256PairingPrecompileAddr   = common.BytesToAddress([]byte{8})

	combCoinPrecompileAddr  = common.BytesToAddress([]byte{100})
	combStampPrecompileAddr = common.BytesToAddress([]byte{200})

	otaBalanceStorageAddr = common.BytesToAddress(big.NewInt(300).Bytes())
	otaImageStorageAddr   = common.BytesToAddress(big.NewInt(301).Bytes())

	// 0.01comb --> "0x0000000000000000000000010000000000000000"
	otaBalancePercentdot001WStorageAddr = common.HexToAddress(combStampdot001)
	otaBalancePercentdot002WStorageAddr = common.HexToAddress(combStampdot002)
	otaBalancePercentdot005WStorageAddr = common.HexToAddress(combStampdot005)
	
	otaBalancePercentdot003WStorageAddr = common.HexToAddress(combStampdot003)
	otaBalancePercentdot006WStorageAddr = common.HexToAddress(combStampdot006)
	otaBalancePercentdot009WStorageAddr = common.HexToAddress(combStampdot009)

	otaBalancePercentdot03WStorageAddr = common.HexToAddress(combStampdot03)
	otaBalancePercentdot06WStorageAddr = common.HexToAddress(combStampdot06)
	otaBalancePercentdot09WStorageAddr = common.HexToAddress(combStampdot09)
	otaBalancePercentdot2WStorageAddr = common.HexToAddress(combStampdot2)
	otaBalancePercentdot5WStorageAddr = common.HexToAddress(combStampdot5)

	otaBalance10WStorageAddr       = common.HexToAddress(combcoin10)
	otaBalance20WStorageAddr       = common.HexToAddress(combcoin20)
	otaBalance50WStorageAddr       = common.HexToAddress(combcoin50)
	otaBalance100WStorageAddr      = common.HexToAddress(combcoin100)

	otaBalance200WStorageAddr       = common.HexToAddress(combcoin200)
	otaBalance500WStorageAddr       = common.HexToAddress(combcoin500)
	otaBalance1000WStorageAddr      = common.HexToAddress(combcoin1000)
	otaBalance5000WStorageAddr      = common.HexToAddress(combcoin5000)
	otaBalance50000WStorageAddr     = common.HexToAddress(combcoin50000)
)

// PrecompiledContract is the basic interface for native Go contracts. The implementation
// requires a deterministic gas count based on the input size of the Run method of the
// contract.
type PrecompiledContract interface {
	RequiredGas(input []byte) uint64                                // RequiredPrice calculates the contract gas use
	Run(input []byte, contract *Contract, evm *EVM) ([]byte, error) // Run runs the precompiled contract
	ValidTx(stateDB StateDB, signer types.Signer, tx *types.Transaction) error
}

// PrecompiledContractsHomestead contains the default set of pre-compiled Ethereum
// contracts used in the Frontier and Homestead releases.
var PrecompiledContractsHomestead = map[common.Address]PrecompiledContract{
	ecrecoverPrecompileAddr:     &ecrecover{},
	sha256hashPrecompileAddr:    &sha256hash{},
	ripemd160hashPrecompileAddr: &ripemd160hash{},
	dataCopyPrecompileAddr:      &dataCopy{},

	combCoinPrecompileAddr:  &combCoinSC{},
	combStampPrecompileAddr: &combchainStampSC{},
}

// PrecompiledContractsByzantium contains the default set of pre-compiled Ethereum
// contracts used in the Byzantium release.
var PrecompiledContractsByzantium = map[common.Address]PrecompiledContract{
	ecrecoverPrecompileAddr:      &ecrecover{},
	sha256hashPrecompileAddr:     &sha256hash{},
	ripemd160hashPrecompileAddr:  &ripemd160hash{},
	dataCopyPrecompileAddr:       &dataCopy{},
	bigModExpPrecompileAddr:      &bigModExp{},
	bn256AddPrecompileAddr:       &bn256Add{},
	bn256ScalarMulPrecompileAddr: &bn256ScalarMul{},
	bn256PairingPrecompileAddr:   &bn256Pairing{},

	combCoinPrecompileAddr:  &combCoinSC{},
	combStampPrecompileAddr: &combchainStampSC{},
}
