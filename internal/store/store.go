package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS update_log (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		started_at DATETIME NOT NULL,
		kind       TEXT NOT NULL,
		target     TEXT NOT NULL,
		operation  TEXT NOT NULL
	)`); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// LogUpdate records the start of an update operation.
// kind: "stack" or "container"
// target: stack name or container name
// operation: "update", "save_images", "save_and_update", "snapshot", "save_image"
func (s *Store) LogUpdate(kind, target, operation string) error {
	_, err := s.db.Exec(
		`INSERT INTO update_log (started_at, kind, target, operation) VALUES (?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339Nano), kind, target, operation,
	)
	return err
}

// LastUpdateByTarget returns the most recent started_at per target name.
func (s *Store) LastUpdateByTarget() (map[string]time.Time, error) {
	rows, err := s.db.Query(`SELECT target, MAX(started_at) FROM update_log GROUP BY target`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]time.Time)
	for rows.Next() {
		var target, raw string
		if err := rows.Scan(&target, &raw); err != nil {
			return nil, err
		}
		t, err := parseSQLiteTime(raw)
		if err != nil {
			return nil, err
		}
		result[target] = t
	}
	return result, rows.Err()
}

// parseSQLiteTime parses timestamps as returned by the sqlite driver.
// Aggregate results (MAX etc.) lose the column decltype, so the driver
// returns strings instead of time.Time.
func parseSQLiteTime(raw string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999 -0700 MST", // legacy rows written as Go time.Time default string
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
	}
	var err error
	for _, layout := range layouts {
		var t time.Time
		if t, err = time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, err
}
