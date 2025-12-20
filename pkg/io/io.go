package io

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

// ReadData reads data from various sources:
// - "-" : Read from Stdin (raw)
// - "@path" : Read from file at path (raw)
// - "base64:..." : Decode base64 literal (optional prefix)
// - "string" : Treat as literal data
// If isBase64 is true, the raw bytes read from the source are decoded as base64.
func ReadData(input string, isBase64 bool) ([]byte, error) {
	var rawData []byte
	var err error

	switch {
	case input == "-":
		rawData, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read from stdin: %v", err)
		}
	case strings.HasPrefix(input, "@"):
		path := strings.TrimPrefix(input, "@")
		rawData, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %v", path, err)
		}
	default:
		// Remove "base64:" prefix if present for convenience
		if strings.HasPrefix(input, "base64:") {
			input = strings.TrimPrefix(input, "base64:")
			isBase64 = true
		}
		rawData = []byte(input)

		// Smart autodetection: if not explicitly a file/stdin and not already marked as base64,
		// try decoding it as base64 if it looks like one.
		if !isBase64 && len(input) > 0 {
			// Basic heuristic: check if it's long enough and has base64-like characters,
			// and attempt decoding. If it fails, we treat it as raw text.
			if decoded, err := base64.StdEncoding.DecodeString(input); err == nil {
				// To avoid false positives for very short strings (like "root"),
				// we only auto-decode if it's long (>16 chars) or contains padding "="
				if len(input) > 16 || strings.HasSuffix(input, "=") {
					return decoded, nil
				}
			}
		}
	}

	if isBase64 {
		// Try standard decoding, then URL decoding if it fails
		decoded, err := base64.StdEncoding.DecodeString(string(rawData))
		if err != nil {
			decoded, err = base64.URLEncoding.DecodeString(string(rawData))
			if err != nil {
				return nil, fmt.Errorf("failed to decode base64: %v", err)
			}
		}
		return decoded, nil
	}

	return rawData, nil
}
