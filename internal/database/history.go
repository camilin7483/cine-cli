package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cam/cine-cli/internal/core"
)

func (s *Store) Add(ctx context.Context, entry core.HistoryEntry) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO history (media_id, title, media_type, season, episode, provider, stream_url, position, duration, watched_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.MediaID, entry.Title, entry.MediaType, entry.Season, entry.Episode,
		entry.Provider, entry.StreamURL, entry.Position, entry.Duration, time.Now())
	return err
}

func (s *Store) List(ctx context.Context, limit, offset int) ([]core.HistoryEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, media_id, title, media_type, season, episode, provider, stream_url, position, duration, watched_at
		 FROM history ORDER BY watched_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []core.HistoryEntry
	for rows.Next() {
		var e core.HistoryEntry
		var watchedAt string
		if err := rows.Scan(&e.ID, &e.MediaID, &e.Title, &e.MediaType, &e.Season, &e.Episode,
			&e.Provider, &e.StreamURL, &e.Position, &e.Duration, &watchedAt); err != nil {
			return nil, err
		}
		e.WatchedAt, _ = time.Parse("2006-01-02 15:04:05", watchedAt)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Store) GetLastPosition(ctx context.Context, mediaID string) (float64, error) {
	var pos sql.NullFloat64
	err := s.db.QueryRowContext(ctx,
		`SELECT position FROM history WHERE media_id = ? ORDER BY watched_at DESC LIMIT 1`, mediaID).Scan(&pos)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return pos.Float64, nil
}

func (s *Store) Stats(ctx context.Context) (*core.HistoryStats, error) {
	stats := &core.HistoryStats{}
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT CASE WHEN media_type = 'movie' THEN media_id END) as movies,
				COUNT(DISTINCT CASE WHEN media_type = 'series' THEN media_id END) as series,
				COUNT(*) as total FROM history`).Scan(&stats.TotalMovies, &stats.TotalShows, &stats.TotalEpisodes)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

func (s *Store) Clear(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM history`)
	return err
}

func (s *Store) Search(ctx context.Context, query string, limit, offset int) ([]core.HistoryEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, media_id, title, media_type, season, episode, provider, stream_url, position, duration, watched_at
		 FROM history WHERE title LIKE ? ORDER BY watched_at DESC LIMIT ? OFFSET ?`,
		"%"+query+"%", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []core.HistoryEntry
	for rows.Next() {
		var e core.HistoryEntry
		var watchedAt string
		if err := rows.Scan(&e.ID, &e.MediaID, &e.Title, &e.MediaType, &e.Season, &e.Episode,
			&e.Provider, &e.StreamURL, &e.Position, &e.Duration, &watchedAt); err != nil {
			return nil, err
		}
		e.WatchedAt, _ = time.Parse("2006-01-02 15:04:05", watchedAt)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Store) ListWithFilters(ctx context.Context, filter core.HistoryFilter) ([]core.HistoryEntry, error) {
	q := `SELECT id, media_id, title, media_type, season, episode, provider, stream_url, position, duration, watched_at FROM history WHERE 1=1`
	var args []any

	if filter.Query != "" {
		q += ` AND title LIKE ?`
		args = append(args, "%"+filter.Query+"%")
	}
	if filter.MediaType != "" {
		q += ` AND media_type = ?`
		args = append(args, string(filter.MediaType))
	}

	sortBy := "watched_at"
	switch filter.SortBy {
	case "title":
		sortBy = "title"
	case "duration":
		sortBy = "duration"
	case "progress":
		sortBy = "position"
	}
	order := "DESC"
	if filter.SortOrder == "asc" {
		order = "ASC"
	}
	q += fmt.Sprintf(` ORDER BY %s %s`, sortBy, order)

	if filter.Limit > 0 {
		q += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		q += ` OFFSET ?`
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []core.HistoryEntry
	for rows.Next() {
		var e core.HistoryEntry
		var watchedAt string
		if err := rows.Scan(&e.ID, &e.MediaID, &e.Title, &e.MediaType, &e.Season, &e.Episode,
			&e.Provider, &e.StreamURL, &e.Position, &e.Duration, &watchedAt); err != nil {
			return nil, err
		}
		e.WatchedAt, _ = time.Parse("2006-01-02 15:04:05", watchedAt)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Store) UpdatePosition(ctx context.Context, mediaID string, season, episode int, position, duration float64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE history SET position = ?, duration = ? WHERE media_id = ? AND season = ? AND episode = ?`,
		position, duration, mediaID, season, episode)
	return err
}

func (s *Store) GetByMediaID(ctx context.Context, mediaID string, limit int) ([]core.HistoryEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, media_id, title, media_type, season, episode, provider, stream_url, position, duration, watched_at
		 FROM history WHERE media_id = ? ORDER BY watched_at DESC LIMIT ?`, mediaID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []core.HistoryEntry
	for rows.Next() {
		var e core.HistoryEntry
		var watchedAt string
		if err := rows.Scan(&e.ID, &e.MediaID, &e.Title, &e.MediaType, &e.Season, &e.Episode,
			&e.Provider, &e.StreamURL, &e.Position, &e.Duration, &watchedAt); err != nil {
			return nil, err
		}
		e.WatchedAt, _ = time.Parse("2006-01-02 15:04:05", watchedAt)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Store) DeleteByID(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM history WHERE id = ?`, id)
	return err
}
