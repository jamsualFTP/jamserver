package utils

import (
	"encoding/json"
	"fmt"
	"os"
)

func SetScroll() string {
	// WARNING: termporary
	CURSOR_UP_ONE := "\x1b[1A"
	ERASE_LINE := "\x1b[2K"

	return fmt.Sprint(CURSOR_UP_ONE, ERASE_LINE)
}

func LoadJSON[T any](filename string) (T, error) {
	var data T
	fileData, err := os.ReadFile(filename)
	if err != nil {
		return data, err
	}
	return data, json.Unmarshal(fileData, &data)
}

func SaveJSON(filename string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, jsonData, 0644)
}
