package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/cam/cine-cli/internal/core"
)

func (s *Store) ensureBookmarks(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS bookmarks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			media_id TEXT NOT NULL,
			title TEXT NOT NULL,
			media_type TEXT NOT NULL,
			season INTEGER DEFAULT 0,
			episode INTEGER DEFAULT 0,
			position REAL DEFAULT 0,
			note TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(media_id, season, episode, position)
		)`)
	return err
}

func (s *Store) AddBookmark(ctx context.Context, mediaID, title string, mediaType core.MediaType, season, episode int, position float64, note string) error {
	if err := s.ensureBookmarks(ctx); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO bookmarks (media_id, title, media_type, season, episode, position, note, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		mediaID, title, mediaType, season, episode, position, note)
	return err
}

type Bookmark struct {
	ID        int64
	MediaID   string
	Title     string
	MediaType core.MediaType
	Season    int
	Episode   int
	Position  float64
	Note      string
	CreatedAt time.Time
}

func (s *Store) ListBookmarks(ctx context.Context, limit int) ([]Bookmark, error) {
	if err := s.ensureBookmarks(ctx); err != nil {
		return nil, err
	}
	q := `SELECT id, media_id, title, media_type, season, episode, position, note, created_at
	      FROM bookmarks ORDER BY created_at DESC`
	if limit > 0 {
		q += ` LIMIT ?`
	}
	var rows *sql.Rows
	var err error
	if limit > 0 {
		rows, err = s.db.QueryContext(ctx, q, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, q)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Bookmark
	for rows.Next() {
		var b Bookmark
		var created string
		if err := rows.Scan(&b.ID, &b.MediaID, &b.Title, &b.MediaType, &b.Season, &b.Episode, &b.Position, &b.Note, &created); err != nil {
			return nil, err
		}
		b.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) DeleteBookmark(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM bookmarks WHERE id = ?`, id)
	return err
}
