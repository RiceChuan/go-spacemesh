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

package util

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// UnmarshalFixedJSON decodes the input as a string with 0x prefix. The length of out
// determines the required input length. This function is commonly used to implement the
// UnmarshalJSON method for fixed-size types.
func UnmarshalFixedJSON(typ reflect.Type, input, out []byte) error {
	if len(input) < 2 || input[0] != '"' || input[len(input)-1] != '"' { // check for quoted string
		return &json.UnmarshalTypeError{Value: "non-string", Type: typ}
	}
	return wrapTypeError(UnmarshalFixedText(typ.String(), input[1:len(input)-1], out), typ)
}

// UnmarshalFixedText decodes the input as a string with 0x prefix. The length of out
// determines the required input length. This function is commonly used to implement the
// UnmarshalText method for fixed-size types.
func UnmarshalFixedText(typename string, input, out []byte) error {
	raw, err := checkText(input, true)
	if err != nil {
		return err
	}
	if len(raw)/2 != len(out) {
		return fmt.Errorf("hex string has length %d, want %d for %s", len(raw), len(out)*2, typename)
	}
	// Pre-verify syntax before modifying out.
	for _, b := range raw {
		if decodeNibble(b) == badNibble {
			return errSyntax
		}
	}
	hex.Decode(out, raw)
	return nil
}

func bytesHave0xPrefix(input []byte) bool {
	return len(input) >= 2 && input[0] == '0' && (input[1] == 'x' || input[1] == 'X')
}

func checkText(input []byte, wantPrefix bool) ([]byte, error) {
	if len(input) == 0 {
		return nil, nil // empty strings are allowed
	}
	if bytesHave0xPrefix(input) {
		input = input[2:]
	} else if wantPrefix {
		return nil, errMissingPrefix
	}
	if len(input)%2 != 0 {
		return nil, errOddLength
	}
	return input, nil
}

func wrapTypeError(err error, typ reflect.Type) error {
	switch {
	case errors.Is(err, errSyntax):
		return &json.UnmarshalTypeError{Value: err.Error(), Type: typ}
	case errors.Is(err, errMissingPrefix):
		return &json.UnmarshalTypeError{Value: err.Error(), Type: typ}
	case errors.Is(err, errOddLength):
		return &json.UnmarshalTypeError{Value: err.Error(), Type: typ}
	case errors.Is(err, errUint64Range):
		return &json.UnmarshalTypeError{Value: err.Error(), Type: typ}
	default:
		return err
	}
}

const badNibble = ^uint64(0)

func decodeNibble(in byte) uint64 {
	switch {
	case in >= '0' && in <= '9':
		return uint64(in - '0')
	case in >= 'A' && in <= 'F':
		return uint64(in - 'A' + 10)
	case in >= 'a' && in <= 'f':
		return uint64(in - 'a' + 10)
	default:
		return badNibble
	}
}

func Base64Encode(src []byte) []byte {
	n := base64.StdEncoding.EncodedLen(len(src))
	dst := make([]byte, n)
	base64.StdEncoding.Encode(dst, src)
	return dst
}

func Base64Decode(dst, src []byte) error {
	n, err := base64.StdEncoding.Decode(dst, src)
	if err != nil {
		return err
	}
	if n != len(dst) {
		return fmt.Errorf("incomplete decoding: %d != %d", n, len(src))
	}
	return err
}
