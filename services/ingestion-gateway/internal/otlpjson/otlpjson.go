// Package otlpjson adapts OTLP/JSON to what Go's protojson expects.
//
// The OTLP specification encodes trace_id, span_id and parent_span_id in JSON as lowercase hex
// strings, but they are protobuf `bytes` fields and protojson decodes `bytes` as base64. Feeding
// OTLP/JSON straight to protojson therefore silently produces ids of the wrong length. This
// package rewrites those three fields from hex to base64 before decoding.
package otlpjson

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var idKeys = map[string]struct{}{
	"traceId": {}, "spanId": {}, "parentSpanId": {},
	"trace_id": {}, "span_id": {}, "parent_span_id": {},
}

// HexIDsToBase64 returns data with every trace/span id field converted from hex to base64.
// Numbers are preserved exactly (json.Number), and all other content is untouched.
func HexIDsToBase64(data []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("otlpjson: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("otlpjson: trailing data after JSON value")
	}
	if err := walk(v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

func walk(v any) error {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if _, isID := idKeys[k]; isID {
				if s, ok := val.(string); ok {
					if s == "" {
						continue
					}
					raw, err := hex.DecodeString(s)
					if err != nil {
						return fmt.Errorf("otlpjson: %s is not valid hex: %w", k, err)
					}
					t[k] = base64.StdEncoding.EncodeToString(raw)
					continue
				}
			}
			if err := walk(val); err != nil {
				return err
			}
		}
	case []any:
		for _, e := range t {
			if err := walk(e); err != nil {
				return err
			}
		}
	}
	return nil
}