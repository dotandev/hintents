// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// canonicalJSON produces a deterministic JSON representation by:
// 1. Sorting all object keys alphabetically
// 2. Using consistent formatting (no extra whitespace)
// 3. Ensuring stable serialization across platforms
func canonicalJSON(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "")

	if err := encodeCanonical(&buf, v); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// encodeCanonical recursively encodes values in canonical form
func encodeCanonical(buf *bytes.Buffer, v interface{}) error {
	switch val := v.(type) {
	case map[string]interface{}:
		return encodeObject(buf, val)
	case []interface{}:
		return encodeArray(buf, val)
	case string:
		encoded, err := json.Marshal(val)
		if err != nil {
			return err
		}
		buf.Write(encoded)
		return nil
	case float64, bool, nil:
		encoded, err := json.Marshal(val)
		if err != nil {
			return err
		}
		buf.Write(encoded)
		return nil
	default:
		// For structs and other types, marshal to map first
		jsonBytes, err := json.Marshal(val)
		if err != nil {
			return err
		}
		var intermediate interface{}
		if err := json.Unmarshal(jsonBytes, &intermediate); err != nil {
			return err
		}
		return encodeCanonical(buf, intermediate)
	}
}

// encodeObject encodes a map with sorted keys
func encodeObject(buf *bytes.Buffer, obj map[string]interface{}) error {
	buf.WriteByte('{')

	// Sort keys for deterministic output
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}

		// Encode key
		keyJSON, err := json.Marshal(k)
		if err != nil {
			return err
		}
		buf.Write(keyJSON)
		buf.WriteByte(':')

		// Encode value
		if err := encodeCanonical(buf, obj[k]); err != nil {
			return err
		}
	}

	buf.WriteByte('}')
	return nil
}

// encodeArray encodes an array
func encodeArray(buf *bytes.Buffer, arr []interface{}) error {
	buf.WriteByte('[')

	for i, item := range arr {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := encodeCanonical(buf, item); err != nil {
			return err
		}
	}

	buf.WriteByte(']')
	return nil
}

// marshalCanonical converts a struct to canonical JSON bytes
func marshalCanonical(v interface{}) ([]byte, error) {
	// First convert to generic interface{} via standard JSON
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("initial marshal failed: %w", err)
	}

	var intermediate interface{}
	if err := json.Unmarshal(jsonBytes, &intermediate); err != nil {
		return nil, fmt.Errorf("unmarshal to interface failed: %w", err)
	}

	// Then apply canonical encoding
	return canonicalJSON(intermediate)
}
