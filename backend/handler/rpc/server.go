package rpc

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sqlmanager/pkg/accesslog"
	"sqlmanager/pkg/auth"
	"strings"

	"github.com/DemonZack/simplejrpc-go/core"
	"github.com/DemonZack/simplejrpc-go/core/config"
	"github.com/DemonZack/simplejrpc-go/core/gi18n"
	"github.com/DemonZack/simplejrpc-go/net/gsock"
	"github.com/DemonZack/simplejrpc-go/os/gpath"
	"github.com/sourcegraph/jsonrpc2"
)

// Options configures access control and transport for the HTTP server.
type Options struct {
	// Auth is the login guard; nil disables authentication.
	Auth *auth.Guard
	// ReadOnly blocks every method that changes a database (see
	// writeMethods) and restricts db.executeSQL to read-only statements.
	ReadOnly bool
	// TLSCert and TLSKey are PEM file paths; when both are set the server
	// speaks HTTPS instead of HTTP.
	TLSCert, TLSKey string
}

type Server struct {
	handler *gsock.JsonRpcSimpleServiceHandler
	opts    Options
}

// writeMethods are refused outright in read-only mode. db.executeSQL is
// not listed: it is checked per statement (see ExecuteSQL).
var writeMethods = map[string]bool{
	"db.createDatabase": true,
	"db.dropDatabase":   true,
	"db.importDatabase": true,
	"db.createTable":    true,
	"db.alterTable":     true,
	"db.dropTable":      true,
	"db.renameTable":    true,
	"db.updateRow":      true,
	"db.insertRow":      true,
	"db.deleteRow":      true,
	"db.batchModify":    true,
	"db.importCSV":      true,
	"db.importTableSQL": true,
}

// NewServer creates a new RPC server instance
func NewServer(opts Options) *Server {
	return &Server{handler: gsock.NewJsonRpcSimpleServiceHandler(), opts: opts}
}

// RegisterHandles registers all RPC handler functions
func (s *Server) RegisterHandles(ds interface {
	RegisterHandle(name string, handler func(*gsock.Request) (any, error), middlewares ...gsock.RPCMiddleware)
},
) {
	// Base endpoint
	ds.RegisterHandle("ping", s.Ping)

	// Filesystem
	ds.RegisterHandle("fs.listDir", s.ListDir)

	// Connection Management
	ds.RegisterHandle("db.testConnection", s.TestConnection)
	ds.RegisterHandle("db.connect", s.Connect)
	ds.RegisterHandle("db.disconnect", s.Disconnect)
	ds.RegisterHandle("db.listConnections", s.ListConnections)

	// Connection Configuration (Saved Connections)
	ds.RegisterHandle("db.saveConnectionConfig", s.SaveConnectionConfig)
	ds.RegisterHandle("db.listConnectionConfigs", s.ListConnectionConfigs)
	ds.RegisterHandle("db.getConnectionConfig", s.GetConnectionConfig)
	ds.RegisterHandle("db.deleteConnectionConfig", s.DeleteConnectionConfig)

	// Database Operations
	ds.RegisterHandle("db.listDatabases", s.ListDatabases)
	ds.RegisterHandle("db.getDatabaseInfo", s.GetDatabaseInfo)
	ds.RegisterHandle("db.createDatabase", s.CreateDatabase)
	ds.RegisterHandle("db.dropDatabase", s.DropDatabase)
	ds.RegisterHandle("db.switchDatabase", s.SwitchDatabase)
	ds.RegisterHandle("db.exportDatabase", s.ExportDatabase)
	ds.RegisterHandle("db.importDatabase", s.ImportDatabase)
	ds.RegisterHandle("db.listDatabaseDumpTasks", s.ListDatabaseDumpTasks)
	ds.RegisterHandle("db.getDatabaseDumpTask", s.GetDatabaseDumpTask)
	ds.RegisterHandle("db.getDatabaseDumpTaskLogs", s.GetDatabaseDumpTaskLogs)
	ds.RegisterHandle("db.cancelDatabaseDumpTask", s.CancelDatabaseDumpTask)

	// Schema Operations (PostgreSQL only)
	ds.RegisterHandle("db.listSchemas", s.ListSchemas)

	// Table Operations
	ds.RegisterHandle("db.listTables", s.ListTables)
	ds.RegisterHandle("db.getTableSchema", s.GetTableSchema)
	ds.RegisterHandle("db.getTableData", s.GetTableData)

	// Table Designer (DDL)
	ds.RegisterHandle("db.createTable", s.CreateTable)
	ds.RegisterHandle("db.alterTable", s.AlterTable)
	ds.RegisterHandle("db.dropTable", s.DropTable)
	ds.RegisterHandle("db.renameTable", s.RenameTable)
	ds.RegisterHandle("db.getTableDDL", s.GetTableDDL)

	// Query Operations
	ds.RegisterHandle("db.executeSQL", s.ExecuteSQL)

	// Data Modification
	ds.RegisterHandle("db.updateRow", s.UpdateRow)
	ds.RegisterHandle("db.insertRow", s.InsertRow)
	ds.RegisterHandle("db.deleteRow", s.DeleteRow)
	ds.RegisterHandle("db.batchModify", s.BatchModify)

	// Import / Export
	ds.RegisterHandle("db.exportTable", s.ExportTable)
	ds.RegisterHandle("db.exportTableDDL", s.ExportTableDDL)
	ds.RegisterHandle("db.importCSV", s.ImportCSV)
	ds.RegisterHandle("db.importTableSQL", s.ImportTableSQL)

}

// Start begins serving HTTP on addr. RPC methods are dispatched under
// /api/<method>; everything else is served as static frontend assets
// from wwwDir, if present. logOpts controls Apache-style access logging;
// pass accesslog.Options{} (LevelNone) to disable it.
func (s *Server) Start(addr, wwwDir string, logOpts accesslog.Options) error {
	gpath.GmCfgPath = "./"
	gi18n.Instance().SetPath("./i18n")
	core.InitContainer(config.WithConfigEnvFormatterOptionFunc("test"))

	s.RegisterHandles(s.handler)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("/api/auth/login", s.opts.Auth.HandleLogin)
	mux.HandleFunc("/api/auth/logout", s.opts.Auth.HandleLogout)
	mux.Handle("/api/", s.opts.Auth.Require(http.HandlerFunc(s.handleRPC)))

	if info, err := os.Stat(wwwDir); err == nil && info.IsDir() {
		mux.Handle("/", http.FileServer(http.Dir(wwwDir)))
	} else {
		log.Printf("frontend directory %s not found, serving API only", wwwDir)
	}

	// Reject cross-origin POSTs (CSRF), including from sibling subdomains
	// that SameSite cookies would let through. Uses Sec-Fetch-Site, falling
	// back to comparing Origin with Host.
	csrf := http.NewCrossOriginProtection()
	csrf.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": http.StatusForbidden, "msg": "cross-origin request blocked"})
	}))

	srv := &http.Server{Addr: addr, Handler: accesslog.Wrap(csrf.Handler(mux), logOpts)}
	if s.opts.TLSCert != "" && s.opts.TLSKey != "" {
		return srv.ListenAndServeTLS(s.opts.TLSCert, s.opts.TLSKey)
	}
	return srv.ListenAndServe()
}

// handleAuthStatus tells the frontend whether it must show the login
// screen and whether to hide write actions. It is reachable without a
// session so the login screen can be rendered.
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{
		"authRequired":  s.opts.Auth.Enabled(),
		"authenticated": s.opts.Auth.Authenticated(r),
		"readOnly":      s.opts.ReadOnly,
	})
}

// handleRPC bridges a plain HTTP POST request into the JSON-RPC handler
// registry, so the frontend can call backend methods directly without a
// host-provided gateway.
func (s *Server) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	method := strings.TrimPrefix(r.URL.Path, "/api/")

	if s.opts.ReadOnly && writeMethods[method] {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": http.StatusForbidden, "msg": gi18n.Instance().T("ReadOnlyBlocked")})
		return
	}

	var body struct {
		Params json.RawMessage `json:"params"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if len(body.Params) == 0 {
		body.Params = json.RawMessage("{}")
	}

	req := gsock.MakeRequest(
		gsock.WithRequestCtxOption(r.Context()),
		gsock.WithRequestReqOption(&jsonrpc2.Request{Method: method, Params: &body.Params}),
	)

	resp, _ := s.handler.Handle(req)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Ping is a simple health check endpoint
func (s *Server) Ping(req *gsock.Request) (any, error) {
	return "pong", nil
}
