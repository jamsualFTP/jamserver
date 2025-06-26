package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

func FormatFileList(files []string) string {
	var formattedList strings.Builder
	for _, file := range files {
		// Format each file entry
		// Example: "-rw-r--r-- 1 owner group 1024 Jan 01 00:00 filename"
		formattedList.WriteString(fmt.Sprintf("-rw-r--r-- 1 owner group 0 Jan 01 00:00 %s\r\n", file))
	}
	return formattedList.String()
}

func ScanAndUpdateChildren(dirPath string, children map[string]interface{}) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		filePath := filepath.Join(dirPath, file.Name())
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return err
		}

		// prepare metadata for this file/directory
		metadata := map[string]interface{}{
			"type":          "file",
			"size":          fileInfo.Size(),
			"last_modified": fileInfo.ModTime().Unix(),
		}

		if file.IsDir() {
			// if its a directory, create a nested children map
			metadata["type"] = "directory"
			metadata["children"] = make(map[string]interface{})

			// recursively scan subdirectory
			subChildren, ok := metadata["children"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("failed to create children for directory: %s", file.Name())
			}
			if err := ScanAndUpdateChildren(filePath, subChildren); err != nil {
				return err
			}

			// if no children, remove the children key
			if len(subChildren) == 0 {
				delete(metadata, "children")
			}
		}

		children[file.Name()] = metadata
	}

	return nil
}
