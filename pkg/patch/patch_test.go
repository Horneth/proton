package patch

import (
	"reflect"
	"testing"
)

func TestSet(t *testing.T) {
	tests := []struct {
		name     string
		initial  map[string]interface{}
		path     string
		value    interface{}
		expected map[string]interface{}
	}{
		{
			name:    "simple set",
			initial: make(map[string]interface{}),
			path:    "a",
			value:   1,
			expected: map[string]interface{}{
				"a": 1,
			},
		},
		{
			name:    "nested set",
			initial: make(map[string]interface{}),
			path:    "a.b.c",
			value:   "val",
			expected: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "val",
					},
				},
			},
		},
		{
			name: "update existing",
			initial: map[string]interface{}{
				"a": map[string]interface{}{
					"b": 1,
				},
			},
			path:  "a.b",
			value: 2,
			expected: map[string]interface{}{
				"a": map[string]interface{}{
					"b": 2,
				},
			},
		},
		{
			name: "add to existing map",
			initial: map[string]interface{}{
				"a": map[string]interface{}{
					"b": 1,
				},
			},
			path:  "a.c",
			value: 2,
			expected: map[string]interface{}{
				"a": map[string]interface{}{
					"b": 1,
					"c": 2,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Set(tt.initial, tt.path, tt.value)
			if !reflect.DeepEqual(tt.initial, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, tt.initial)
			}
		})
	}
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"true", true},
		{"false", false},
		{"123", 123},
		{"-456", -456},
		{"hello", "hello"},
		{"123.45", "123.45"}, // We only support int for now
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseValue(tt.input)
			if got != tt.expected {
				t.Errorf("expected %v (%T), got %v (%T)", tt.expected, tt.expected, got, got)
			}
		})
	}
}
