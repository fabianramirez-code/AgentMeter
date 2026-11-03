package main

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// decodeKeyringSecret undoes the prefixes go-keyring adds so the secret
// survives text-only tools (`security -w`, `secret-tool`); anything else is
// returned as is.
func decodeKeyringSecret(s string) ([]byte, error) {
	switch {
	case strings.HasPrefix(s, "go-keyring-base64:"):
		return base64.StdEncoding.DecodeString(strings.TrimPrefix(s, "go-keyring-base64:"))
	case strings.HasPrefix(s, "go-keyring-encoded:"):
		return hex.DecodeString(strings.TrimPrefix(s, "go-keyring-encoded:"))
	default:
		return []byte(s), nil
	}
}
