package files

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Exists checks whether a file or directory exists
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

// IsDir checks whether the path is a directory
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// IsFile checks whether the path is a file
func IsFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// ReadFile reads file contents
func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile writes file contents
func WriteFile(path string, data []byte, perm os.FileMode) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if !Exists(dir) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, perm)
}

// ReadJSON reads a JSON file and unmarshals it into a struct
func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// WriteJSON marshals a struct to JSON and writes it to a file
func WriteJSON(path string, v any, indent bool) error {
	var data []byte
	var err error

	if indent {
		data, err = json.MarshalIndent(v, "", "  ")
	} else {
		data, err = json.Marshal(v)
	}

	if err != nil {
		return err
	}

	return WriteFile(path, data, 0644)
}

// EnsureDir ensures a directory exists, creating it if necessary
func EnsureDir(path string) error {
	if !Exists(path) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}

// Remove deletes a file or directory
func Remove(path string) error {
	return os.RemoveAll(path)
}

// Copy copies a file
func Copy(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return WriteFile(dst, data, 0644)
}

// ListDir lists all files and subdirectories under a directory
func ListDir(path string) ([]os.FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return f.Readdir(-1)
}

// GetFileSize returns the file size in bytes
func GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
