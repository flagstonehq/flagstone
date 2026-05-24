package auth

import (
	"bytes"
	"testing"
)

func TestBase62RoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"all zeros", []byte{0, 0, 0, 0}},
		{"leading zeros", []byte{0, 0, 1, 2, 3, 4, 250, 255}},
		{"single byte", []byte{255}},
		{"32 bytes", bytes.Repeat([]byte{42}, 32)},
		{"empty", nil},
		{"empty slice", []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := Base62Encode(tt.input)

			decoded, err := Base62Decode(encoded)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			if !bytes.Equal(tt.input, decoded) {
				t.Fatalf("round trip mismatch: got %x, want %x", decoded, tt.input)
			}
		})
	}
}

func TestBase62EmptyEncode(t *testing.T) {
	if got := Base62Encode(nil); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
	if got := Base62Encode([]byte{}); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestBase62InvalidChar(t *testing.T) {
	invalid := []string{"abc+123", "-test", "ab/c", "a b"}
	for _, s := range invalid {
		_, err := Base62Decode(s)
		if err == nil {
			t.Errorf("expected error for input %q", s)
		}
	}
}
