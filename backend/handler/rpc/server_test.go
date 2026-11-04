package rpc

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithBasePath(t *testing.T) {
	h := withBasePath("/db-admin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	tests := []struct{ path, wantBody, wantLocation string }{
		{"/db-admin/api/ping", "/api/ping", ""}, // proxy kept the prefix
		{"/api/ping", "/api/ping", ""},          // proxy stripped it
		{"/db-admin/", "/", ""},
		{"/db-admin?x=1", "", "/db-admin/?x=1"},
		{"/db-adminx/api", "/db-adminx/api", ""}, // not our prefix
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", tt.path, nil))
		if loc := rec.Header().Get("Location"); loc != tt.wantLocation || (tt.wantLocation == "" && rec.Body.String() != tt.wantBody) {
			t.Errorf("%s: body %q location %q", tt.path, rec.Body.String(), loc)
		}
	}
}
