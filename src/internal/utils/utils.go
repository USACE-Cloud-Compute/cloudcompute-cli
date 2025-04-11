package utils

import (
	"encoding/json"
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
