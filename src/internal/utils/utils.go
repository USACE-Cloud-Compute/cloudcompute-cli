package utils

import (
	"encoding/json"
	"fmt"
	"os"
)

func ReadJson[T any](jsonpath string) (*T, error) {
	file, err := os.Open(jsonpath)
	if err != nil {
		return nil, err
	}
	var t T
	err = json.NewDecoder(file).Decode(&t)
	return &t, err
}

func PelSliceToMap(inputs []any) ([]map[string]string, error) {
	results := make([]map[string]string, len(inputs))
	for i, input := range inputs {
		if m, ok := input.(map[string]any); ok {
			n := make(map[string]string)
			for key, val := range m {
				n[key] = fmt.Sprintf("%s", val)
			}
			results[i] = n
		} else {
			return nil, fmt.Errorf("invalid input.  Array must contain map[string]any")
		}
	}
	return results, nil
}
