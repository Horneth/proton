package io

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

// ReadData reads data from a string or file path.
// If input starts with '@', the rest is treated as a file path.
// If input matches a file on disk but is missing '@', it returns an error.
// Otherwise, the input is treated as literal data.
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
		// Check if it looks like a file but user forgot '@'
		if info, fsErr := os.Stat(input); fsErr == nil && !info.IsDir() {
			return nil, fmt.Errorf("input %q matches a file on disk but is missing '@' prefix. To read from file, use '@%s'", input, input)
		}

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

// EncodeData encodes binary data to string (optionally base64).
func EncodeData(data []byte, asBase64 bool) string {
	if asBase64 {
		return base64.StdEncoding.EncodeToString(data)
	}
	return string(data)
}
