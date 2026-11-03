// Package sqlguard decides whether a SQL editor query may run in read-only mode.
//
// It is the first of two layers: CheckReadOnly allows a single statement
// starting with a read-only keyword, and the service then runs that
// statement inside a read-only transaction (PostgreSQL/MySQL) or with
// PRAGMA query_only (SQLite). The lexer is deliberately conservative: when
// quoting is ambiguous it rejects the query rather than guess.
package sqlguard

import (
	"errors"
	"regexp"
	"strings"

	"sqlmanager/pkg/db"
)

var (
	ErrMultipleStatements = errors.New("only a single statement is allowed")
	ErrNotReadOnly        = errors.New("only SELECT, WITH, SHOW, EXPLAIN, DESCRIBE, PRAGMA, VALUES and TABLE statements are allowed")
	ErrUnsupportedSyntax  = errors.New("MySQL executable comments (/*! ... */) and PostgreSQL dollar-quoted strings are not allowed")
	ErrFileWrite          = errors.New("SELECT ... INTO OUTFILE/DUMPFILE is not allowed")
)

var allowedKeywords = map[string]bool{
	"SELECT": true, "WITH": true, "SHOW": true, "EXPLAIN": true,
	"DESCRIBE": true, "DESC": true, "PRAGMA": true, "VALUES": true, "TABLE": true,
}

var intoFileRe = regexp.MustCompile(`\bINTO\s+(OUTFILE|DUMPFILE)\b`)

// CheckReadOnly returns nil if query is a single read-only statement.
func CheckReadOnly(dbType db.DBType, query string) error {
	// Scan twice, with and without backslash escapes in string literals.
	// Server modes (MySQL NO_BACKSLASH_ESCAPES, PostgreSQL E'' strings)
	// change where a string ends; requiring both readings to pass means a
	// ';' can't hide inside a literal under one reading only.
	for _, backslash := range []bool{false, true} {
		code, err := strip(dbType, query, backslash)
		if err != nil {
			return err
		}
		if err := checkCode(code); err != nil {
			return err
		}
	}
	return nil
}

func checkCode(code string) error {
	code = strings.TrimRight(code, " \t\r\n;")
	if strings.Contains(code, ";") {
		return ErrMultipleStatements
	}
	upper := strings.ToUpper(strings.TrimSpace(code))
	keyword := upper
	if i := strings.IndexFunc(upper, func(r rune) bool { return !(r >= 'A' && r <= 'Z') }); i >= 0 {
		keyword = upper[:i]
	}
	if !allowedKeywords[keyword] {
		return ErrNotReadOnly
	}
	if keyword == "PRAGMA" && strings.Contains(upper, "=") {
		return ErrNotReadOnly
	}
	if intoFileRe.MatchString(upper) {
		return ErrFileWrite
	}
	return nil
}

// strip returns query with comments replaced by a space and quoted
// strings/identifiers replaced by a placeholder, so the remaining text
// can be checked for ';' and keywords.
func strip(dbType db.DBType, q string, backslash bool) (string, error) {
	var b strings.Builder
	n := len(q)
	for i := 0; i < n; i++ {
		c := q[i]
		next := byte(0)
		if i+1 < n {
			next = q[i+1]
		}
		switch {
		case c == '-' && next == '-' && isLineComment(dbType, q, i):
			for i < n && q[i] != '\n' {
				i++
			}
			b.WriteByte(' ')
		case c == '#' && dbType == db.MySQL:
			for i < n && q[i] != '\n' {
				i++
			}
			b.WriteByte(' ')
		case c == '/' && next == '*':
			if dbType == db.MySQL && i+2 < n && q[i+2] == '!' {
				return "", ErrUnsupportedSyntax
			}
			end := strings.Index(q[i+2:], "*/")
			if end < 0 {
				i = n
			} else {
				i += 2 + end + 1
			}
			b.WriteByte(' ')
		case c == '$' && dbType == db.PostgreSQL && (next == '$' || isIdentStart(next)) && !(i > 0 && isIdentChar(q[i-1])):
			return "", ErrUnsupportedSyntax
		case c == '\'' || c == '"' || (c == '`' && dbType != db.PostgreSQL) || (c == '[' && dbType == db.SQLite):
			closer := c
			if c == '[' {
				closer = ']'
			}
			escapes := backslash && (c == '\'' || c == '"')
			i++
			for i < n {
				if escapes && q[i] == '\\' {
					i += 2
					continue
				}
				if q[i] == closer {
					if closer != ']' && i+1 < n && q[i+1] == closer {
						i += 2
						continue
					}
					break
				}
				i++
			}
			b.WriteString("''")
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), nil
}

// isLineComment reports whether the "--" at q[i] starts a comment. MySQL
// only treats "--" as a comment when followed by whitespace, control
// character or end of input; otherwise "1--1" is arithmetic.
func isLineComment(dbType db.DBType, q string, i int) bool {
	if dbType != db.MySQL {
		return true
	}
	if i+2 >= len(q) {
		return true
	}
	c := q[i+2]
	return c <= ' '
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isIdentChar(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == '$'
}
