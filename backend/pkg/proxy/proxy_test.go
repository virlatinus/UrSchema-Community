package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrap(t *testing.T) {
	trusted, err := Parse("10.0.0.0/8, 192.168.1.5")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		trusted    Trusted
		peer       string
		xff, proto string
		wantAddr   string
		wantHTTPS  bool
	}{
		{"trusted proxy", trusted, "10.1.2.3:4000", "203.0.113.7", "https", "203.0.113.7", true},
		{"single trusted IP", trusted, "192.168.1.5:4000", "203.0.113.7", "", "203.0.113.7", false},
		{"forged hop left of the real client", trusted, "10.1.2.3:4000", "1.1.1.1, 203.0.113.7", "", "203.0.113.7", false},
		{"chain of trusted proxies", trusted, "10.1.2.3:4000", "203.0.113.7, 10.9.9.9", "", "203.0.113.7", false},
		{"garbage hop", trusted, "10.1.2.3:4000", "nonsense, 203.0.113.7", "", "203.0.113.7", false},
		{"untrusted peer", trusted, "198.51.100.1:4000", "203.0.113.7", "https", "198.51.100.1:4000", false},
		{"no proxies configured", nil, "198.51.100.1:4000", "203.0.113.7", "https", "198.51.100.1:4000", true},
		{"trusted proxy, no header", trusted, "10.1.2.3:4000", "", "", "10.1.2.3:4000", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotAddr string
			var gotHTTPS bool
			h := tt.trusted.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAddr, gotHTTPS = r.RemoteAddr, HTTPS(r)
			}))
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tt.peer
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.proto != "" {
				r.Header.Set("X-Forwarded-Proto", tt.proto)
			}
			h.ServeHTTP(httptest.NewRecorder(), r)
			if gotAddr != tt.wantAddr || gotHTTPS != tt.wantHTTPS {
				t.Fatalf("got %q https=%v, want %q https=%v", gotAddr, gotHTTPS, tt.wantAddr, tt.wantHTTPS)
			}
		})
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := Parse("10.0.0.0/8 nope"); err == nil {
		t.Fatal("want an error")
	}
}
