package db

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// DBType represents the type of database
type DBType string

const (
	MySQL      DBType = "mysql"
	PostgreSQL DBType = "postgres"
	SQLite     DBType = "sqlite"
)

// ConnectionConfig holds database connection configuration
type ConnectionConfig struct {
	DBType   DBType `json:"dbType"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
	FilePath string `json:"filePath"` // For SQLite
}

// DBConnection represents a database connection interface
type DBConnection interface {
	// Connect establishes a database connection
	Connect(config ConnectionConfig) error

	// Close closes the database connection
	Close() error

	// Ping checks if the connection is alive
	Ping() error

	// Query executes a query that returns rows
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)

	// QueryRow executes a query that is expected to return at most one row
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row

	// Exec executes a query that doesn't return rows
	Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)

	// GetDB returns the underlying *sql.DB
	GetDB() *sql.DB

	// GetDBType returns the database type
	GetDBType() DBType

	// GetConfig returns the connection configuration
	GetConfig() ConnectionConfig

	// SelectDatabase returns a connection bound to database. It doesn't
	// change the receiver, so concurrent requests for different databases
	// on the same connection can't run against each other's database.
	SelectDatabase(database string) (DBConnection, error)
}

// databasePools holds one pool per extra database of a server connection
// (MySQL, PostgreSQL), opened on first use and closed with the connection.
type databasePools struct {
	mu    sync.Mutex
	pools map[string]*sql.DB
}

// selectDatabase returns parent itself for its own database, otherwise a
// databaseView on the pool for database, opening it with open if needed.
func (dp *databasePools) selectDatabase(parent DBConnection, base *BaseConnection, database string, open func(ConnectionConfig) (*sql.DB, error)) (DBConnection, error) {
	if database == "" || database == base.config.Database {
		return parent, nil
	}
	if base.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	cfg := base.config
	cfg.Database = database

	dp.mu.Lock()
	defer dp.mu.Unlock()
	pool, ok := dp.pools[database]
	if !ok {
		var err error
		if pool, err = open(cfg); err != nil {
			return nil, err
		}
		if dp.pools == nil {
			dp.pools = make(map[string]*sql.DB)
		}
		dp.pools[database] = pool
	}
	return &databaseView{BaseConnection: BaseConnection{db: pool, dbType: base.dbType, config: cfg}, parent: parent}, nil
}

func (dp *databasePools) closeAll() {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	for _, pool := range dp.pools {
		pool.Close()
	}
	dp.pools = nil
}

// databaseView is a server connection bound to one database. Its pool
// belongs to the parent connection, which also closes it.
type databaseView struct {
	BaseConnection
	parent DBConnection
}

func (v *databaseView) Connect(ConnectionConfig) error {
	return fmt.Errorf("cannot reconnect a database view")
}

func (v *databaseView) Close() error { return nil }

func (v *databaseView) SelectDatabase(database string) (DBConnection, error) {
	return v.parent.SelectDatabase(database)
}

// BaseConnection provides common functionality for all database connections
type BaseConnection struct {
	db     *sql.DB
	dbType DBType
	config ConnectionConfig
}

// GetDB returns the underlying *sql.DB
func (bc *BaseConnection) GetDB() *sql.DB {
	return bc.db
}

// GetDBType returns the database type
func (bc *BaseConnection) GetDBType() DBType {
	return bc.dbType
}

// GetConfig returns the connection configuration
func (bc *BaseConnection) GetConfig() ConnectionConfig {
	return bc.config
}

// Close closes the database connection
func (bc *BaseConnection) Close() error {
	if bc.db != nil {
		return bc.db.Close()
	}
	return nil
}

// Ping checks if the connection is alive
func (bc *BaseConnection) Ping() error {
	if bc.db == nil {
		return fmt.Errorf("database connection is nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return bc.db.PingContext(ctx)
}

// Query executes a query that returns rows
func (bc *BaseConnection) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if bc.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}
	return bc.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that is expected to return at most one row
func (bc *BaseConnection) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if bc.db == nil {
		return nil
	}
	return bc.db.QueryRowContext(ctx, query, args...)
}

// Exec executes a query that doesn't return rows
func (bc *BaseConnection) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if bc.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}
	return bc.db.ExecContext(ctx, query, args...)
}

// SetConnectionPool sets connection pool settings
func (bc *BaseConnection) SetConnectionPool(maxOpen, maxIdle int, maxLifetime time.Duration) {
	if bc.db != nil {
		bc.db.SetMaxOpenConns(maxOpen)
		bc.db.SetMaxIdleConns(maxIdle)
		bc.db.SetConnMaxLifetime(maxLifetime)
	}
}

// NewConnection creates a new database connection based on the database type
func NewConnection(dbType DBType) (DBConnection, error) {
	switch dbType {
	case MySQL:
		return &MySQLConnection{}, nil
	case PostgreSQL:
		return &PostgreSQLConnection{}, nil
	case SQLite:
		return &SQLiteConnection{}, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}
