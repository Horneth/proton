package patch

import (
	"strconv"
	"strings"
)

// Set nested map value using dot-notation.
// e.g., Set(data, "a.b.c", 1) results in {"a": {"b": {"c": 1}}}
func Set(data map[string]interface{}, path string, value interface{}) {
	parts := strings.Split(path, ".")
	curr := data

	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if next, ok := curr[part].(map[string]interface{}); ok {
			curr = next
		} else {
			// If path doesn't exist or is not a map, create it
			newMap := make(map[string]interface{})
			curr[part] = newMap
			curr = newMap
		}
	}

	last := parts[len(parts)-1]
	curr[last] = value
}

// ParseValue attempts to parse strings into typed values (bool, int)
// If it fails, it returns the original string.
func ParseValue(s string) interface{} {
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return s
}
