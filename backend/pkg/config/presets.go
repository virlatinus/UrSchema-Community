package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

// presetIDPrefix marks connections from URSCHEMA_CONNECTIONS_FILE. Their ID
// is derived from the name so it stays the same across restarts.
const presetIDPrefix = "preset:"

// presets are loaded once at start-up, before the server runs, and never
// change afterwards, so they need no locking.
var presets []*SavedConnection

// LoadPresets reads the connections file, a JSON list such as
//
//	[{"name": "local", "type": "mysql", "host": "mysql", "port": 3306,
//	  "user": "root", "password_file": "/run/secrets/db_pass"}]
//
// Passwords are read now, so a missing file stops the server at start-up.
func LoadPresets(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var entries []struct {
		Name         string `json:"name"`
		Type         string `json:"type"`
		Host         string `json:"host"`
		Port         int    `json:"port"`
		User         string `json:"user"`
		PasswordFile string `json:"password_file"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&entries); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	defaultPorts := map[string]int{"mysql": 3306, "postgres": 5432}
	seen := map[string]bool{}
	var loaded []*SavedConnection
	for i, e := range entries {
		switch {
		case e.Name == "":
			return fmt.Errorf("connection %d: name is required", i+1)
		case seen[e.Name]:
			return fmt.Errorf("connection %q is listed twice", e.Name)
		case defaultPorts[e.Type] == 0:
			return fmt.Errorf("connection %q: type must be mysql or postgres", e.Name)
		case e.Host == "":
			return fmt.Errorf("connection %q: host is required", e.Name)
		}
		seen[e.Name] = true
		if e.Port == 0 {
			e.Port = defaultPorts[e.Type]
		}
		password := ""
		if e.PasswordFile != "" {
			b, err := os.ReadFile(e.PasswordFile)
			if err != nil {
				return fmt.Errorf("connection %q: reading password_file: %w", e.Name, err)
			}
			password = strings.TrimSpace(string(b))
		}
		loaded = append(loaded, &SavedConnection{
			ID:       presetIDPrefix + e.Name,
			Name:     e.Name,
			DBType:   e.Type,
			Host:     e.Host,
			Port:     e.Port,
			Username: e.User,
			Password: password,
			Preset:   true,
		})
	}
	presets = loaded
	log.Printf("[ConnectionStore] loaded %d connection(s) from %s", len(loaded), path)
	return nil
}

func findPreset(id string) *SavedConnection {
	for _, p := range presets {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// errPreset refuses changes to a connection the file defines.
func errPreset(id string) error {
	return fmt.Errorf("connection %q comes from URSCHEMA_CONNECTIONS_FILE and can't be changed here", strings.TrimPrefix(id, presetIDPrefix))
}
