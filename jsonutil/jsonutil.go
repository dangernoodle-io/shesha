// Package jsonutil provides shesha's canonical JSON encode/decode helpers:
// Marshal/Unmarshal wrapping encoding/json with consistent, context-rich
// error messages, plus an Indent variant for human-readable output. Typed
// GetJSON/SetJSON helpers layer this over the store.Store seam (MC-15), so
// consumers stop hand-encoding JSON blobs into store string values.
//
// An HTTP JSON-response helper is deliberately out of scope here — that's
// httpx.WriteJSON (MC-6), which calls Marshal internally, keeping this
// package encoding/json-pure for its non-HTTP consumers (store, testkit,
// hooks).
package jsonutil

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNotFound is returned by GetJSON when the underlying store has no value
// for the requested key.
var ErrNotFound = errors.New("jsonutil: key not found")

// Marshal encodes v as compact JSON, wrapping any encoding/json error with
// the value's type for context.
func Marshal(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("jsonutil: marshal %T: %w", v, err)
	}

	return b, nil
}

// MarshalIndent encodes v as JSON indented two spaces per nesting level,
// wrapping any encoding/json error with the value's type for context.
func MarshalIndent(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("jsonutil: marshal indent %T: %w", v, err)
	}

	return b, nil
}

// Unmarshal decodes data into v, wrapping any encoding/json error with v's
// type for context.
func Unmarshal(data []byte, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("jsonutil: unmarshal into %T: %w", v, err)
	}

	return nil
}
