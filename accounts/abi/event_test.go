// Copyright 2016 The go-ethereum Authors
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

package abi

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

var jsonEventTransfer = []byte(`{
  "anonymous": false,
  "inputs": [
    {
      "indexed": true, "name": "from", "type": "address"
    }, {
      "indexed": true, "name": "to", "type": "address"
    }, {
      "indexed": false, "name": "value", "type": "uint256"
  }],
  "name": "Transfer",
  "type": "event"
}`)

var jsonEventPledge = []byte(`{
  "anonymous": false,
  "inputs": [{
      "indexed": false, "name": "who", "type": "address"
    }, {
      "indexed": false, "name": "wad", "type": "uint128"
    }, {
      "indexed": false, "name": "currency", "type": "bytes3"
  }],
  "name": "Pledge",
  "type": "event"
}`)

// LogStaticArray(uint[3] indexed a, uint[3] b, string c);
var jsonEventStaticArray = []byte(`{
  "anonymous": false,
  "inputs": [{
      "indexed": true, "name": "a", "type": "uint256[3]"
    }, {
      "indexed": false, "name": "b", "type": "uint256[3]"
    }, {
      "indexed": false, "name": "c", "type": "string"
  }],
  "name": "LogStaticArray",
  "type": "event"
}`)

// 1000000
var transferData1 = "00000000000000000000000000000000000000000000000000000000000f4240"

// "0x00Ce0d46d924CC8437c806721496599FC3FFA268", 2218516807680, "usd"
var pledgeData1 = "00000000000000000000000000ce0d46d924cc8437c806721496599fc3ffa2680000000000000000000000000000000000000000000000000000020489e800007573640000000000000000000000000000000000000000000000000000000000"

// LogStaticArray([uint(1),2,3], [uint(4),5,6], "abc");
// topics: [ '0x5bd247ff7033ae96613f65c11f8074e9b2f92187afa39bf5386ae114538c0393', '0x6e0c627900b24bd432fe7b1f713f1b0744091a646a9fe4a65a18dfed21f2949c' ]
var staticArrayEventData = "000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000050000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000036162630000000000000000000000000000000000000000000000000000000000"

func TestEventId(t *testing.T) {
	var table = []struct {
		definition   string
		expectations map[string]common.Hash
	}{
		{
			definition: `[
			{ "type" : "event", "name" : "balance", "inputs": [{ "name" : "in", "type": "uint256" }] },
			{ "type" : "event", "name" : "check", "inputs": [{ "name" : "t", "type": "address" }, { "name": "b", "type": "uint256" }] }
			]`,
			expectations: map[string]common.Hash{
				"balance": crypto.Keccak256Hash([]byte("balance(uint256)")),
				"check":   crypto.Keccak256Hash([]byte("check(address,uint256)")),
			},
		},
	}

	for _, test := range table {
		abi, err := JSON(strings.NewReader(test.definition))
		if err != nil {
			t.Fatal(err)
		}

		for name, event := range abi.Events {
			if event.Id() != test.expectations[name] {
				t.Errorf("expected id to be %x, got %x", test.expectations[name], event.Id())
			}
		}
	}
}

func TestEventTupleUnpack(t *testing.T) {

	type EventTransfer struct {
		Value *big.Int
	}

	type EventPledge struct {
		Who      common.Address
		Wad      *big.Int
		Currency [3]byte
	}

	type BadEventPledge struct {
		Who      string
		Wad      int
		Currency [3]byte
	}

	bigint := new(big.Int)
	bigintExpected := big.NewInt(1000000)
	bigintExpected2 := big.NewInt(2218516807680)
	addr := common.HexToAddress("0x00Ce0d46d924CC8437c806721496599FC3FFA268")
	var testCases = []struct {
		data     string
		dest     interface{}
		expected interface{}
		jsonLog  []byte
		error    string
		name     string
	}{{
		transferData1,
		&EventTransfer{},
		&EventTransfer{Value: bigintExpected},
		jsonEventTransfer,
		"",
		"Can unpack ERC20 Transfer event into structure",
	}, {
		transferData1,
		&[]interface{}{&bigint},
		&[]interface{}{&bigintExpected},
		jsonEventTransfer,
		"",
		"Can unpack ERC20 Transfer event into slice",
	}, {
		pledgeData1,
		&EventPledge{},
		&EventPledge{
			addr,
			bigintExpected2,
			[3]byte{'u', 's', 'd'}},
		jsonEventPledge,
		"",
		"Can unpack Pledge event into structure",
	}, {
		pledgeData1,
		&[]interface{}{&common.Address{}, &bigint, &[3]byte{}},
		&[]interface{}{
			&addr,
			&bigintExpected2,
			&[3]byte{'u', 's', 'd'}},
		jsonEventPledge,
		"",
		"Can unpack Pledge event into slice",
	}, {
		pledgeData1,
		&[3]interface{}{&common.Address{}, &bigint, &[3]byte{}},
		&[3]interface{}{
			&addr,
			&bigintExpected2,
			&[3]byte{'u', 's', 'd'}},
		jsonEventPledge,
		"",
		"Can unpack Pledge event into an array",
	}, {
		pledgeData1,
		&[]interface{}{new(int), 0, 0},
		&[]interface{}{},
		jsonEventPledge,
		"abi: cannot unmarshal common.Address in to int",
		"Can not unpack Pledge event into slice with wrong types",
	}, {
		pledgeData1,
		&BadEventPledge{},
		&BadEventPledge{},
		jsonEventPledge,
		"abi: cannot unmarshal common.Address in to string",
		"Can not unpack Pledge event into struct with wrong filed types",
	}, {
		pledgeData1,
		&[]interface{}{common.Address{}, new(big.Int)},
		&[]interface{}{},
		jsonEventPledge,
		"abi: insufficient number of elements in the list/array for unpack, want 3, got 2",
		"Can not unpack Pledge event into too short slice",
	}, {
		pledgeData1,
		new(map[string]interface{}),
		&[]interface{}{},
		jsonEventPledge,
		"abi: cannot unmarshal tuple into map[string]interface {}",
		"Can not unpack Pledge event into map",
	}, {
		staticArrayEventData,
		&[3]interface{}{&[3]*big.Int{}, new(string)},
		&[3]interface{}{
			&[3]*big.Int{big.NewInt(4), big.NewInt(5), big.NewInt(6)},
			strPtr("abc")},
		jsonEventStaticArray,
		"",
		"Can unpack LogStaticArray event into an array",
	}}

	for _, tc := range testCases {
		assert := assert.New(t)
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := unpackTestEventData(tc.dest, tc.data, tc.jsonLog, assert)
			if tc.error == "" {
				assert.Nil(err, "Should be able to unpack event data.")
				assert.Equal(tc.expected, tc.dest)
			} else {
				assert.EqualError(err, tc.error)
			}
		})
	}
}

func unpackTestEventData(dest interface{}, hexData string, jsonEvent []byte, assert *assert.Assertions) error {
	data, err := hex.DecodeString(hexData)
	assert.NoError(err, "Hex data should be a correct hex-string")
	var e Event
	assert.NoError(json.Unmarshal(jsonEvent, &e), "Should be able to unmarshal event ABI")
	a := ABI{Events: map[string]Event{"e": e}}
	return a.Unpack(dest, "e", data)
}

func strPtr(s string) *string {
	return &s
}
