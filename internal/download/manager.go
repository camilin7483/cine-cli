package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cam/cine-cli/internal/core"
)

type Manager struct {
	mu            sync.RWMutex
	downloads     map[string]*activeDownload
	maxConcurrent int
	downloadDir   string
	client        *http.Client
	store         core.DownloadStore
	sem           chan struct{}
	progressFn    func(core.Download)
}

type activeDownload struct {
	dl      core.Download
	cancel  context.CancelFunc
	paused  bool
	pauseCh chan struct{}
}

func NewManager(downloadDir string, maxConcurrent int, store core.DownloadStore) *Manager {
	_ = os.MkdirAll(downloadDir, 0755)
	return &Manager{
		downloads:     make(map[string]*activeDownload),
		maxConcurrent: maxConcurrent,
		downloadDir:   downloadDir,
		store:         store,
		sem:           make(chan struct{}, maxConcurrent),
		client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:    10,
				IdleConnTimeout: 30 * time.Second,
			},
		},
	}
}

func (m *Manager) Enqueue(ctx context.Context, dl core.Download) error {
	if dl.ID == "" {
		dl.ID = fmt.Sprintf("%x", time.Now().UnixNano())
	}
	dl.Status = core.DownloadStatusQueued
	dl.CreatedAt = time.Now()
	dl.UpdatedAt = time.Now()

	if dl.FilePath == "" {
		dl.FilePath = filepath.Join(m.downloadDir, FormatFilename(dl))
	}

	if err := m.store.Save(ctx, dl); err != nil {
		return fmt.Errorf("save download: %w", err)
	}

	go m.startDownload(&dl)
	return nil
}

func (m *Manager) startDownload(dl *core.Download) {
	m.sem <- struct{}{}
	defer func() { <-m.sem }()

	m.mu.Lock()
	ad := &activeDownload{
		dl:      *dl,
		pauseCh: make(chan struct{}),
	}
	m.downloads[dl.ID] = ad
	m.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	ad.cancel = cancel
	defer cancel()

	m.executeDownload(ctx, ad)
}

func (m *Manager) executeDownload(ctx context.Context, ad *activeDownload) {
	dl := &ad.dl
	dl.Status = core.DownloadStatusDownloading
	dl.UpdatedAt = time.Now()
	_ = m.store.Update(ctx, *dl)

	file, err := os.Create(dl.FilePath)
	if err != nil {
		dl.Status = core.DownloadStatusFailed
		dl.Error = err.Error()
		_ = m.store.Update(ctx, *dl)
		return
	}
	defer file.Close()

	req, err := http.NewRequestWithContext(ctx, "GET", dl.URL, nil)
	if err != nil {
		dl.Status = core.DownloadStatusFailed
		dl.Error = err.Error()
		_ = m.store.Update(ctx, *dl)
		return
	}

	if dl.Referer != "" {
		req.Header.Set("Referer", dl.Referer)
	}
	if dl.UserAgent != "" {
		req.Header.Set("User-Agent", dl.UserAgent)
	}

	if dl.Downloaded > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", dl.Downloaded))
		_, _ = file.Seek(dl.Downloaded, 0)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		dl.Status = core.DownloadStatusFailed
		dl.Error = err.Error()
		_ = m.store.Update(ctx, *dl)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		dl.Status = core.DownloadStatusFailed
		dl.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		_ = m.store.Update(ctx, *dl)
		return
	}

	if dl.TotalBytes == 0 {
		if resp.ContentLength > 0 {
			dl.TotalBytes = resp.ContentLength
		} else if h := resp.Header.Get("Content-Length"); h != "" {
			if n, err := strconv.ParseInt(h, 10, 64); err == nil {
				dl.TotalBytes = dl.Downloaded + n
			}
		}
	}

	buf := make([]byte, 32*1024)
	lastUpdate := time.Now()
	var lastBytes int64

	for {
		select {
		case <-ctx.Done():
			dl.Status = core.DownloadStatusCancelled
			_ = m.store.Update(ctx, *dl)
			return
		default:
		}

		m.mu.RLock()
		paused := ad.paused
		m.mu.RUnlock()

		if paused {
			select {
			case <-ad.pauseCh:
			case <-ctx.Done():
				dl.Status = core.DownloadStatusCancelled
				_ = m.store.Update(ctx, *dl)
				return
			}
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := file.Write(buf[:n]); werr != nil {
				dl.Status = core.DownloadStatusFailed
				dl.Error = werr.Error()
				_ = m.store.Update(ctx, *dl)
				return
			}
			dl.Downloaded += int64(n)
			if dl.TotalBytes > 0 {
				dl.Progress = float64(dl.Downloaded) / float64(dl.TotalBytes) * 100
			}

			now := time.Now()
			elapsed := now.Sub(lastUpdate).Seconds()
			if elapsed >= 0.5 {
				dl.Speed = float64(dl.Downloaded-lastBytes) / elapsed
				lastBytes = dl.Downloaded
				lastUpdate = now
				dl.UpdatedAt = now
				_ = m.store.Update(ctx, *dl)
				if m.progressFn != nil {
					m.progressFn(*dl)
				}
			}
		}

		if err != nil {
			if err == io.EOF {
				dl.Status = core.DownloadStatusCompleted
				dl.Progress = 100
				dl.Speed = 0
				now := time.Now()
				dl.CompletedAt = &now
				dl.UpdatedAt = now
				_ = m.store.Update(ctx, *dl)
				if m.progressFn != nil {
					m.progressFn(*dl)
				}
				return
			}
			dl.Status = core.DownloadStatusFailed
			dl.Error = err.Error()
			_ = m.store.Update(ctx, *dl)
			return
		}
	}
}

func (m *Manager) Pause(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ad, ok := m.downloads[id]
	if !ok {
		return fmt.Errorf("download %s not active", id)
	}
	ad.paused = true
	ad.dl.Status = core.DownloadStatusPaused
	ad.dl.UpdatedAt = time.Now()
	_ = m.store.Update(ctx, ad.dl)
	return nil
}

func (m *Manager) Resume(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ad, ok := m.downloads[id]
	if !ok {
		return m.resumeFromStore(ctx, id)
	}
	ad.paused = false
	ad.dl.Status = core.DownloadStatusQueued
	ad.dl.UpdatedAt = time.Now()
	_ = m.store.Update(ctx, ad.dl)
	close(ad.pauseCh)
	ad.pauseCh = make(chan struct{})
	return nil
}

func (m *Manager) resumeFromStore(ctx context.Context, id string) error {
	dl, err := m.store.Get(ctx, id)
	if err != nil {
		return err
	}
	if dl == nil {
		return fmt.Errorf("download %s not found", id)
	}
	dl.Status = core.DownloadStatusQueued
	dl.UpdatedAt = time.Now()
	_ = m.store.Update(ctx, *dl)
	go m.startDownload(dl)
	return nil
}

func (m *Manager) Cancel(ctx context.Context, id string) error {
	m.mu.Lock()
	ad, ok := m.downloads[id]
	m.mu.Unlock()

	if ok && ad.cancel != nil {
		ad.cancel()
	}

	dl, err := m.store.Get(ctx, id)
	if err != nil {
		return err
	}
	if dl == nil {
		return fmt.Errorf("download %s not found", id)
	}

	dl.Status = core.DownloadStatusCancelled
	dl.UpdatedAt = time.Now()
	_ = m.store.Update(ctx, *dl)

	if dl.FilePath != "" {
		os.Remove(dl.FilePath)
	}
	return nil
}

func (m *Manager) List(ctx context.Context, status core.DownloadStatus) ([]core.Download, error) {
	return m.store.List(ctx, status)
}

func (m *Manager) Get(ctx context.Context, id string) (*core.Download, error) {
	return m.store.Get(ctx, id)
}

func (m *Manager) Progress(ctx context.Context, id string) (float64, error) {
	dl, err := m.store.Get(ctx, id)
	if err != nil {
		return 0, err
	}
	if dl == nil {
		return 0, fmt.Errorf("download %s not found", id)
	}
	return dl.Progress, nil
}

func (m *Manager) Cleanup(ctx context.Context) error {
	entries, err := m.store.List(ctx, core.DownloadStatusCompleted)
	if err != nil {
		return err
	}
	for _, dl := range entries {
		if time.Since(dl.UpdatedAt) > 7*24*time.Hour {
			_ = m.store.Delete(ctx, dl.ID)
		}
	}
	return nil
}

func (m *Manager) SetProgressCallback(fn func(core.Download)) {
	m.progressFn = fn
}

func FormatFilename(dl core.Download) string {
	safe := func(s string) string {
		s = strings.Map(func(r rune) rune {
			if r > 31 && r < 127 || r > 127 {
				return r
			}
			return -1
		}, s)
		s = strings.ReplaceAll(s, "/", "-")
		s = strings.ReplaceAll(s, "\\", "-")
		return s
	}

	q := ""
	if dl.Quality != "" {
		q = " [" + dl.Quality + "]"
	}

	if dl.MediaType == core.MediaTypeSeries {
		return safe(fmt.Sprintf("%s - S%02dE%02d%s.mp4", dl.Title, dl.Season, dl.Episode, q))
	}
	return safe(fmt.Sprintf("%s%s.mp4", dl.Title, q))
}
