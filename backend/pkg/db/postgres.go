package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// PostgreSQLConnection represents a PostgreSQL database connection
type PostgreSQLConnection struct {
	BaseConnection
	databasePools
}

// Connect establishes a PostgreSQL database connection
func (pc *PostgreSQLConnection) Connect(config ConnectionConfig) error {
	pc.config = config
	pc.dbType = PostgreSQL

	db, err := openPostgres(config)
	if err != nil {
		return err
	}
	pc.db = db
	return nil
}

func openPostgres(config ConnectionConfig) (*sql.DB, error) {
	// Use default 'postgres' database if not specified
	dbName := config.Database
	if dbName == "" {
		dbName = "postgres"
	}

	// Build connection string with SSL disabled for local development
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Host,
		config.Port,
		config.Username,
		config.Password,
		dbName,
	)

	// Open connection
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL connection: %w", err)
	}

	// Set connection pool settings (optimized for desktop single-user app)
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(10 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL database: %w", err)
	}
	return db, nil
}

// SelectDatabase returns a connection to database. PostgreSQL fixes the
// database per session, so each database gets its own pool.
func (pc *PostgreSQLConnection) SelectDatabase(database string) (DBConnection, error) {
	return pc.selectDatabase(pc, &pc.BaseConnection, database, openPostgres)
}

// Close closes the connection and every per-database pool.
func (pc *PostgreSQLConnection) Close() error {
	pc.closeAll()
	return pc.BaseConnection.Close()
}
