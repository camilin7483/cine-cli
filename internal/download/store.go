package download

import (
	"context"
	"database/sql"

	"github.com/cam/cine-cli/internal/core"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Save(ctx context.Context, dl core.Download) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO downloads (id, media_id, title, media_type, season, episode, url, referer, user_agent,
		 status, progress, total_bytes, downloaded, speed, file_path, error, quality, provider,
		 created_at, updated_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dl.ID, dl.MediaID, dl.Title, dl.MediaType, dl.Season, dl.Episode,
		dl.URL, dl.Referer, dl.UserAgent,
		dl.Status, dl.Progress, dl.TotalBytes, dl.Downloaded, dl.Speed,
		dl.FilePath, dl.Error, dl.Quality, dl.Provider,
		dl.CreatedAt, dl.UpdatedAt, dl.CompletedAt)
	return err
}

func (s *Store) Update(ctx context.Context, dl core.Download) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE downloads SET status=?, progress=?, total_bytes=?, downloaded=?, speed=?,
		 file_path=?, error=?, updated_at=?, completed_at=? WHERE id=?`,
		dl.Status, dl.Progress, dl.TotalBytes, dl.Downloaded, dl.Speed,
		dl.FilePath, dl.Error, dl.UpdatedAt, dl.CompletedAt, dl.ID)
	return err
}

func (s *Store) Get(ctx context.Context, id string) (*core.Download, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, media_id, title, media_type, season, episode, url, referer, user_agent,
		 status, progress, total_bytes, downloaded, speed, file_path, COALESCE(error,''), quality, provider,
		 created_at, updated_at, completed_at
		 FROM downloads WHERE id=?`, id)

	var dl core.Download
	var completedAt sql.NullTime
	err := row.Scan(
		&dl.ID, &dl.MediaID, &dl.Title, &dl.MediaType, &dl.Season, &dl.Episode,
		&dl.URL, &dl.Referer, &dl.UserAgent,
		&dl.Status, &dl.Progress, &dl.TotalBytes, &dl.Downloaded, &dl.Speed,
		&dl.FilePath, &dl.Error, &dl.Quality, &dl.Provider,
		&dl.CreatedAt, &dl.UpdatedAt, &completedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		dl.CompletedAt = &completedAt.Time
	}
	return &dl, nil
}

func (s *Store) List(ctx context.Context, status core.DownloadStatus) ([]core.Download, error) {
	var rows *sql.Rows
	var err error
	if status == "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, media_id, title, media_type, season, episode, url, referer, user_agent,
			 status, progress, total_bytes, downloaded, speed, file_path, COALESCE(error,''), quality, provider,
			 created_at, updated_at, completed_at
			 FROM downloads ORDER BY created_at DESC`)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, media_id, title, media_type, season, episode, url, referer, user_agent,
			 status, progress, total_bytes, downloaded, speed, file_path, COALESCE(error,''), quality, provider,
			 created_at, updated_at, completed_at
			 FROM downloads WHERE status=? ORDER BY created_at DESC`, status)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []core.Download
	for rows.Next() {
		var dl core.Download
		var completedAt sql.NullTime
		if err := rows.Scan(
			&dl.ID, &dl.MediaID, &dl.Title, &dl.MediaType, &dl.Season, &dl.Episode,
			&dl.URL, &dl.Referer, &dl.UserAgent,
			&dl.Status, &dl.Progress, &dl.TotalBytes, &dl.Downloaded, &dl.Speed,
			&dl.FilePath, &dl.Error, &dl.Quality, &dl.Provider,
			&dl.CreatedAt, &dl.UpdatedAt, &completedAt); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			dl.CompletedAt = &completedAt.Time
		}
		results = append(results, dl)
	}
	return results, rows.Err()
}

func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM downloads WHERE id=?`, id)
	return err
}

func (s *Store) Cleanup(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM downloads WHERE status IN ('completed','cancelled','failed')
		 AND updated_at < datetime('now', '-7 days')`)
	return err
}
