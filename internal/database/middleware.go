package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// QueryMiddleware provides query instrumentation and monitoring
type QueryMiddleware struct {
	pool *DatabasePool
}

// NewQueryMiddleware creates a new query instrumentation middleware
func NewQueryMiddleware(pool *DatabasePool) *QueryMiddleware {
	return &QueryMiddleware{
		pool: pool,
	}
}

// InstrumentedConn wraps a database connection with instrumentation
type InstrumentedConn struct {
	pgx.Conn
	middleware *QueryMiddleware
}

// InstrumentedTx wraps a database transaction with instrumentation
type InstrumentedTx struct {
	pgx.Tx
	middleware *QueryMiddleware
	startTime  time.Time
}

// Query executes a query with instrumentation
func (c *InstrumentedConn) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		operation, table := parseSQL(sql)
		c.middleware.pool.RecordQuery(operation, table, duration, nil)
	}()

	return c.Conn.Query(ctx, sql, args...)
}

// QueryRow executes a single-row query with instrumentation
func (c *InstrumentedConn) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		operation, table := parseSQL(sql)
		c.middleware.pool.RecordQuery(operation, table, duration, nil)
	}()

	return c.Conn.QueryRow(ctx, sql, args...)
}

// Exec executes a query with instrumentation
func (c *InstrumentedConn) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	start := time.Now()
	result, err := c.Conn.Exec(ctx, sql, args...)
	duration := time.Since(start)

	operation, table := parseSQL(sql)
	c.middleware.pool.RecordQuery(operation, table, duration, err)

	return result, err
}

// Begin starts a transaction with instrumentation
func (c *InstrumentedConn) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := c.Conn.Begin(ctx)
	if err != nil {
		return nil, err
	}

	return &InstrumentedTx{
		Tx:         tx,
		middleware: c.middleware,
		startTime:  time.Now(),
	}, nil
}

// Query executes a query within a transaction with instrumentation
func (tx *InstrumentedTx) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		operation, table := parseSQL(sql)
		tx.middleware.pool.RecordQuery(operation, table, duration, nil)
	}()

	return tx.Tx.Query(ctx, sql, args...)
}

// QueryRow executes a single-row query within a transaction with instrumentation
func (tx *InstrumentedTx) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		operation, table := parseSQL(sql)
		tx.middleware.pool.RecordQuery(operation, table, duration, nil)
	}()

	return tx.Tx.QueryRow(ctx, sql, args...)
}

// Exec executes a query within a transaction with instrumentation
func (tx *InstrumentedTx) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	start := time.Now()
	result, err := tx.Tx.Exec(ctx, sql, args...)
	duration := time.Since(start)

	operation, table := parseSQL(sql)
	tx.middleware.pool.RecordQuery(operation, table, duration, err)

	return result, err
}

// Commit commits the transaction and records transaction duration
func (tx *InstrumentedTx) Commit(ctx context.Context) error {
	defer func() {
		duration := time.Since(tx.startTime)
		tx.middleware.pool.RecordTransaction(duration)
	}()

	return tx.Tx.Commit(ctx)
}

// Rollback rolls back the transaction and records transaction duration
func (tx *InstrumentedTx) Rollback(ctx context.Context) error {
	defer func() {
		duration := time.Since(tx.startTime)
		tx.middleware.pool.RecordTransaction(duration)
	}()

	return tx.Tx.Rollback(ctx)
}

// QueryExecutor provides a unified interface for instrumented query execution
type QueryExecutor struct {
	pool       *DatabasePool
	middleware *QueryMiddleware
}

// NewQueryExecutor creates a new query executor with instrumentation
func NewQueryExecutor(pool *DatabasePool) *QueryExecutor {
	return &QueryExecutor{
		pool:       pool,
		middleware: NewQueryMiddleware(pool),
	}
}

// QueryWithInstrumentation executes a query with automatic instrumentation
func (qe *QueryExecutor) QueryWithInstrumentation(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	start := time.Now()
	rows, err := qe.pool.Query(ctx, sql, args...)
	duration := time.Since(start)

	operation, table := parseSQL(sql)
	qe.pool.RecordQuery(operation, table, duration, err)

	return rows, err
}

// QueryRowWithInstrumentation executes a single-row query with automatic instrumentation
func (qe *QueryExecutor) QueryRowWithInstrumentation(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		operation, table := parseSQL(sql)
		qe.pool.RecordQuery(operation, table, duration, nil)
	}()

	return qe.pool.QueryRow(ctx, sql, args...)
}

// ExecWithInstrumentation executes a query with automatic instrumentation
func (qe *QueryExecutor) ExecWithInstrumentation(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	start := time.Now()
	result, err := qe.pool.Exec(ctx, sql, args...)
	duration := time.Since(start)

	operation, table := parseSQL(sql)
	qe.pool.RecordQuery(operation, table, duration, err)

	return result, err
}

// BeginTxWithInstrumentation starts a transaction with instrumentation
func (qe *QueryExecutor) BeginTxWithInstrumentation(ctx context.Context) (*InstrumentedTx, error) {
	tx, err := qe.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	return &InstrumentedTx{
		Tx:         tx,
		middleware: qe.middleware,
		startTime:  time.Now(),
	}, nil
}

// WithTx executes a function within a transaction with automatic rollback on error
func (qe *QueryExecutor) WithTx(ctx context.Context, fn func(*InstrumentedTx) error) error {
	tx, err := qe.BeginTxWithInstrumentation(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback(ctx)
			panic(p)
		}
	}()

	err = fn(tx)
	if err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			// Log rollback error but return original error
			return err
		}
		return err
	}

	return tx.Commit(ctx)
}

// parseSQL extracts operation and table name from SQL query for metrics
func parseSQL(sql string) (operation, table string) {
	// This is a simplified parser for basic SQL operations
	// In production, you might want to use a more sophisticated SQL parser

	// Default values
	operation = "unknown"
	table = "unknown"

	// Remove leading/trailing whitespace and convert to lowercase
	sql = trimAndLower(sql)

	// Extract operation (first word)
	if len(sql) > 0 {
		words := splitWords(sql)
		if len(words) > 0 {
			operation = words[0]
		}

		// Extract table name based on operation
		switch operation {
		case "select":
			table = extractTableFromSelect(words)
		case "insert":
			table = extractTableFromInsert(words)
		case "update":
			table = extractTableFromUpdate(words)
		case "delete":
			table = extractTableFromDelete(words)
		default:
			if len(words) > 1 {
				table = words[1]
			}
		}
	}

	return operation, table
}

// Helper functions for SQL parsing
func trimAndLower(s string) string {
	// Simple trim and lowercase function
	start := 0
	end := len(s)

	// Trim leading whitespace
	for start < end && isWhitespace(s[start]) {
		start++
	}

	// Trim trailing whitespace
	for end > start && isWhitespace(s[end-1]) {
		end--
	}

	// Convert to lowercase
	result := make([]byte, end-start)
	for i := start; i < end; i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			result[i-start] = s[i] + 32
		} else {
			result[i-start] = s[i]
		}
	}

	return string(result)
}

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func splitWords(s string) []string {
	var words []string
	var current []byte

	for i := 0; i < len(s); i++ {
		if isWhitespace(s[i]) {
			if len(current) > 0 {
				words = append(words, string(current))
				current = current[:0]
			}
		} else {
			current = append(current, s[i])
		}
	}

	if len(current) > 0 {
		words = append(words, string(current))
	}

	return words
}

func extractTableFromSelect(words []string) string {
	for i, word := range words {
		if word == "from" && i+1 < len(words) {
			return words[i+1]
		}
	}
	return "unknown"
}

func extractTableFromInsert(words []string) string {
	for i, word := range words {
		if word == "into" && i+1 < len(words) {
			return words[i+1]
		}
	}
	return "unknown"
}

func extractTableFromUpdate(words []string) string {
	if len(words) > 1 {
		return words[1]
	}
	return "unknown"
}

func extractTableFromDelete(words []string) string {
	for i, word := range words {
		if word == "from" && i+1 < len(words) {
			return words[i+1]
		}
	}
	return "unknown"
}