package storage

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS requests (
    id          TEXT PRIMARY KEY,
    tunnel_id   TEXT NOT NULL,
    method      TEXT NOT NULL,
    path        TEXT NOT NULL,
    headers     TEXT NOT NULL DEFAULT '{}',
    body        BLOB,
    status_code INTEGER DEFAULT 0,
    res_headers TEXT DEFAULT '{}',
    res_body    BLOB,
    duration_ms INTEGER DEFAULT 0,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_requests_tunnel ON requests(tunnel_id);
CREATE INDEX IF NOT EXISTS idx_requests_created ON requests(created_at DESC);
`

// Store provides CRUD operations for webhook request storage.
type Store struct {
	db *sql.DB
}

// New opens (or creates) a SQLite database at the given path and applies the schema.
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Insert stores a new request record.
func (s *Store) Insert(r *RequestRecord) error {
	_, err := s.db.Exec(
		`INSERT INTO requests (id, tunnel_id, method, path, headers, body, status_code, res_headers, res_body, duration_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.TunnelID, r.Method, r.Path, r.Headers, r.Body,
		r.StatusCode, r.ResHeaders, r.ResBody, r.DurationMs,
	)
	if err != nil {
		return fmt.Errorf("insert request: %w", err)
	}
	return nil
}

// UpdateResponse updates the response fields for a stored request.
func (s *Store) UpdateResponse(id string, statusCode int, headers string, body []byte, durationMs int64) error {
	_, err := s.db.Exec(
		`UPDATE requests SET status_code = ?, res_headers = ?, res_body = ?, duration_ms = ? WHERE id = ?`,
		statusCode, headers, body, durationMs, id,
	)
	if err != nil {
		return fmt.Errorf("update response: %w", err)
	}
	return nil
}

// Get retrieves a single request record by ID.
func (s *Store) Get(id string) (*RequestRecord, error) {
	r := &RequestRecord{}
	err := s.db.QueryRow(
		`SELECT id, tunnel_id, method, path, headers, body, status_code, res_headers, res_body, duration_ms, created_at
		 FROM requests WHERE id = ?`, id,
	).Scan(&r.ID, &r.TunnelID, &r.Method, &r.Path, &r.Headers, &r.Body,
		&r.StatusCode, &r.ResHeaders, &r.ResBody, &r.DurationMs, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}
	return r, nil
}

// List returns the most recent requests, ordered by creation time descending.
func (s *Store) List(limit int) ([]*RequestRecord, error) {
	rows, err := s.db.Query(
		`SELECT id, tunnel_id, method, path, headers, body, status_code, res_headers, res_body, duration_ms, created_at
		 FROM requests ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}
	defer rows.Close()

	var records []*RequestRecord
	for rows.Next() {
		r := &RequestRecord{}
		if err := rows.Scan(&r.ID, &r.TunnelID, &r.Method, &r.Path, &r.Headers, &r.Body,
			&r.StatusCode, &r.ResHeaders, &r.ResBody, &r.DurationMs, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan request: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}
