// Package accesslog provides Apache-style HTTP access logging middleware
// with four levels of verbosity: common log format, combined log format,
// combined log format plus request/response headers, and a full dump of
// headers plus the request payload and response body.
package accesslog

import (
	"io"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
)

// Level selects how much detail is logged for each HTTP request.
type Level int

const (
	// LevelNone disables access logging entirely.
	LevelNone Level = iota
	// LevelCommon logs the Apache Common Log Format (-v).
	LevelCommon
	// LevelCombined logs the Apache Combined Log Format (-vv).
	LevelCombined
	// LevelHeaders logs the combined format plus request/response headers (-vvv).
	LevelHeaders
	// LevelDump logs the combined format, headers, and the request/response
	// bodies (-dump).
	LevelDump
)

// DumpFormat selects how the -dump payload is rendered.
type DumpFormat string

const (
	DumpFormatJSON DumpFormat = "json"
	DumpFormatText DumpFormat = "text"
)

// Options configures the access log middleware.
type Options struct {
	// Level is the verbosity level to log at. LevelNone disables the
	// middleware, so Wrap returns the handler unchanged.
	Level Level
	// DumpFormat selects the rendering of headers/body entries logged at
	// LevelHeaders and LevelDump. Defaults to DumpFormatJSON.
	DumpFormat DumpFormat
	// Output is where log lines are written. Defaults to os.Stdout.
	Output io.Writer
}

// Wrap returns h wrapped with Apache-style access logging according to opts.
func Wrap(h http.Handler, opts Options) http.Handler {
	if opts.Level == LevelNone {
		return h
	}

	out := opts.Output
	if out == nil {
		out = os.Stdout
	}

	format := opts.DumpFormat
	if format == "" {
		format = DumpFormatJSON
	}

	handler := h
	if opts.Level >= LevelHeaders {
		handler = dumpHandler(handler, out, format, opts.Level == LevelDump)
	}

	switch {
	case opts.Level == LevelCommon:
		handler = handlers.LoggingHandler(out, handler)
	case opts.Level >= LevelCombined:
		handler = handlers.CombinedLoggingHandler(out, handler)
	}

	return handler
}
