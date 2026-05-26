package auth

import (
	"fmt"
	"math/big"
	"strings"
)

const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var base62Indexes = func() [256]int16 {
	var m [256]int16
	for i := range m {
		m[i] = -1
	}
	for i := 0; i < len(base62Alphabet); i++ {
		m[base62Alphabet[i]] = int16(i)
	}
	return m
}()

// Base62Encode encodes a byte slice to a base62 string.
func Base62Encode(src []byte) string {
	if len(src) == 0 {
		return ""
	}

	leadingZeroes := 0
	for leadingZeroes < len(src) && src[leadingZeroes] == 0 {
		leadingZeroes++
	}

	x := new(big.Int).SetBytes(src)
	if x.Sign() == 0 {
		return strings.Repeat(string(base62Alphabet[0]), leadingZeroes)
	}

	base := big.NewInt(62)
	zero := big.NewInt(0)
	mod := new(big.Int)

	out := make([]byte, 0, len(src)*2)
	for x.Cmp(zero) > 0 {
		x.DivMod(x, base, mod)
		out = append(out, base62Alphabet[mod.Int64()])
	}

	for i := 0; i < leadingZeroes; i++ {
		out = append(out, base62Alphabet[0])
	}

	reverseBytes(out)
	return string(out)
}

// Base62Decode decodes a base62 string back to a byte slice.
func Base62Decode(s string) ([]byte, error) {
	if s == "" {
		return []byte{}, nil
	}

	leadingZeroes := 0
	for leadingZeroes < len(s) && s[leadingZeroes] == base62Alphabet[0] {
		leadingZeroes++
	}

	x := big.NewInt(0)
	base := big.NewInt(62)

	for i := 0; i < len(s); i++ {
		idx := base62Indexes[s[i]]
		if idx < 0 {
			return nil, fmt.Errorf("auth: invalid base62 character %q", s[i])
		}
		x.Mul(x, base)
		x.Add(x, big.NewInt(int64(idx)))
	}

	decoded := x.Bytes()
	if leadingZeroes == 0 {
		return decoded, nil
	}

	out := make([]byte, leadingZeroes+len(decoded))
	copy(out[leadingZeroes:], decoded)
	return out, nil
}

func reverseBytes(b []byte) {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
}
