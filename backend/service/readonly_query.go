package service

import (
	"context"
	"database/sql"
	"time"

	"sqlmanager/pkg/db"
	"sqlmanager/pkg/sqlguard"
)

// ExecuteSQLReadOnly runs a SQL editor query in read-only mode. The query
// must pass sqlguard.CheckReadOnly, and then runs inside a read-only
// transaction (PostgreSQL, MySQL) or with PRAGMA query_only (SQLite), so
// side effects the keyword check can't see (data-modifying CTEs,
// EXPLAIN ANALYZE of a write) are still refused by the database.
func (ts *TableService) ExecuteSQLReadOnly(conn db.DBConnection, sqlQuery string) (*QueryResult, error) {
	if err := sqlguard.CheckReadOnly(conn.GetDBType(), sqlQuery); err != nil {
		return nil, err
	}

	startTime := time.Now()
	result := &QueryResult{Success: true}
	fail := func(err error) (*QueryResult, error) {
		result.Success = false
		result.Error = err.Error()
		result.ExecutionTime = float64(time.Since(startTime).Milliseconds())
		return result, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c, err := conn.GetDB().Conn(ctx)
	if err != nil {
		return fail(err)
	}
	defer c.Close()

	var rows *sql.Rows
	if conn.GetDBType() == db.SQLite {
		// modernc.org/sqlite ignores TxOptions.ReadOnly; query_only is
		// per-connection, so reset it before the conn returns to the pool.
		if _, err := c.ExecContext(ctx, "PRAGMA query_only = ON"); err != nil {
			return fail(err)
		}
		defer c.ExecContext(context.Background(), "PRAGMA query_only = OFF")
		rows, err = c.QueryContext(ctx, sqlQuery)
	} else {
		tx, txErr := c.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if txErr != nil {
			return fail(txErr)
		}
		defer tx.Rollback()
		rows, err = tx.QueryContext(ctx, sqlQuery)
	}
	if err != nil {
		return fail(err)
	}
	defer rows.Close()

	data, err := readAllRows(rows)
	if err != nil {
		return fail(err)
	}
	result.Data = data
	result.ExecutionTime = float64(time.Since(startTime).Milliseconds())
	return result, nil
}

// readAllRows reads every row of rows into a single-page TableData.
func readAllRows(rows *sql.Rows) (*TableData, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var data [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}
		row := make([]interface{}, len(columns))
		for i, v := range values {
			row[i] = safeConvertValue(v)
		}
		data = append(data, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &TableData{
		Columns:  columns,
		Rows:     data,
		Total:    int64(len(data)),
		Page:     1,
		PageSize: len(data),
	}, nil
}
