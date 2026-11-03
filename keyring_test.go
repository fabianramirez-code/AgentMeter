package main

import "testing"

func TestDecodeKeyringSecret(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`{"token":{}}`, `{"token":{}}`},
		{"go-keyring-base64:eyJ0b2tlbiI6e319", `{"token":{}}`},
		{"go-keyring-encoded:7b7d", `{}`},
	} {
		got, err := decodeKeyringSecret(tc.in)
		if err != nil {
			t.Fatalf("decodeKeyringSecret(%q): %v", tc.in, err)
		}
		if string(got) != tc.want {
			t.Errorf("decodeKeyringSecret(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
