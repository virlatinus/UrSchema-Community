// Package secret reads settings that may be passed either directly in an
// environment variable or, Docker-secrets style, in a file named by
// <NAME>_FILE, so secrets don't have to live in the environment or next
// to the data they protect.
package secret

import (
	"fmt"
	"os"
	"strings"
)

// FromEnv returns the value of name, or the contents of the file named by
// name+"_FILE" with surrounding whitespace trimmed. It returns "" when
// neither is set, and an error when both are set or the file can't be read.
func FromEnv(name string) (string, error) {
	value, path := os.Getenv(name), os.Getenv(name+"_FILE")
	switch {
	case path == "":
		return value, nil
	case value != "":
		return "", fmt.Errorf("set %s or %s_FILE, not both", name, name)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s_FILE: %w", name, err)
	}
	return strings.TrimSpace(string(data)), nil
}
