package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

const schema = `
CREATE TABLE IF NOT EXISTS request_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
	upstream TEXT NOT NULL,
	tool TEXT NOT NULL,
	arguments TEXT,
	status TEXT,
	error TEXT,
	duration_ms INTEGER
);
CREATE INDEX IF NOT EXISTS idx_timestamp ON request_logs(timestamp DESC);
`

// CallRecord represents a single tool invocation log.
type CallRecord struct {
	ID         int64
	Timestamp  time.Time
	Upstream   string
	Tool       string
	Arguments  string
	Status     string
	Error      string
	DurationMs int64
}

// InitStore initializes the SQLite database.
func InitStore() error {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".gomcp", "audit.db")

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("create db dir: %w", err)
	}

	var err error
	DB, err = sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	if _, err := DB.Exec(schema); err != nil {
		return fmt.Errorf("init schema: %w", err)
	}

	return nil
}

// RecordCall logs a tool execution.
func RecordCall(upstream, tool, args string, status string, errStr string, duration time.Duration) error {
	if DB == nil {
		return nil
	}
	_, err := DB.Exec(`
		INSERT INTO request_logs (timestamp, upstream, tool, arguments, status, error, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, time.Now(), upstream, tool, args, status, errStr, duration.Milliseconds())
	return err
}

// GetRecentCalls retrieves the last N calls.
func GetRecentCalls(limit int) ([]CallRecord, error) {
	if DB == nil {
		return nil, nil
	}
	rows, err := DB.Query(`
		SELECT id, timestamp, upstream, tool, arguments, status, error, duration_ms
		FROM request_logs
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []CallRecord
	for rows.Next() {
		var r CallRecord
		var errStr sql.NullString
		if err := rows.Scan(&r.ID, &r.Timestamp, &r.Upstream, &r.Tool, &r.Arguments, &r.Status, &errStr, &r.DurationMs); err != nil {
			return nil, err
		}
		r.Error = errStr.String
		records = append(records, r)
	}
	return records, nil
}

func Close() {
	if DB != nil {
		DB.Close()
	}
}
