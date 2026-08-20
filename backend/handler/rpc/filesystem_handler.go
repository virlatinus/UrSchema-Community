package rpc

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sqlmanager/pkg/rpcutil"
	"strings"

	"github.com/DemonZack/simplejrpc-go/net/gsock"
)

// FileEntry describes a single file or directory returned by ListDir
type FileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

// DirListing is the response for a directory listing request
type DirListing struct {
	Path    string      `json:"path"`
	Parent  string      `json:"parent"`
	Entries []FileEntry `json:"entries"`
}

// ListDir lists the contents of a directory on the backend host, so the
// frontend can offer a server-side file/folder browser for import and
// export paths (the backend has direct filesystem access; the browser does not).
func (s *Server) ListDir(req *gsock.Request) (any, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := rpcutil.ParseParams(req, &args); err != nil {
		return nil, err
	}

	path := args.Path
	if path == "" {
		if home, err := os.UserHomeDir(); err == nil {
			path = home
		} else {
			path = "/"
		}
	}
	path = filepath.Clean(path)

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access path: %w", err)
	}
	if !info.IsDir() {
		path = filepath.Dir(path)
	}

	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("cannot list directory: %w", err)
	}

	entries := make([]FileEntry, 0, len(dirEntries))
	for _, e := range dirEntries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		entries = append(entries, FileEntry{
			Name:  name,
			Path:  filepath.Join(path, name),
			IsDir: e.IsDir(),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	parent := ""
	if dir := filepath.Dir(path); dir != path {
		parent = dir
	}

	return DirListing{Path: path, Parent: parent, Entries: entries}, nil
}
