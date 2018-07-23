// Copyright 2018 combchain Foundation Ltd
// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/combchain/combchain/crypto"
	"github.com/combchain/go-combchain/common"
	"github.com/combchain/go-combchain/common/hexutil"
	"github.com/combchain/go-combchain/common/math"
	"github.com/combchain/go-combchain/core/state"
	"github.com/combchain/go-combchain/core/types"
	"github.com/combchain/go-combchain/ethdb"
	"github.com/combchain/go-combchain/log"
	"github.com/combchain/go-combchain/params"
	"github.com/combchain/go-combchain/rlp"
)

//go:generate gencodec -type Genesis -field-override genesisSpecMarshaling -out gen_genesis.go
//go:generate gencodec -type GenesisAccount -field-override genesisAccountMarshaling -out gen_genesis_account.go

var errGenesisNoConfig = errors.New("genesis has no chain configuration")

// Genesis specifies the header fields, state of a genesis block. It also defines hard
// fork switch-over blocks through the chain configuration.
type Genesis struct {
	Config     *params.ChainConfig `json:"config"`
	Nonce      uint64              `json:"nonce"`
	Timestamp  uint64              `json:"timestamp"`
	ExtraData  []byte              `json:"extraData"`
	GasLimit   uint64              `json:"gasLimit"   gencodec:"required"`
	Difficulty *big.Int            `json:"difficulty" gencodec:"required"`
	Mixhash    common.Hash         `json:"mixHash"`
	Coinbase   common.Address      `json:"coinbase"`
	Alloc      GenesisAlloc        `json:"alloc"      gencodec:"required"`

	// These fields are used for consensus tests. Please don't use them
	// in actual genesis blocks.
	Number     uint64      `json:"number"`
	GasUsed    uint64      `json:"gasUsed"`
	ParentHash common.Hash `json:"parentHash"`
}

// GenesisAlloc specifies the initial state that is part of the genesis block.
type GenesisAlloc map[common.Address]GenesisAccount

func (ga *GenesisAlloc) UnmarshalJSON(data []byte) error {
	m := make(map[common.UnprefixedAddress]GenesisAccount)
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*ga = make(GenesisAlloc)
	for addr, a := range m {
		(*ga)[common.Address(addr)] = a
	}
	return nil
}

// GenesisAccount is an account in the state of the genesis block.
type GenesisAccount struct {
	Code       []byte                      `json:"code,omitempty"`
	Storage    map[common.Hash]common.Hash `json:"storage,omitempty"`
	Balance    *big.Int                    `json:"balance" gencodec:"required"`
	Nonce      uint64                      `json:"nonce,omitempty"`
	PrivateKey []byte                      `json:"secretKey,omitempty"` // for tests
}

// field type overrides for gencodec
type genesisSpecMarshaling struct {
	Nonce      math.HexOrDecimal64
	Timestamp  math.HexOrDecimal64
	ExtraData  hexutil.Bytes
	GasLimit   math.HexOrDecimal64
	GasUsed    math.HexOrDecimal64
	Number     math.HexOrDecimal64
	Difficulty *math.HexOrDecimal256
	Alloc      map[common.UnprefixedAddress]GenesisAccount
}

type genesisAccountMarshaling struct {
	Code       hexutil.Bytes
	Balance    *math.HexOrDecimal256
	Nonce      math.HexOrDecimal64
	Storage    map[storageJSON]storageJSON
	PrivateKey hexutil.Bytes
}

// storageJSON represents a 256 bit byte array, but allows less than 256 bits when
// unmarshaling from hex.
type storageJSON common.Hash

func (h *storageJSON) UnmarshalText(text []byte) error {
	text = bytes.TrimPrefix(text, []byte("0x"))
	if len(text) > 64 {
		return fmt.Errorf("too many hex characters in storage key/value %q", text)
	}
	offset := len(h) - len(text)/2 // pad on the left
	if _, err := hex.Decode(h[offset:], text); err != nil {
		fmt.Println(err)
		return fmt.Errorf("invalid hex storage key/value %q", text)
	}
	return nil
}

func (h storageJSON) MarshalText() ([]byte, error) {
	return hexutil.Bytes(h[:]).MarshalText()
}

// GenesisMismatchError is raised when trying to overwrite an existing
// genesis block with an incompatible one.
type GenesisMismatchError struct {
	Stored, New common.Hash
}

func (e *GenesisMismatchError) Error() string {
	return fmt.Sprintf("database already contains an incompatible genesis block (have %x, new %x)", e.Stored[:8], e.New[:8])
}

// SetupGenesisBlock writes or updates the genesis block in db.
// The block that will be used is:
//
//                          genesis == nil       genesis != nil
//                       +------------------------------------------
//     db has no genesis |  main-net default  |  genesis
//     db has genesis    |  from DB           |  genesis (if compatible)
//
// The stored chain configuration will be updated if it is compatible (i.e. does not
// specify a fork block below the local head block). In case of a conflict, the
// error is a *params.ConfigCompatError and the new, unwritten config is returned.
//
// The returned chain configuration is never nil.

func SetupGenesisBlock(db ethdb.Database, genesis *Genesis) (*params.ChainConfig, common.Hash, error) {
	if genesis != nil && genesis.Config == nil {
		return params.AllProtocolChanges, common.Hash{}, errGenesisNoConfig
	}

	// Just commit the new block if there is no stored genesis block.
	stored := GetCanonicalHash(db, 0)
	if (stored == common.Hash{}) {
		if genesis == nil {
			log.Info("Writing default main-net genesis block")
			genesis = DefaultGenesisBlock()
		} else if genesis.Config.ChainId.Cmp(big.NewInt(3)) == 0 {
			log.Info("Writing default  test net genesis block")
			genesis = DefaultTestnetGenesisBlock()
		} else {
			log.Info("Writing custom genesis block")
		}
		block, err := genesis.Commit(db)
		return genesis.Config, block.Hash(), err
	}

	// Check whether the genesis block is already written.
	if genesis != nil {
		block, _ := genesis.ToBlock()
		hash := block.Hash()
		if hash != stored {
			return genesis.Config, block.Hash(), &GenesisMismatchError{stored, hash}
		}
	}

	// Get the existing chain configuration.
	newcfg := genesis.configOrDefault(stored)
	storedcfg, err := GetChainConfig(db, stored)
	if err != nil {
		if err == ErrChainConfigNotFound {
			// This case happens if a genesis write was interrupted.
			log.Warn("Found genesis block without chain config")
			err = WriteChainConfig(db, stored, newcfg)
		}
		return newcfg, stored, err
	}
	// Special case: don't change the existing config of a non-mainnet chain if no new
	// config is supplied. These chains would get AllProtocolChanges (and a compat error)
	// if we just continued here.
	if genesis == nil && stored != params.MainnetGenesisHash {
		return storedcfg, stored, nil
	}

	// Check config compatibility and write the config. Compatibility errors
	// are returned to the caller unless we're already at block zero.
	height := GetBlockNumber(db, GetHeadHeaderHash(db))
	if height == missingNumber {
		return newcfg, stored, fmt.Errorf("missing block number for head header hash")
	}
	compatErr := storedcfg.CheckCompatible(newcfg, height)
	if compatErr != nil && height != 0 && compatErr.RewindTo != 0 {
		return newcfg, stored, compatErr
	}
	return newcfg, stored, WriteChainConfig(db, stored, newcfg)
}

func (g *Genesis) configOrDefault(ghash common.Hash) *params.ChainConfig {
	switch {
	case g != nil:
		return g.Config
	case ghash == params.MainnetGenesisHash:
		return params.combchainChainConfig
	case ghash == params.TestnetGenesisHash:
		return params.TestnetChainConfig
	case ghash == params.InternalGenesisHash:
		return params.TestnetChainConfig

	case ghash == params.PlutoGenesisHash:
		return params.PlutoChainConfig
	default:
		return params.AllProtocolChanges
	}
}

// ToBlock creates the block and state of a genesis specification.
func (g *Genesis) ToBlock() (*types.Block, *state.StateDB) {
	db, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	for addr, account := range g.Alloc {
		statedb.AddBalance(addr, account.Balance)
		statedb.SetCode(addr, account.Code)
		statedb.SetNonce(addr, account.Nonce)
		for key, value := range account.Storage {
			statedb.SetState(addr, key, value)
		}
	}
	root := statedb.IntermediateRoot(false)
	head := &types.Header{
		Number:     new(big.Int).SetUint64(g.Number),
		Nonce:      types.EncodeNonce(g.Nonce),
		Time:       new(big.Int).SetUint64(g.Timestamp),
		ParentHash: g.ParentHash,
		Extra:      g.ExtraData,
		GasLimit:   new(big.Int).SetUint64(g.GasLimit),
		GasUsed:    new(big.Int).SetUint64(g.GasUsed),
		Difficulty: g.Difficulty,
		MixDigest:  g.Mixhash,
		Coinbase:   g.Coinbase,
		Root:       root,
	}
	if g.GasLimit == 0 {
		head.GasLimit = params.GenesisGasLimit
	}
	if g.Difficulty == nil {
		head.Difficulty = params.GenesisDifficulty
	}
	return types.NewBlock(head, nil, nil, nil), statedb
}

// Commit writes the block and state of a genesis specification to the database.
// The block is committed as the canonical head block.
func (g *Genesis) Commit(db ethdb.Database) (*types.Block, error) {
	block, statedb := g.ToBlock()
	if block.Number().Sign() != 0 {
		return nil, fmt.Errorf("can't commit genesis block with number > 0")
	}
	if _, err := statedb.CommitTo(db, false); err != nil {
		return nil, fmt.Errorf("cannot write state: %v", err)
	}
	if err := WriteTd(db, block.Hash(), block.NumberU64(), g.Difficulty); err != nil {
		return nil, err
	}
	if err := WriteBlock(db, block); err != nil {
		return nil, err
	}
	if err := WriteBlockReceipts(db, block.Hash(), block.NumberU64(), nil); err != nil {
		return nil, err
	}
	if err := WriteCanonicalHash(db, block.Hash(), block.NumberU64()); err != nil {
		return nil, err
	}
	if err := WriteHeadBlockHash(db, block.Hash()); err != nil {
		return nil, err
	}
	if err := WriteHeadHeaderHash(db, block.Hash()); err != nil {
		return nil, err
	}
	config := g.Config
	if config == nil {
		config = params.AllProtocolChanges
	}
	return block, WriteChainConfig(db, block.Hash(), config)
}

// MustCommit writes the genesis block and state to db, panicking on error.
// The block is committed as the canonical head block.
func (g *Genesis) MustCommit(db ethdb.Database) *types.Block {
	block, err := g.Commit(db)
	if err != nil {
		panic(err)
	}
	return block
}

// GenesisBlockForTesting creates and writes a block in which addr has the given wei balance.
func GenesisBlockForTesting(db ethdb.Database, addr common.Address, balance *big.Int) *types.Block {
	g := Genesis{Alloc: GenesisAlloc{addr: {Balance: balance}}}
	return g.MustCommit(db)
}

// DefaultPPOWTestingGenesisBlock returns the combchain ppow testing genesis block
func DefaultPPOWTestingGenesisBlock() *Genesis {

	key, _ := crypto.HexToECDSA("f1572f76b75b40a7da72d6f2ee7fda3d1189c2d28f0a2f096347055abe344d7f")
	coinbase := crypto.PubkeyToAddress(key.PublicKey)
	return &Genesis{
		Coinbase:   coinbase,
		Config:     params.TestChainConfig,
		Nonce:      66,
		ExtraData:  hexutil.MustDecode("0xf9b32578b4420a36f132db32b56f3831a7cc1804810524175efa012446103d1a04c9f4263a962accdb05642eabc8347ec78e21bdf0d906ba579d423ab5eb9bf02a924367ed9d4f86dfcb1c572cd9a4f80036805b6846f26ac35f2a7d7eda4a2a58f08e8ef073d4e52c506f3f288faa9db1c1e5ae0f1e70f8c38eb01bce9bcb61327532dc5a540da4cf484ae57e98bc5a465c1d2afa6b9376709a525981f53d493a46ef1eb55428b3b88a222d80d23531054ef51dbd100cf8286136659a7d63a38a154e28dbf3e0fd"),
		GasLimit:   0x47b760,
		Difficulty: big.NewInt(1),
		Alloc:      jsonPrealloc(combchainPPOWTestAllocJson),
	}
}

// DefaultGenesisBlock returns the Ethereum main net genesis block.
func DefaultGenesisBlock() *Genesis {
	return &Genesis{
		Config:     params.combchainChainConfig,
		Nonce:      98,
		ExtraData:  hexutil.MustDecode(getMainNetPpwSignStr()),
		GasLimit:   0x2fefd8,
		Difficulty: big.NewInt(1048576),
		//Difficulty: big.NewInt(17179869184),
		Alloc: jsonPrealloc(combchainAllocJson),
	}
}

// DefaultTestnetGenesisBlock returns the Ropsten network genesis block.
func DefaultTestnetGenesisBlock() *Genesis {
	return &Genesis{
		Config:     params.TestnetChainConfig,
		Nonce:      28, //same with the version
		ExtraData:  hexutil.MustDecode(getTestNetPpwSignStr()),
		GasLimit:   0x2fefd8,
		Difficulty: big.NewInt(1048576),
		Alloc:      jsonPrealloc(combchainTestAllocJson),
	}
}

// DefaultInternalGenesisBlock returns the Rinkeby network genesis block.
func DefaultInternalGenesisBlock() *Genesis {
	return &Genesis{
		Config:     params.InternalChainConfig,
		Nonce:      20,
		ExtraData:  hexutil.MustDecode(getInternalNetPpwSignStr()),
		GasLimit:   0x2fefd8,
		Difficulty: big.NewInt(1),
		Alloc:      jsonPrealloc(combchainTestAllocJson),
	}
}

// DefaultPlutoGenesisBlock returns the Pluto network genesis block.

func DefaultPlutoGenesisBlock() *Genesis {
	return &Genesis{
		Config:     params.PlutoChainConfig,
		Timestamp:  0x59f83144,
		ExtraData:  hexutil.MustDecode("0x0000000000000000000000000000000000000000000000000000000000000000e8ffc3d0c02c0bfc39b139fa49e2c5475f0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"),
		GasLimit:   0x47b760,
		Difficulty: big.NewInt(1),
		Alloc:      jsonPrealloc(PlutoAllocJson),
	}
}

// DevGenesisBlock returns the 'geth --dev' genesis block.
func DevGenesisBlock() *Genesis {
	return &Genesis{
		Config:     params.AllProtocolChanges,
		Nonce:      42,
		ExtraData:  hexutil.MustDecode("0x9da26fc2e1d6ad9fdd46138906b0104ae68a65d8"),
		GasLimit:   4712388,
		Difficulty: big.NewInt(1),
		Alloc:      jsonPrealloc(combchainPPOWDevAllocJson),
	}
}

func decodePrealloc(data string) GenesisAlloc {
	var p []struct{ Addr, Balance *big.Int }
	if err := rlp.NewStream(strings.NewReader(data), 0).Decode(&p); err != nil {
		panic(err)
	}
	ga := make(GenesisAlloc, len(p))
	for _, account := range p {
		ga[common.BigToAddress(account.Addr)] = GenesisAccount{Balance: account.Balance}
	}
	return ga
}

func jsonPrealloc(data string) GenesisAlloc {
	var ga GenesisAlloc
	if err := json.Unmarshal([]byte(data), &ga); err != nil {
		panic(err)
	}
	return ga
}
