package accesslog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// jsonBody renders a captured body as raw JSON when it already is valid
// JSON (the common case for this API), falling back to a quoted string
// otherwise. An empty body is omitted from the encoded entry.
type jsonBody []byte

func (b jsonBody) MarshalJSON() ([]byte, error) {
	if json.Valid(b) {
		return b, nil
	}
	return json.Marshal(string(b))
}

// entry is the structured record logged at LevelHeaders and LevelDump.
type entry struct {
	Time            time.Time   `json:"time"`
	Method          string      `json:"method"`
	Path            string      `json:"path"`
	Proto           string      `json:"proto"`
	RemoteAddr      string      `json:"remoteAddr"`
	RequestHeaders  http.Header `json:"requestHeaders"`
	RequestBody     jsonBody    `json:"requestBody,omitempty"`
	StatusCode      int         `json:"statusCode"`
	Duration        string      `json:"duration"`
	ResponseHeaders http.Header `json:"responseHeaders"`
	ResponseBody    jsonBody    `json:"responseBody,omitempty"`
}

// dumpHandler logs request/response headers (and, when includeBody is set,
// the request payload and response body) around the wrapped handler.
func dumpHandler(next http.Handler, out io.Writer, format DumpFormat, includeBody bool) http.Handler {
	// Headers-only mode (-vvv, no -dump) always renders as a plain-text,
	// curl -v style dump; the json/text choice only applies to -dump,
	// which additionally includes the request/response bodies.
	if !includeBody {
		format = DumpFormatText
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e := &entry{
			Time:           time.Now(),
			Method:         r.Method,
			Path:           r.URL.RequestURI(),
			Proto:          r.Proto,
			RemoteAddr:     r.RemoteAddr,
			RequestHeaders: redactHeaders(r.Header, "Cookie"),
		}

		if includeBody && r.Body != nil {
			body, err := io.ReadAll(r.Body)
			_ = r.Body.Close()
			if err == nil {
				e.RequestBody = jsonBody(redactSecrets(body))
				r.Body = io.NopCloser(bytes.NewReader(body))
			}
		}

		rec := &recorder{ResponseWriter: w, statusCode: http.StatusOK, captureBody: includeBody}
		start := time.Now()
		next.ServeHTTP(rec, r)

		e.StatusCode = rec.statusCode
		e.Duration = time.Since(start).String()
		e.ResponseHeaders = redactHeaders(w.Header(), "Set-Cookie")
		if includeBody {
			e.ResponseBody = jsonBody(redactSecrets(rec.body.Bytes()))
		}

		writeEntry(out, format, e)
	})
}

// redactSecrets masks the values of JSON object fields whose name contains
// "password" or "secret" (the login password, database passwords in the
// connection form and in saved-connection responses), at any depth. Bodies
// that aren't JSON, or contain no such field, are returned unchanged.
func redactSecrets(body []byte) []byte {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil || !redactValue(v) {
		return body
	}
	out, err := json.Marshal(v)
	if err != nil {
		return body
	}
	return out
}

// redactValue masks secret fields in place and reports whether it changed
// anything.
func redactValue(v any) bool {
	changed := false
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			name := strings.ToLower(k)
			if s, ok := val.(string); ok && s != "" && (strings.Contains(name, "password") || strings.Contains(name, "secret")) {
				t[k] = "[redacted]"
				changed = true
				continue
			}
			if redactValue(val) {
				changed = true
			}
		}
	case []any:
		for _, val := range t {
			if redactValue(val) {
				changed = true
			}
		}
	}
	return changed
}

// redactHeaders clones h with the named header's values masked, so
// session cookies don't end up in log files.
func redactHeaders(h http.Header, name string) http.Header {
	c := h.Clone()
	if vals := c.Values(name); len(vals) > 0 {
		c.Del(name)
		for range vals {
			c.Add(name, "[redacted]")
		}
	}
	return c
}

// recorder wraps a http.ResponseWriter to capture the status code and,
// optionally, a copy of the response body as it is written through.
type recorder struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
	captureBody bool
	body        bytes.Buffer
}

func (r *recorder) WriteHeader(statusCode int) {
	if !r.wroteHeader {
		r.statusCode = statusCode
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *recorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	if r.captureBody {
		r.body.Write(b)
	}
	return r.ResponseWriter.Write(b)
}

func writeEntry(out io.Writer, format DumpFormat, e *entry) {
	if format == DumpFormatText {
		writeEntryText(out, e)
		return
	}
	_ = json.NewEncoder(out).Encode(e)
}

// writeEntryText renders e in the style of `curl -v`: request line and
// headers prefixed with "> ", response status line and headers prefixed
// with "< ", and informational lines (timing, remote address) prefixed
// with "* ".
func writeEntryText(out io.Writer, e *entry) {
	var b strings.Builder

	fmt.Fprintf(&b, "* %s from %s\n", e.Time.Format(time.RFC3339), e.RemoteAddr)
	fmt.Fprintf(&b, "> %s %s %s\n", e.Method, e.Path, e.Proto)
	writeHeadersText(&b, "> ", e.RequestHeaders)
	if len(e.RequestBody) > 0 {
		fmt.Fprintf(&b, ">\n%s\n", e.RequestBody)
	}

	fmt.Fprintf(&b, "< %s %d %s\n", e.Proto, e.StatusCode, http.StatusText(e.StatusCode))
	writeHeadersText(&b, "< ", e.ResponseHeaders)
	if len(e.ResponseBody) > 0 {
		fmt.Fprintf(&b, "<\n%s\n", e.ResponseBody)
	}
	fmt.Fprintf(&b, "* Done in %s\n", e.Duration)

	io.WriteString(out, b.String())
}

func writeHeadersText(b *strings.Builder, prefix string, headers http.Header) {
	for name, values := range headers {
		for _, value := range values {
			fmt.Fprintf(b, "%s%s: %s\n", prefix, name, value)
		}
	}
}
