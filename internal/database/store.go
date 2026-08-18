package database

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}

	var v1 int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_version WHERE version = 1`).Scan(&v1)
	if err != nil {
		return err
	}

	if v1 == 0 {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
			CREATE TABLE IF NOT EXISTS history (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				media_id TEXT NOT NULL,
				title TEXT NOT NULL,
				media_type TEXT NOT NULL,
				season INTEGER DEFAULT 0,
				episode INTEGER DEFAULT 0,
				provider TEXT NOT NULL DEFAULT '',
				stream_url TEXT DEFAULT '',
				position REAL DEFAULT 0,
				duration REAL DEFAULT 0,
				watched_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);

			CREATE INDEX IF NOT EXISTS idx_history_media_id ON history(media_id);
			CREATE INDEX IF NOT EXISTS idx_history_watched_at ON history(watched_at);

			CREATE TABLE IF NOT EXISTS favorites (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				media_id TEXT NOT NULL UNIQUE,
				title TEXT NOT NULL,
				media_type TEXT NOT NULL,
				poster_url TEXT DEFAULT '',
				added_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);

			CREATE TABLE IF NOT EXISTS watchlist (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				media_id TEXT NOT NULL UNIQUE,
				title TEXT NOT NULL,
				media_type TEXT NOT NULL,
				season INTEGER DEFAULT 0,
				episode INTEGER DEFAULT 0,
				status TEXT DEFAULT 'pending',
				added_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);

			CREATE TABLE IF NOT EXISTS cache (
				key TEXT PRIMARY KEY,
				value BLOB NOT NULL,
				expires_at DATETIME NOT NULL
			);

			CREATE INDEX IF NOT EXISTS idx_cache_expires ON cache(expires_at);

			INSERT INTO schema_version (version) VALUES (1)
		`)
		if err != nil {
			_ = tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	var v2 int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM schema_version WHERE version = 2`).Scan(&v2)
	if err != nil {
		return err
	}

	if v2 == 0 {
		_, err := s.db.Exec(`
			CREATE TABLE IF NOT EXISTS continue_watching (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				media_id TEXT NOT NULL,
				title TEXT NOT NULL,
				media_type TEXT NOT NULL,
				season INTEGER DEFAULT 0,
				episode INTEGER DEFAULT 0,
				position REAL DEFAULT 0,
				duration REAL DEFAULT 0,
				percentage REAL DEFAULT 0,
				provider TEXT DEFAULT '',
				stream_url TEXT DEFAULT '',
				last_watched DATETIME DEFAULT CURRENT_TIMESTAMP,
				completed INTEGER DEFAULT 0,
				UNIQUE(media_id, season, episode)
			);

			INSERT INTO schema_version (version) VALUES (2)
		`)
		if err != nil {
			return err
		}
	}

	var v3 int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM schema_version WHERE version = 3`).Scan(&v3)
	if err != nil {
		return err
	}

	if v3 == 0 {
		_, err := s.db.Exec(`
			CREATE TABLE IF NOT EXISTS downloads (
				id TEXT PRIMARY KEY,
				media_id TEXT NOT NULL DEFAULT '',
				title TEXT NOT NULL DEFAULT '',
				media_type TEXT NOT NULL DEFAULT '',
				season INTEGER DEFAULT 0,
				episode INTEGER DEFAULT 0,
				url TEXT NOT NULL DEFAULT '',
				referer TEXT DEFAULT '',
				user_agent TEXT DEFAULT '',
				status TEXT NOT NULL DEFAULT 'queued',
				progress REAL DEFAULT 0,
				total_bytes INTEGER DEFAULT 0,
				downloaded INTEGER DEFAULT 0,
				speed REAL DEFAULT 0,
				file_path TEXT DEFAULT '',
				error TEXT DEFAULT '',
				quality TEXT DEFAULT '',
				provider TEXT DEFAULT '',
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				completed_at DATETIME
			);

			CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status);

			INSERT INTO schema_version (version) VALUES (3)
		`)
		if err != nil {
			return err
		}
	}

	return nil
}
