package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// Store 负责审计日志存储，当前实现为占位。
type Store struct {
	db *sql.DB
}

func Open(defaultPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(defaultPath), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	db, err := sql.Open("sqlite3", defaultPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Record(event string, payload string) error {
	// TODO: 创建表结构并插入审计记录。此处为占位实现。
	_ = event
	_ = payload
	return nil
}
