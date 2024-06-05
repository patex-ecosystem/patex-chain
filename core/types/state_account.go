// Copyright 2021 The go-ethereum Authors
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

package types

import (
	"bytes"
	"github.com/ethereum/go-ethereum/rlp"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

//go:generate go run ../../rlp/rlpgen -type StateAccount -out gen_account_rlp.go
//go:generate go run ../../rlp/rlpgen -type StateAccountLegacy -out gen_account_legacy_rlp.go
const (
	YieldAutomatic = iota
	YieldDisabled
	YieldClaimable
)

// StateAccount is the Ethereum consensus representation of accounts.
// These objects are stored in the main account trie.
type StateAccountLegacy struct {
	Nonce    uint64
	Balance  *big.Int
	Root     common.Hash // merkle root of the storage trie
	CodeHash []byte
}

// Ethereum account representation with restaking ability
type StateAccount struct {
	Nonce uint64
	Flags uint8 // valid values: 0-2

	// raw representation
	Fixed *big.Int

	// share representation
	Shares    *big.Int
	Remainder *big.Int

	Root     common.Hash // merkle root of the storage trie
	CodeHash []byte
}

// NewEmptyStateAccount constructs an empty state account.
func NewEmptyStateAccount() *StateAccount {
	return &StateAccount{
		Fixed:     new(big.Int),
		Shares:    new(big.Int),
		Remainder: new(big.Int),
		Root:      EmptyRootHash,
		CodeHash:  EmptyCodeHash.Bytes(),
	}
}

// SlimAccount is a modified version of an Account, where the root is replaced
// with a byte slice. This format can be used to represent full-consensus format
// or slim format which replaces the empty root and code hash as nil byte slice.
type SlimAccount struct {
	Nonce     uint64
	Flags     uint8
	Fixed     *big.Int
	Shares    *big.Int
	Remainder *big.Int
	Root      []byte // Nil if root equals to types.EmptyRootHash
	CodeHash  []byte // Nil if hash equals to types.EmptyCodeHash
}

// SlimAccountRLP encodes the state account in 'slim RLP' format.
func SlimAccountRLP(account StateAccount) []byte {
	slim := SlimAccount{
		Nonce:     account.Nonce,
		Flags:     account.Flags,
		Fixed:     account.Fixed,
		Shares:    account.Shares,
		Remainder: account.Remainder,
	}
	if account.Root != EmptyRootHash {
		slim.Root = account.Root[:]
	}
	if !bytes.Equal(account.CodeHash, EmptyCodeHash[:]) {
		slim.CodeHash = account.CodeHash
	}
	data, err := rlp.EncodeToBytes(slim)
	if err != nil {
		panic(err)
	}
	return data
}

// FullAccount decodes the data on the 'slim RLP' format and returns
// the consensus format account.
func FullAccount(data []byte) (*StateAccount, error) {
	var slim SlimAccount
	if err := rlp.DecodeBytes(data, &slim); err != nil {
		return nil, err
	}
	account := StateAccount{
		Nonce:     slim.Nonce,
		Flags:     slim.Flags,
		Fixed:     slim.Fixed,
		Shares:    slim.Shares,
		Remainder: slim.Remainder,
	}

	// Interpret the storage root and code hash in slim format.
	if len(slim.Root) == 0 {
		account.Root = EmptyRootHash
	} else {
		account.Root = common.BytesToHash(slim.Root)
	}
	if len(slim.CodeHash) == 0 {
		account.CodeHash = EmptyCodeHash[:]
	} else {
		account.CodeHash = slim.CodeHash
	}
	return &account, nil
}

// FullAccountRLP converts data on the 'slim RLP' format into the full RLP-format.
func FullAccountRLP(data []byte) ([]byte, error) {
	account, err := FullAccount(data)
	if err != nil {
		return nil, err
	}
	return rlp.EncodeToBytes(account)
}
