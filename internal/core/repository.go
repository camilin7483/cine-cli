package core

import "context"

type HistoryStore interface {
	Add(ctx context.Context, entry HistoryEntry) error
	List(ctx context.Context, limit, offset int) ([]HistoryEntry, error)
	GetLastPosition(ctx context.Context, mediaID string) (float64, error)
	Stats(ctx context.Context) (*HistoryStats, error)
	Clear(ctx context.Context) error
}

type FavoritesStore interface {
	Add(ctx context.Context, fav Favorite) error
	Remove(ctx context.Context, mediaID string) error
	List(ctx context.Context) ([]Favorite, error)
	Exists(ctx context.Context, mediaID string) (bool, error)
}

type ContinueWatchingStore interface {
	SaveProgress(ctx context.Context, entry ContinueWatching) error
	GetProgress(ctx context.Context, mediaID string, season, episode int) (*ContinueWatching, error)
	ListContinueWatching(ctx context.Context, limit int) ([]ContinueWatching, error)
	MarkCompleted(ctx context.Context, mediaID string, season, episode int) error
	DeleteProgress(ctx context.Context, mediaID string, season, episode int) error
}

type WatchlistStore interface {
	Add(ctx context.Context, item WatchlistItem) error
	Remove(ctx context.Context, mediaID string) error
	List(ctx context.Context) ([]WatchlistItem, error)
	UpdateStatus(ctx context.Context, mediaID string, status string) error
}

type AdvancedHistoryStore interface {
	HistoryStore
	Search(ctx context.Context, query string, limit, offset int) ([]HistoryEntry, error)
	ListWithFilters(ctx context.Context, filter HistoryFilter) ([]HistoryEntry, error)
	UpdatePosition(ctx context.Context, mediaID string, season, episode int, position, duration float64) error
	GetByMediaID(ctx context.Context, mediaID string, limit int) ([]HistoryEntry, error)
	DeleteByID(ctx context.Context, id int64) error
}

type AdvancedFavoritesStore interface {
	FavoritesStore
	ExportJSON(ctx context.Context) ([]byte, error)
	ImportJSON(ctx context.Context, data []byte) (int, error)
	Backup(ctx context.Context, path string) error
	Restore(ctx context.Context, path string) error
	RemoveDuplicates(ctx context.Context) (int, error)
	Count(ctx context.Context) (int, error)
}

type AdvancedWatchlistStore interface {
	WatchlistStore
	ExportJSON(ctx context.Context) ([]byte, error)
	ImportJSON(ctx context.Context, data []byte) (int, error)
	Backup(ctx context.Context, path string) error
	Restore(ctx context.Context, path string) error
	RemoveDuplicates(ctx context.Context) (int, error)
	Count(ctx context.Context) (int, error)
	ListWithStatus(ctx context.Context, status string) ([]WatchlistItem, error)
	GetByMediaID(ctx context.Context, mediaID string) (*WatchlistItem, error)
}
