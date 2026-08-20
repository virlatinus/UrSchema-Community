package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sqlmanager/handler/rpc"
	"sqlmanager/pkg/accesslog"
	"sqlmanager/pkg/db"
	"syscall"
	"time"

	"github.com/DemonZack/simplejrpc-go/core"
)

func main() {
	var vFlag, vvFlag, vvvFlag bool
	var dump dumpFlag

	flag.BoolVar(&vFlag, "v", false, "log HTTP requests (Apache common log format)")
	flag.BoolVar(&vvFlag, "vv", false, "log HTTP requests (Apache combined log format)")
	flag.BoolVar(&vvvFlag, "vvv", false, "log HTTP requests (combined log format + request/response headers)")
	flag.Var(&dump, "dump", `same as -vvv, plus the full request payload and response body; format is "text" or "json" (default "json"), e.g. -dump=text`)
	flag.Parse()

	logOpts := accesslog.Options{}
	switch {
	case vvvFlag:
		logOpts.Level = accesslog.LevelHeaders
	case vvFlag:
		logOpts.Level = accesslog.LevelCombined
	case vFlag:
		logOpts.Level = accesslog.LevelCommon
	}
	if dump.set {
		logOpts.Level = accesslog.LevelDump
		logOpts.DumpFormat = accesslog.DumpFormat(dump.format)
	}

	// Frontend static assets live at <root>/app/www, next to the backend
	// binary at <root>/app/bin/<exe> (see backend/Makefile package layout).
	wwwDir := "../www"
	if exe, err := os.Executable(); err == nil {
		wwwDir = filepath.Join(filepath.Dir(exe), "..", "www")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	// Start idle connection cleanup (checks every 5 minutes, evicts connections unused for 15 minutes)
	manager := db.GetManager()
	manager.StartCleanupRoutine(5*time.Minute, 15*time.Minute)

	// Create the RPC server and register services
	server := rpc.NewServer()

	// Listen for shutdown signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		core.Container.Log().Info("received shutdown signal, shutting down gracefully...")

		// Close all database connections
		manager.Shutdown()
		core.Container.Log().Info("all database connections closed")

		os.Exit(0)
	}()

	// Start the server (blocking)
	log.Printf("UrSchema starting on %s (serving frontend from %s)", addr, wwwDir)
	if err := server.Start(addr, wwwDir, logOpts); err != nil {
		core.Container.Log().Error(fmt.Sprintf("failed to start HTTP server: %v", err))
		return
	}
}

// dumpFlag implements flag.Value for -dump, which is a boolean-like flag
// that also accepts an optional format: bare "-dump" defaults to JSON,
// while "-dump=text" or "-dump=json" pick the format explicitly.
type dumpFlag struct {
	set    bool
	format string
}

func (d *dumpFlag) String() string { return d.format }

func (d *dumpFlag) Set(value string) error {
	format := value
	if format == "" || format == "true" {
		format = "json"
	}
	if format != "text" && format != "json" {
		return fmt.Errorf("invalid -dump format %q (want %q or %q)", value, "text", "json")
	}
	d.set = true
	d.format = format
	return nil
}

// IsBoolFlag lets the flag package treat "-dump" as not requiring a
// following argument, so bare "-dump" is accepted (see flag.Value docs).
func (d *dumpFlag) IsBoolFlag() bool { return true }
