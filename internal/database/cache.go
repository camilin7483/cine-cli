package database

import (
	"context"
	"database/sql"
	"time"
)

func (s *Store) CacheGet(ctx context.Context, key string) ([]byte, bool, error) {
	var value []byte
	var expiresAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT value, expires_at FROM cache WHERE key = ? AND expires_at > datetime('now')`, key).Scan(&value, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func (s *Store) CacheSet(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO cache (key, value, expires_at) VALUES (?, ?, datetime('now', ?))`,
		key, value, "+"+ttl.String())
	return err
}

func (s *Store) CacheDelete(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM cache WHERE key = ?`, key)
	return err
}

func (s *Store) CacheCleanup(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM cache WHERE expires_at < datetime('now')`)
	return err
}
