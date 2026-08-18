package database

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/cam/cine-cli/internal/core"
)

func (s *Store) AddWatchlistItem(ctx context.Context, item core.WatchlistItem) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO watchlist (media_id, title, media_type, season, episode, status, added_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		item.MediaID, item.Title, item.MediaType, item.Season, item.Episode, item.Status, time.Now())
	return err
}

func (s *Store) RemoveWatchlistItem(ctx context.Context, mediaID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM watchlist WHERE media_id = ?`, mediaID)
	return err
}

func (s *Store) ListWatchlist(ctx context.Context) ([]core.WatchlistItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, media_id, title, media_type, season, episode, status, added_at
		 FROM watchlist ORDER BY added_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []core.WatchlistItem
	for rows.Next() {
		var item core.WatchlistItem
		var addedAt string
		if err := rows.Scan(&item.ID, &item.MediaID, &item.Title, &item.MediaType,
			&item.Season, &item.Episode, &item.Status, &addedAt); err != nil {
			return nil, err
		}
		item.AddedAt, _ = time.Parse("2006-01-02 15:04:05", addedAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpdateWatchlistStatus(ctx context.Context, mediaID string, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE watchlist SET status = ? WHERE media_id = ?`, status, mediaID)
	return err
}

func (s *Store) ExportWatchlistJSON(ctx context.Context) ([]byte, error) {
	items, err := s.ListWatchlist(ctx)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(items, "", "  ")
}

func (s *Store) ImportWatchlistJSON(ctx context.Context, data []byte) (int, error) {
	var items []core.WatchlistItem
	if err := json.Unmarshal(data, &items); err != nil {
		return 0, err
	}
	count := 0
	for _, item := range items {
		if err := s.AddWatchlistItem(ctx, item); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *Store) BackupWatchlist(ctx context.Context, path string) error {
	data, err := s.ExportWatchlistJSON(ctx)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *Store) RestoreWatchlist(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = s.ImportWatchlistJSON(ctx, data)
	return err
}

func (s *Store) RemoveDuplicateWatchlistItems(ctx context.Context) (int, error) {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM watchlist WHERE id NOT IN (SELECT MIN(id) FROM watchlist GROUP BY media_id)`)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

func (s *Store) CountWatchlist(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM watchlist`).Scan(&count)
	return count, err
}

func (s *Store) ListWatchlistWithStatus(ctx context.Context, status string) ([]core.WatchlistItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, media_id, title, media_type, season, episode, status, added_at
		 FROM watchlist WHERE status = ? ORDER BY added_at DESC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []core.WatchlistItem
	for rows.Next() {
		var item core.WatchlistItem
		var addedAt string
		if err := rows.Scan(&item.ID, &item.MediaID, &item.Title, &item.MediaType,
			&item.Season, &item.Episode, &item.Status, &addedAt); err != nil {
			return nil, err
		}
		item.AddedAt, _ = time.Parse("2006-01-02 15:04:05", addedAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetWatchlistByMediaID(ctx context.Context, mediaID string) (*core.WatchlistItem, error) {
	var item core.WatchlistItem
	var addedAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, media_id, title, media_type, season, episode, status, added_at
		 FROM watchlist WHERE media_id = ?`, mediaID).Scan(
		&item.ID, &item.MediaID, &item.Title, &item.MediaType,
		&item.Season, &item.Episode, &item.Status, &addedAt)
	if err != nil {
		return nil, err
	}
	item.AddedAt, _ = time.Parse("2006-01-02 15:04:05", addedAt)
	return &item, nil
}
