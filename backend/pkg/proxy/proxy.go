// Package proxy finds the real client of a request that arrived through
// one of the reverse proxies listed in URSCHEMA_TRUSTED_PROXIES.
package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"unicode"
)

// Trusted is the set of proxy addresses whose X-Forwarded-For and
// X-Forwarded-Proto headers are believed.
type Trusted []netip.Prefix

// Parse reads a comma- or space-separated list of CIDRs or single IPs.
func Parse(s string) (Trusted, error) {
	var t Trusted
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || unicode.IsSpace(r) }) {
		p, err := netip.ParsePrefix(f)
		if err != nil {
			a, aerr := netip.ParseAddr(f)
			if aerr != nil {
				return nil, fmt.Errorf("invalid trusted proxy %q (want a CIDR such as 10.0.0.0/8, or an IP)", f)
			}
			p = netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen())
		}
		t = append(t, p.Masked())
	}
	return t, nil
}

func (t Trusted) contains(a netip.Addr) bool {
	a = a.Unmap()
	for _, p := range t {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

type httpsKey struct{}

// Wrap passes requests on to h. When the direct peer is a trusted proxy,
// r.RemoteAddr becomes the client address from X-Forwarded-For (without a
// port), so login throttling and the access log see the real client.
//
// It also records whether the client used HTTPS (see HTTPS). Without any
// trusted proxies configured, X-Forwarded-Proto is believed from every
// peer, as before this setting existed: a forged header only gives the
// forger a Secure cookie their own browser won't send back over HTTP.
func (t Trusted) Wrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fromProxy := false
		if ap, err := netip.ParseAddrPort(r.RemoteAddr); err == nil {
			fromProxy = t.contains(ap.Addr())
		}
		https := (len(t) == 0 || fromProxy) && r.Header.Get("X-Forwarded-Proto") == "https"
		r = r.WithContext(context.WithValue(r.Context(), httpsKey{}, https))
		if fromProxy {
			if client := t.forwardedClient(r.Header); client != "" {
				r.RemoteAddr = client
			}
		}
		h.ServeHTTP(w, r)
	})
}

// forwardedClient walks X-Forwarded-For from the right, past our own
// proxies, and returns the first address they didn't add themselves.
// Anything left of it was written by the client and can't be trusted.
func (t Trusted) forwardedClient(header http.Header) string {
	var hops []string
	for _, v := range header.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(v, ",")...)
	}
	client := ""
	for i := len(hops) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break
		}
		client = a.Unmap().String()
		if !t.contains(a) {
			break
		}
	}
	return client
}

// HTTPS reports whether the client reached us over HTTPS, directly or
// through a proxy whose X-Forwarded-Proto Wrap believed.
func HTTPS(r *http.Request) bool {
	https, _ := r.Context().Value(httpsKey{}).(bool)
	return r.TLS != nil || https
}
