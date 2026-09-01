package xdr

import (
	"bytes"
	"encoding/json"
	"errors"
)

// ZeroCopyRawMessage is a json.RawMessage that prevents encoding/json from
// allocating a copy. The caller must guarantee the underlying buffer outlives
// the message or is copied before use.
type ZeroCopyRawMessage []byte

func (z *ZeroCopyRawMessage) UnmarshalJSON(b []byte) error {
	*z = b
	return nil
}

// UnescapeJSONString extracts the unescaped string from a JSON string token
// without allocating if there are no escape sequences.
func UnescapeJSONString(raw []byte) ([]byte, error) {
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return nil, errors.New("not a JSON string")
	}

	inner := raw[1 : len(raw)-1]

	// If there are no escape characters, we can just return the subslice zero-copy
	if bytes.IndexByte(inner, '\\') == -1 {
		return inner, nil
	}

	// Fallback to standard json unmarshal for escaped strings
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return []byte(s), nil
}
