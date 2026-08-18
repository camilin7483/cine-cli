package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/cam/cine-cli/internal/core"
)

func (s *Store) SaveProgress(ctx context.Context, mediaID, title string, mediaType core.MediaType, season, episode int, position, duration float64, provider, streamURL string) error {
	percentage := 0.0
	if duration > 0 {
		percentage = (position / duration) * 100
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO continue_watching (media_id, title, media_type, season, episode, position, duration, percentage, provider, stream_url, last_watched)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(media_id, season, episode) DO UPDATE SET
			title = excluded.title,
			media_type = excluded.media_type,
			position = excluded.position,
			duration = excluded.duration,
			percentage = excluded.percentage,
			provider = excluded.provider,
			stream_url = excluded.stream_url,
			last_watched = CURRENT_TIMESTAMP,
			completed = 0`,
		mediaID, title, mediaType, season, episode, position, duration, percentage, provider, streamURL)
	return err
}

func (s *Store) GetProgress(ctx context.Context, mediaID string, season, episode int) (*core.ContinueWatching, error) {
	var cw core.ContinueWatching
	var lastWatched string
	var completed int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, media_id, title, media_type, season, episode, position, duration, percentage, provider, stream_url, last_watched, completed
		 FROM continue_watching WHERE media_id = ? AND season = ? AND episode = ?`,
		mediaID, season, episode).Scan(
		&cw.ID, &cw.MediaID, &cw.Title, &cw.MediaType, &cw.Season, &cw.Episode,
		&cw.Position, &cw.Duration, &cw.Percentage, &cw.Provider, &cw.StreamURL,
		&lastWatched, &completed)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cw.LastWatched, _ = time.Parse("2006-01-02 15:04:05", lastWatched)
	cw.Completed = completed == 1
	return &cw, nil
}

func (s *Store) ListContinueWatching(ctx context.Context, limit int) ([]core.ContinueWatching, error) {
	q := `SELECT id, media_id, title, media_type, season, episode, position, duration, percentage, provider, stream_url, last_watched, completed
		  FROM continue_watching WHERE completed = 0 ORDER BY last_watched DESC`
	var args []any
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []core.ContinueWatching
	for rows.Next() {
		var cw core.ContinueWatching
		var lastWatched string
		var completed int
		if err := rows.Scan(&cw.ID, &cw.MediaID, &cw.Title, &cw.MediaType, &cw.Season, &cw.Episode,
			&cw.Position, &cw.Duration, &cw.Percentage, &cw.Provider, &cw.StreamURL,
			&lastWatched, &completed); err != nil {
			return nil, err
		}
		cw.LastWatched, _ = time.Parse("2006-01-02 15:04:05", lastWatched)
		cw.Completed = completed == 1
		entries = append(entries, cw)
	}
	return entries, rows.Err()
}

func (s *Store) MarkCompleted(ctx context.Context, mediaID string, season, episode int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE continue_watching SET completed = 1, position = duration, percentage = 100 WHERE media_id = ? AND season = ? AND episode = ?`,
		mediaID, season, episode)
	return err
}

func (s *Store) DeleteProgress(ctx context.Context, mediaID string, season, episode int) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM continue_watching WHERE media_id = ? AND season = ? AND episode = ?`,
		mediaID, season, episode)
	return err
}
