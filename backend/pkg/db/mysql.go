package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLConnection represents a MySQL database connection
type MySQLConnection struct {
	BaseConnection
	databasePools
}

// Connect establishes a MySQL database connection
func (mc *MySQLConnection) Connect(config ConnectionConfig) error {
	mc.config = config
	mc.dbType = MySQL

	db, err := openMySQL(config)
	if err != nil {
		return err
	}
	mc.db = db
	return nil
}

func openMySQL(config ConnectionConfig) (*sql.DB, error) {
	// Build connection string with multi-statement support
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&multiStatements=true",
		config.Username,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
	)

	// Open connection
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	// Set connection pool settings (optimized for desktop single-user app)
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(10 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping MySQL database: %w", err)
	}
	return db, nil
}

// SelectDatabase returns a connection whose pool has database in its DSN.
func (mc *MySQLConnection) SelectDatabase(database string) (DBConnection, error) {
	return mc.selectDatabase(mc, &mc.BaseConnection, database, openMySQL)
}

// Close closes the connection and every per-database pool.
func (mc *MySQLConnection) Close() error {
	mc.closeAll()
	return mc.BaseConnection.Close()
}
