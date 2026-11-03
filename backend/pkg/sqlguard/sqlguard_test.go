package sqlguard

import (
	"testing"

	"sqlmanager/pkg/db"
)

func TestCheckReadOnly(t *testing.T) {
	cases := []struct {
		name    string
		dbType  db.DBType
		query   string
		allowed bool
	}{
		{"select", db.MySQL, "SELECT * FROM t", true},
		{"select trailing semicolons", db.MySQL, "select 1;;  \n", true},
		{"leading comment", db.PostgreSQL, "-- hi\n/* x */ SELECT 1", true},
		{"with", db.PostgreSQL, "WITH a AS (SELECT 1) SELECT * FROM a", true},
		{"show", db.MySQL, "SHOW TABLES", true},
		{"explain", db.PostgreSQL, "EXPLAIN SELECT 1", true},
		{"desc", db.MySQL, "DESC t", true},
		{"pragma read", db.SQLite, "PRAGMA table_info(t)", true},
		{"semicolon in string", db.MySQL, "SELECT ';'", true},
		{"semicolon in identifier", db.MySQL, "SELECT `a;b` FROM t", true},
		{"doubled quote", db.PostgreSQL, "SELECT 'it''s; fine'", true},
		{"mysql minus minus is arithmetic", db.MySQL, "SELECT 1--1", true},
		{"pg identifier with dollar", db.PostgreSQL, "SELECT a$b FROM t", true},

		{"insert", db.MySQL, "INSERT INTO t VALUES (1)", false},
		{"drop", db.PostgreSQL, "DROP TABLE t", false},
		{"set", db.PostgreSQL, "SET TRANSACTION READ WRITE", false},
		{"commit then write", db.PostgreSQL, "COMMIT; DELETE FROM t", false},
		{"two statements", db.MySQL, "SELECT 1; DROP TABLE t", false},
		{"pragma write", db.SQLite, "PRAGMA query_only = 0", false},
		{"into outfile", db.MySQL, "SELECT * FROM t INTO OUTFILE '/tmp/x'", false},
		{"mysql executable comment", db.MySQL, "SELECT 1 /*! ; DROP TABLE t */", false},
		{"mysql dashes no space hide nothing", db.MySQL, "SELECT 1--1; DROP TABLE t", false},
		{"mysql backslash ambiguity", db.MySQL, `SELECT 'a\'; DROP TABLE t; -- '`, false},
		{"pg e-string ambiguity", db.PostgreSQL, `SELECT E'a\''; DROP TABLE t; --'`, false},
		{"pg dollar quote", db.PostgreSQL, "SELECT $$'$$; DROP TABLE t; --'", false},
		{"sqlite bracket identifier", db.SQLite, "SELECT [it's]; PRAGMA query_only=0; --'", false},
		{"empty", db.MySQL, "  ", false},
		{"comment only", db.MySQL, "-- SELECT 1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckReadOnly(tc.dbType, tc.query)
			if tc.allowed && err != nil {
				t.Fatalf("expected allowed, got %v", err)
			}
			if !tc.allowed && err == nil {
				t.Fatalf("expected rejection")
			}
		})
	}
}
