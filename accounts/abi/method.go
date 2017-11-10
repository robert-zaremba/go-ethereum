// Copyright 2015 The go-ethereum Authors
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
	"fmt"
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// Method represents a callable given a `Name` and whether the method is a constant.
// If the method is `Const` no transaction needs to be created for this
// particular Method call. It can easily be simulated using a local VM.
// For example a `Balance()` method only needs to retrieve something
// from the storage and therefor requires no Tx to be send to the
// network. A method such as `Transact` does require a Tx and thus will
// be flagged `true`.
// Input specifies the required input parameters for this gives method.
type Method struct {
	Name    string
	Const   bool
	Inputs  []Argument
	Outputs []Argument
}

func (m Method) pack(args ...interface{}) ([]byte, error) {
	// Make sure arguments match up and pack them
	if len(args) != len(m.Inputs) {
		return nil, fmt.Errorf("argument count mismatch: %d for %d", len(args), len(m.Inputs))
	}
	// variable input is the output appended at the end of packed
	// output. This is used for strings and bytes types input.
	var variableInput []byte

	var ret []byte
	for i, a := range args {
		input := m.Inputs[i]
		// pack the input
		packed, err := input.Type.pack(reflect.ValueOf(a))
		if err != nil {
			return nil, fmt.Errorf("`%s` %v", m.Name, err)
		}

		// check for a slice type (string, bytes, slice)
		if input.Type.requiresLengthPrefix() {
			// calculate the offset
			offset := len(m.Inputs)*32 + len(variableInput)
			// set the offset
			ret = append(ret, packNum(reflect.ValueOf(offset))...)
			// Append the packed output to the variable input. The variable input
			// will be appended at the end of the input.
			variableInput = append(variableInput, packed...)
		} else {
			// append the packed value to the input
			ret = append(ret, packed...)
		}
	}
	// append the variable input at the end of the packed input
	ret = append(ret, variableInput...)

	return ret, nil
}

// unpacks a method return tuple into a struct of corresponding go types
//
// Unpacking can be done into a struct or a slice/array.
func (m Method) tupleUnpack(v interface{}, output []byte) error {
	// make sure the passed value is a pointer
	valueOf := reflect.ValueOf(v)
	if reflect.Ptr != valueOf.Kind() {
		return fmt.Errorf("abi: Unpack(non-pointer %T)", v)
	}

	var (
		value = valueOf.Elem()
		typ   = value.Type()
		kind  = value.Kind()
	)
	if err := requireUnpackKind(value, typ, kind, m.Outputs, false); err != nil {
		return err
	}

	j := 0
	for i, o := range m.Outputs {
		if o.Type.T == ArrayTy {
			// need to move this up because they read sequentially
			j += o.Type.Size
		}
		marshalledValue, err := toGoType((i+j)*32, o.Type, output)
		if err != nil {
			return err
		}
		reflectValue := reflect.ValueOf(marshalledValue)

		switch kind {
		case reflect.Struct:
			for j := 0; j < typ.NumField(); j++ {
				field := typ.Field(j)
				// TODO read tags: `abi:"fieldName"`
				if field.Name == strings.ToUpper(o.Name[:1])+o.Name[1:] {
					if err := set(value.Field(j), reflectValue, o); err != nil {
						return err
					}
				}
			}
		case reflect.Slice, reflect.Array:
			v := value.Index(i)
			if err := requireAssignable(v, reflectValue); err != nil {
				return err
			}
			if err := set(v.Elem(), reflectValue, o); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m Method) isTupleReturn() bool { return len(m.Outputs) > 1 }

func (m Method) singleUnpack(v interface{}, output []byte) error {
	// make sure the passed value is a pointer
	valueOf := reflect.ValueOf(v)
	if reflect.Ptr != valueOf.Kind() {
		return fmt.Errorf("abi: Unpack(non-pointer %T)", v)
	}

	value := valueOf.Elem()

	marshalledValue, err := toGoType(0, m.Outputs[0].Type, output)
	if err != nil {
		return err
	}
	return set(value, reflect.ValueOf(marshalledValue), m.Outputs[0])
}

// Sig returns the methods string signature according to the ABI spec.
//
// Example
//
//     function foo(uint32 a, int b)    =    "foo(uint32,int256)"
//
// Please note that "int" is substitute for its canonical representation "int256"
func (m Method) Sig() string {
	types := make([]string, len(m.Inputs))
	for i, input := range m.Inputs {
		types[i] = input.Type.String()
	}
	return fmt.Sprintf("%v(%v)", m.Name, strings.Join(types, ","))
}

func (m Method) String() string {
	inputs := make([]string, len(m.Inputs))
	for i, input := range m.Inputs {
		inputs[i] = fmt.Sprintf("%v %v", input.Name, input.Type)
	}
	outputs := make([]string, len(m.Outputs))
	for i, output := range m.Outputs {
		if len(output.Name) > 0 {
			outputs[i] = fmt.Sprintf("%v ", output.Name)
		}
		outputs[i] += output.Type.String()
	}
	constant := ""
	if m.Const {
		constant = "constant "
	}
	return fmt.Sprintf("function %v(%v) %sreturns(%v)", m.Name, strings.Join(inputs, ", "), constant, strings.Join(outputs, ", "))
}

// Id returns the canonical representation of the method's signature used by the
// abi definition to identify the method name.
func (m Method) Id() []byte {
	return crypto.Keccak256([]byte(m.Sig()))[:4]
}
