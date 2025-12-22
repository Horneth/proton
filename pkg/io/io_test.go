package io

import (
	"bytes"
	"encoding/base64"
	"os"
	"testing"
)

func TestReadData(t *testing.T) {
	// Create a temp file for testing
	content := []byte("hello world")
	tmpfile, err := os.CreateTemp("", "proton-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	tests := []struct {
		name     string
		input    string
		isBase64 bool
		expected []byte
		wantErr  bool
	}{
		{
			name:     "literal string",
			input:    "hello",
			expected: []byte("hello"),
		},
		{
			name:     "file reference",
			input:    "@" + tmpfile.Name(),
			expected: content,
		},
		{
			name:     "explicit base64 prefix",
			input:    "base64:" + base64.StdEncoding.EncodeToString(content),
			expected: content,
		},
		{
			name:     "base64 flag",
			input:    base64.StdEncoding.EncodeToString(content),
			isBase64: true,
			expected: content,
		},
		{
			name:     "autodetect base64 (long string)",
			input:    base64.StdEncoding.EncodeToString([]byte("this is a long enough string for autodetection")),
			expected: []byte("this is a long enough string for autodetection"),
		},
		{
			name:     "no autodetect for short string",
			input:    "root",
			expected: []byte("root"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadData(tt.input, tt.isBase64)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("ReadData() got = %v, want %v", string(got), string(tt.expected))
			}
		})
	}
}
