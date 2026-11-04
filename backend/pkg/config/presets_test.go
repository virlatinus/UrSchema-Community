package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPresets(t *testing.T) {
	dir := t.TempDir()
	pw := filepath.Join(dir, "db_pass")
	file := filepath.Join(dir, "connections.json")
	os.WriteFile(pw, []byte("s3cret\n"), 0600)
	os.WriteFile(file, []byte(`[{"name":"local","type":"mysql","host":"mysql","user":"root","password_file":"`+pw+`"}]`), 0600)
	t.Cleanup(func() { presets = nil })

	if err := LoadPresets(file); err != nil {
		t.Fatal(err)
	}
	p := findPreset("preset:local")
	if p == nil || p.Password != "s3cret" || p.Port != 3306 || !p.Preset {
		t.Fatalf("got %+v", p)
	}

	cs := &ConnectionStore{}
	if err := cs.SaveConnection(&SavedConnection{ID: "preset:local", Name: "x"}); err == nil || !strings.Contains(err.Error(), "URSCHEMA_CONNECTIONS_FILE") {
		t.Fatalf("save: %v", err)
	}
	if err := cs.DeleteConnection("preset:local"); err == nil {
		t.Fatal("delete should be refused")
	}
}

func TestLoadPresetsRejectsBadEntries(t *testing.T) {
	dir := t.TempDir()
	for _, body := range []string{
		`[{"name":"a","type":"oracle","host":"h"}]`,
		`[{"name":"a","type":"mysql","host":"h"},{"name":"a","type":"mysql","host":"h"}]`,
		`[{"name":"a","type":"mysql","host":"h","password":"inline"}]`,
		`[{"name":"a","type":"mysql","host":"h","password_file":"/does/not/exist"}]`,
	} {
		file := filepath.Join(dir, "c.json")
		os.WriteFile(file, []byte(body), 0600)
		if err := LoadPresets(file); err == nil {
			t.Errorf("%s: want an error", body)
		}
	}
}
