package secret

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFromEnv(t *testing.T) {
	file := filepath.Join(t.TempDir(), "pw")
	if err := os.WriteFile(file, []byte("from-file\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, value, path, want string
		wantErr                 bool
	}{
		{name: "unset"},
		{name: "env only", value: "from-env", want: "from-env"},
		{name: "file only, trailing newline trimmed", path: file, want: "from-file"},
		{name: "both set", value: "from-env", path: file, wantErr: true},
		{name: "missing file", path: filepath.Join(t.TempDir(), "nope"), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TEST_SECRET", tc.value)
			t.Setenv("TEST_SECRET_FILE", tc.path)
			got, err := FromEnv("TEST_SECRET")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
