package provider

import (
	"encoding/json"
	"strings"
)

// PruneJSON traverses the parsed JSON structure and removes noise fields.
func PruneJSON(data []byte, noiseKeys []string) ([]byte, error) {
	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}

	parsed = prune(parsed, noiseKeys)

	return json.MarshalIndent(parsed, "", "  ")
}

func prune(v interface{}, noiseKeys []string) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			if isNoise(k, noiseKeys) {
				delete(val, k)
			} else {
				val[k] = prune(child, noiseKeys)
			}
		}
	case []interface{}:
		for i, child := range val {
			val[i] = prune(child, noiseKeys)
		}
	}
	return v
}

func isNoise(key string, noiseKeys []string) bool {
	lowerKey := strings.ToLower(key)
	for _, noise := range noiseKeys {
		if lowerKey == noise {
			return true
		}
	}
	return false
}
