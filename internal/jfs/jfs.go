package jfs

import (
	"fmt"
	"jamserver/pkg/utils"
	"os"
	"path/filepath"
	"time"
)

// interesting: https://github.com/1pkg/gopium/issues/24
type FileMetadata struct {
	Children     map[string]FileMetadata `json:"children,omitempty"`      // children (only for dirs ofc)
	LastModified time.Time               `json:"last_modified,omitempty"` // timestamp of last modification
	Created      time.Time               `json:"created,omitempty"`       // timestamp of file creation
	Owner        string                  `json:"owner,omitempty"`         // just owner
	Type         string                  `json:"type"`                    // file or directiory
	Permissions  os.FileMode             `json:"permissions,omitempty"`   // actually uint32 WOW
}

// update the existing JSON structure with directory contents
func UpdateFileSystemMetadata(basePath string, jsonPath string) error {
	var fileSystem map[string]interface{}
	fileSystem, err := utils.LoadJSON[map[string]interface{}](jsonPath)
	if err != nil {
		return fmt.Errorf("reading JSON file error: %v", err)
	}

	root, ok := fileSystem["root"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid JSON structure: missing 'root' key")
	}

	children, ok := root["children"].(map[string]interface{})
	if !ok {
		root["children"] = make(map[string]interface{})
		children = root["children"].(map[string]interface{})
	}

	// Scan the directory and update children
	if err := utils.ScanAndUpdateChildren(basePath, children); err != nil {
		return fmt.Errorf("scanning directory error: %v", err)
	}

	// Save the updated JSON
	if err := utils.SaveJSON(jsonPath, fileSystem); err != nil {
		return fmt.Errorf("writing JSON file error: %v", err)
	}

	return nil
}

// NOTE: actual file system initialization my friends
func InitializeFS(basePath string) error {
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		if err := os.MkdirAll(basePath, 0755); err != nil {
			return fmt.Errorf("creating base path error: %v", err)
		}
	}

	// update the filesystem json with the current directory structure
	err := UpdateFileSystemMetadata(basePath, "app/filesystem.json")
	if err != nil {
		return fmt.Errorf("updating filesystem JSON error: %v", err)
	}

	// add some immersion
	time.Sleep(time.Second / 3)
	fmt.Println("File system metadata initialized successfully.")
	time.Sleep(time.Second / 2)
	fmt.Println("File system initialized successfully.")

	return nil
}

type FileSystem struct {
	BasePath string
}

func NewFileSystem(basePath string) *FileSystem {
	return &FileSystem{BasePath: basePath}
}

// lists all files in the given directory
func (fs *FileSystem) ListFiles() ([]string, error) {
	files, err := os.ReadDir(fs.BasePath)
	if err != nil {
		return nil, err
	}

	var fileNames []string
	for _, file := range files {
		fileNames = append(fileNames, file.Name())
	}
	return fileNames, nil
}

// reads a file's contents
func (fs *FileSystem) ReadFile(fileName string) ([]byte, error) {
	return os.ReadFile(filepath.Join(fs.BasePath, fileName))
}

// writes data to a file
func (fs *FileSystem) WriteFile(fileName string, data []byte) error {
	return os.WriteFile(filepath.Join(fs.BasePath, fileName), data, 0644)
}
