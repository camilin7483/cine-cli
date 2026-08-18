package database

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/cam/cine-cli/internal/core"
)

func (s *Store) AddFavorite(ctx context.Context, fav core.Favorite) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO favorites (media_id, title, media_type, poster_url, added_at) VALUES (?, ?, ?, ?, ?)`,
		fav.MediaID, fav.Title, fav.MediaType, fav.PosterURL, time.Now())
	return err
}

func (s *Store) RemoveFavorite(ctx context.Context, mediaID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM favorites WHERE media_id = ?`, mediaID)
	return err
}

func (s *Store) ListFavorites(ctx context.Context) ([]core.Favorite, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, media_id, title, media_type, poster_url, added_at FROM favorites ORDER BY added_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var favs []core.Favorite
	for rows.Next() {
		var f core.Favorite
		var addedAt string
		if err := rows.Scan(&f.ID, &f.MediaID, &f.Title, &f.MediaType, &f.PosterURL, &addedAt); err != nil {
			return nil, err
		}
		f.AddedAt, _ = time.Parse("2006-01-02 15:04:05", addedAt)
		favs = append(favs, f)
	}
	return favs, rows.Err()
}

func (s *Store) FavoriteExists(ctx context.Context, mediaID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM favorites WHERE media_id = ?`, mediaID).Scan(&count)
	return count > 0, err
}

func (s *Store) ExportFavoritesJSON(ctx context.Context) ([]byte, error) {
	favs, err := s.ListFavorites(ctx)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(favs, "", "  ")
}

func (s *Store) ImportFavoritesJSON(ctx context.Context, data []byte) (int, error) {
	var favs []core.Favorite
	if err := json.Unmarshal(data, &favs); err != nil {
		return 0, err
	}
	count := 0
	for _, f := range favs {
		if err := s.AddFavorite(ctx, f); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *Store) BackupFavorites(ctx context.Context, path string) error {
	data, err := s.ExportFavoritesJSON(ctx)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *Store) RestoreFavorites(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = s.ImportFavoritesJSON(ctx, data)
	return err
}

func (s *Store) RemoveDuplicateFavorites(ctx context.Context) (int, error) {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM favorites WHERE id NOT IN (SELECT MIN(id) FROM favorites GROUP BY media_id)`)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

func (s *Store) CountFavorites(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM favorites`).Scan(&count)
	return count, err
}
