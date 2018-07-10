// Copyright 2017 The go-ethereum Authors
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
	"math/big"
	"reflect"
	"testing"

	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/combchain/go-combchain/common"
	"github.com/combchain/go-combchain/ethdb"
	"github.com/combchain/go-combchain/params"
)

func TestDefaultGenesisBlock(t *testing.T) {
	block, _ := DefaultGenesisBlock().ToBlock()
	fmt.Println(common.ToHex(block.Hash().Bytes()))
	if block.Hash() != params.MainnetGenesisHash {
		t.Errorf("wrong mainnet genesis hash, got %v, combt %v", block.Hash(), params.MainnetGenesisHash)
	}

	block, _ = DefaultTestnetGenesisBlock().ToBlock()
	fmt.Println(common.ToHex(block.Hash().Bytes()))
	if block.Hash() != params.TestnetGenesisHash {
		t.Errorf("wrong testnet genesis hash, got %v, combt %v", block.Hash(), params.TestnetGenesisHash)
	}

	block, _ = DefaultInternalGenesisBlock().ToBlock()
	fmt.Println(common.ToHex(block.Hash().Bytes()))
	if block.Hash() != params.InternalGenesisHash {
		t.Errorf("wrong testnet genesis hash, got %v, combt %v", block.Hash(), params.TestnetGenesisHash)
	}
}

func TestDefaultTestnetGenesisBlock(t *testing.T) {
	block, _ := DefaultGenesisBlock().ToBlock()
	if block.Hash() != params.MainnetGenesisHash {
		t.Errorf("wrong mainnet genesis hash, got %v, combt %v", block.Hash(), params.MainnetGenesisHash)
	}

	block, _ = DefaultTestnetGenesisBlock().ToBlock()
	if block.Hash() != params.TestnetGenesisHash {
		t.Errorf("wrong testnet genesis hash, got %v, combt %v", block.Hash(), params.TestnetGenesisHash)
	}
}

func TestSetupGenesis(t *testing.T) {
	var (
		customghash = common.HexToHash("0x89c99d90b79719238d2645c7642f2c9295246e80775b38cfd162b696817fbd50")
		customg     = Genesis{
			Config: &params.ChainConfig{ByzantiumBlock: big.NewInt(3)},
			Alloc: GenesisAlloc{
				{1}: {Balance: big.NewInt(1), Storage: map[common.Hash]common.Hash{{1}: {1}}},
			},
		}
		oldcustomg = customg
	)
	oldcustomg.Config = &params.ChainConfig{ByzantiumBlock: big.NewInt(2)}
	tests := []struct {
		name       string
		fn         func(ethdb.Database) (*params.ChainConfig, common.Hash, error)
		combtConfig *params.ChainConfig
		combtHash   common.Hash
		combtErr    error
	}{
		{
			name: "genesis without ChainConfig",
			fn: func(db ethdb.Database) (*params.ChainConfig, common.Hash, error) {
				return SetupGenesisBlock(db, new(Genesis))
			},
			combtErr:    errGenesisNoConfig,
			combtConfig: params.AllProtocolChanges,
		},
		{
			name: "no block in DB, genesis == nil",
			fn: func(db ethdb.Database) (*params.ChainConfig, common.Hash, error) {
				return SetupGenesisBlock(db, nil)
			},
			combtHash:   params.MainnetGenesisHash,
			combtConfig: params.MainnetChainConfig,
		},
		{
			name: "mainnet block in DB, genesis == nil",
			fn: func(db ethdb.Database) (*params.ChainConfig, common.Hash, error) {
				DefaultGenesisBlock().MustCommit(db)
				return SetupGenesisBlock(db, nil)
			},
			combtHash:   params.MainnetGenesisHash,
			combtConfig: params.MainnetChainConfig,
		},
		{
			name: "custom block in DB, genesis == nil",
			fn: func(db ethdb.Database) (*params.ChainConfig, common.Hash, error) {
				customg.MustCommit(db)
				return SetupGenesisBlock(db, nil)
			},
			combtHash:   customghash,
			combtConfig: customg.Config,
		},
		{
			name: "custom block in DB, genesis == testnet",
			fn: func(db ethdb.Database) (*params.ChainConfig, common.Hash, error) {
				customg.MustCommit(db)
				return SetupGenesisBlock(db, DefaultTestnetGenesisBlock())
			},
			combtErr:    &GenesisMismatchError{Stored: customghash, New: params.TestnetGenesisHash},
			combtHash:   params.TestnetGenesisHash,
			combtConfig: params.TestnetChainConfig,
		},
		{
			name: "compatible config in DB",
			fn: func(db ethdb.Database) (*params.ChainConfig, common.Hash, error) {
				oldcustomg.MustCommit(db)
				return SetupGenesisBlock(db, &customg)
			},
			combtHash:   customghash,
			combtConfig: customg.Config,
		},

		/*
			{
				name: "incompatible config in DB",
				fn: func(db ethdb.Database) (*params.ChainConfig, common.Hash, error) {
					// Commit the 'old' genesis block with Homestead transition at #2.
					// Advance to block #4, past the homestead transition block of customg.
					genesis := oldcustomg.MustCommit(db)
					bc, _ := NewBlockChain(db, oldcustomg.Config, ethash.NewFullFaker(), vm.Config{})
					defer bc.Stop()
					bc.SetValidator(bproc{})
					bc.InsertChain(makeBlockChainWithDiff(genesis, []int{2, 3, 4, 5}, 0))
					bc.CurrentBlock()
					// This should return a compatibility error.
					return SetupGenesisBlock(db, &customg)
				},
				combtHash:   customghash,
				combtConfig: customg.Config,
				combtErr: &params.ConfigCompatError{
					What:         "Homestead fork block",
					StoredConfig: big.NewInt(2),
					NewConfig:    big.NewInt(3),
					RewindTo:     1,
				},
			},
		*/

	}

	for _, test := range tests {
		db, _ := ethdb.NewMemDatabase()
		config, hash, err := test.fn(db)
		// Check the return values.
		if !reflect.DeepEqual(err, test.combtErr) {
			spew := spew.ConfigState{DisablePointerAddresses: true, DisableCapacities: true}
			t.Errorf("%s: returned error %#v, combt %#v", test.name, spew.NewFormatter(err), spew.NewFormatter(test.combtErr))
		}
		if !reflect.DeepEqual(config, test.combtConfig) {
			t.Errorf("%s:\nreturned %v\ncombt     %v", test.name, config, test.combtConfig)
		}
		if hash != test.combtHash {
			t.Errorf("%s: returned hash %s, combt %s", test.name, hash.Hex(), test.combtHash.Hex())
		} else if err == nil {
			// Check database content.
			stored := GetBlock(db, test.combtHash, 0)
			if stored.Hash() != test.combtHash {
				t.Errorf("%s: block in DB has hash %s, combt %s", test.name, stored.Hash(), test.combtHash)
			}
		}
	}
}
