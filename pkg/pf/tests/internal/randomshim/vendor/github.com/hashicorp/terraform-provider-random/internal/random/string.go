// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package random

import (
	"crypto/rand"
	"math/big"
	"sort"
)

type StringParams struct {
	Length          int64
	Upper           bool
	MinUpper        int64
	Lower           bool
	MinLower        int64
	Numeric         bool
	MinNumeric      int64
	Special         bool
	MinSpecial      int64
	OverrideSpecial string
}

func CreateString(input StringParams) ([]byte, error) {
	const numChars = "0123456789"
	const lowerChars = "abcdefghijklmnopqrstuvwxyz"
	const upperChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	var specialChars = "!@#$%&*()-_=+[]{}<>:?"
	var result []byte

	if input.OverrideSpecial != "" {
		specialChars = input.OverrideSpecial
	}

	var chars = ""
	if input.Upper {
		chars += upperChars
	}
	if input.Lower {
		chars += lowerChars
	}
	if input.Numeric {
		chars += numChars
	}
	if input.Special {
		chars += specialChars
	}

	minMapping := map[string]int64{
		numChars:     input.MinNumeric,
		lowerChars:   input.MinLower,
		upperChars:   input.MinUpper,
		specialChars: input.MinSpecial,
	}

	result = make([]byte, 0, input.Length)

	for k, v := range minMapping {
		s, err := generateRandomBytes(&k, v)
		if err != nil {
			return nil, err
		}
		result = append(result, s...)
	}

	s, err := generateRandomBytes(&chars, input.Length-int64(len(result)))
	if err != nil {
		return nil, err
	}

	result = append(result, s...)

	order := make([]byte, len(result))
	if _, err := rand.Read(order); err != nil {
		return nil, err
	}

	sort.Slice(result, func(i, j int) bool {
		return order[i] < order[j]
	})

	return result, nil
}

func generateRandomBytes(charSet *string, length int64) ([]byte, error) {
	bytes := make([]byte, length)
	setLen := big.NewInt(int64(len(*charSet)))
	for i := range bytes {
		idx, err := rand.Int(rand.Reader, setLen)
		if err != nil {
			return nil, err
		}
		bytes[i] = (*charSet)[idx.Int64()]
	}
	return bytes, nil
}
