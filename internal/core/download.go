package core

import (
	"context"
	"time"
)

type DownloadStatus string

const (
	DownloadStatusQueued      DownloadStatus = "queued"
	DownloadStatusDownloading DownloadStatus = "downloading"
	DownloadStatusPaused      DownloadStatus = "paused"
	DownloadStatusCompleted   DownloadStatus = "completed"
	DownloadStatusFailed      DownloadStatus = "failed"
	DownloadStatusCancelled   DownloadStatus = "cancelled"
)

type Download struct {
	ID          string         `json:"id"`
	MediaID     string         `json:"media_id"`
	Title       string         `json:"title"`
	MediaType   MediaType      `json:"media_type"`
	Season      int            `json:"season"`
	Episode     int            `json:"episode"`
	URL         string         `json:"url"`
	Referer     string         `json:"referer"`
	UserAgent   string         `json:"user_agent"`
	Status      DownloadStatus `json:"status"`
	Progress    float64        `json:"progress"`
	TotalBytes  int64          `json:"total_bytes"`
	Downloaded  int64          `json:"downloaded"`
	Speed       float64        `json:"speed"`
	FilePath    string         `json:"file_path"`
	Error       string         `json:"error,omitempty"`
	Quality     string         `json:"quality"`
	Provider    string         `json:"provider"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

type DownloadManager interface {
	Enqueue(ctx context.Context, dl Download) error
	Pause(ctx context.Context, id string) error
	Resume(ctx context.Context, id string) error
	Cancel(ctx context.Context, id string) error
	List(ctx context.Context, status DownloadStatus) ([]Download, error)
	Get(ctx context.Context, id string) (*Download, error)
	Progress(ctx context.Context, id string) (float64, error)
	Cleanup(ctx context.Context) error
}

type DownloadStore interface {
	Save(ctx context.Context, dl Download) error
	Update(ctx context.Context, dl Download) error
	Get(ctx context.Context, id string) (*Download, error)
	List(ctx context.Context, status DownloadStatus) ([]Download, error)
	Delete(ctx context.Context, id string) error
	Cleanup(ctx context.Context) error
}
