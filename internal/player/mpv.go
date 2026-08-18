package player

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cam/cine-cli/internal/core"
)

type MPV struct {
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	mu      sync.Mutex
	running bool
	args    []string
	ipcPath string
}

func NewMPV(extraArgs []string) *MPV {
	return &MPV{args: extraArgs}
}

func (p *MPV) Name() string { return "mpv" }

func (p *MPV) Play(ctx context.Context, opts core.PlayOptions) error {
	p.mu.Lock()
	if p.running {
		p.stopLocked()
		time.Sleep(300 * time.Millisecond)
	}
	p.mu.Unlock()

	playURL, headers := normalizeStream(opts.StreamURL, opts.Referer, opts.UserAgent)
	ua := headers["User-Agent"]

	// Validate BEFORE opening a window — avoids flash open/close on 403/428
	if err := preflightStream(ctx, playURL, headers); err != nil {
		return err
	}

	p.mu.Lock()
	p.ipcPath = filepath.Join(os.TempDir(), fmt.Sprintf("cine-mpv-%d.sock", time.Now().UnixNano()))
	_ = os.Remove(p.ipcPath)

	args := make([]string, 0, 24)
	args = append(args, "--input-ipc-server="+p.ipcPath)
	args = append(args, "--idle=no")
	// Only show window once we know the stream answers
	args = append(args, "--force-window=yes")
	args = append(args, "--no-ytdl")
	args = append(args, "--user-agent="+ua)
	if hf := headerFieldsArg(headers); hf != "" {
		args = append(args, "--http-header-fields="+hf)
	}
	if ref := headers["Referer"]; ref != "" {
		args = append(args, "--referrer="+ref)
	}
	if opts.Title != "" {
		args = append(args, "--title="+opts.Title)
	}
	if opts.PreferredLang != "" && opts.PreferredLang != "original" {
		args = append(args, "--alang="+langCodes(opts.PreferredLang))
	}
	if opts.SubsLang != "" && opts.SubsLang != "original" {
		args = append(args, "--slang="+langCodes(opts.SubsLang))
	}
	for _, sub := range opts.Subtitles {
		if sub.URL != "" {
			args = append(args, "--sub-file="+sub.URL)
		}
	}
	for _, extra := range opts.ExtraArgs {
		args = append(args, extra)
	}
	args = append(args, p.args...)
	args = append(args, playURL)

	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	cmd := exec.CommandContext(ctx, "mpv", args...)
	// Keep TUI clean: don't dump ffmpeg noise into the alternate screen
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	p.cmd = cmd
	p.running = true

	if err := cmd.Start(); err != nil {
		p.running = false
		p.ipcPath = ""
		p.mu.Unlock()
		return fmt.Errorf("mpv start: %w", err)
	}
	p.mu.Unlock()

	go func() {
		_ = cmd.Wait()
		p.mu.Lock()
		p.running = false
		if p.ipcPath != "" {
			_ = os.Remove(p.ipcPath)
			p.ipcPath = ""
		}
		p.mu.Unlock()
	}()

	time.Sleep(500 * time.Millisecond)
	if !p.Running() {
		return fmt.Errorf("mpv cerró al instante (stream inválido)")
	}
	return nil
}

func langCodes(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	aliases := map[string]string{
		"en": "en,eng,en-US,en-GB",
		"es": "es,spa,es-ES,es-MX,es-419",
		"pt": "pt,por,pt-BR,pt-PT",
		"fr": "fr,fre,fra,fr-FR",
		"de": "de,ger,deu,de-DE",
		"it": "it,ita,it-IT",
		"ja": "ja,jpn,jp",
		"ko": "ko,kor,kr",
		"zh": "zh,chi,zho,zh-CN,zh-TW,cmn",
		"ar": "ar,ara",
		"ru": "ru,rus",
		"hi": "hi,hin",
	}
	if v, ok := aliases[code]; ok {
		return v
	}
	return code
}

func (p *MPV) stopLocked() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Signal(syscall.SIGTERM)
	}
	if p.ipcPath != "" {
		_ = os.Remove(p.ipcPath)
		p.ipcPath = ""
	}
}

func (p *MPV) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
	return nil
}

func (p *MPV) Pause() error {
	return p.ipc(`["set_property","pause",true]`)
}

func (p *MPV) Resume() error {
	return p.ipc(`["set_property","pause",false]`)
}

func (p *MPV) Position() (time.Duration, error) {
	raw, err := p.ipcQuery(`["get_property","time-pos"]`)
	if err != nil {
		return 0, err
	}
	var resp struct {
		Data float64 `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, err
	}
	return time.Duration(resp.Data * float64(time.Second)), nil
}

func (p *MPV) Duration() (time.Duration, error) {
	raw, err := p.ipcQuery(`["get_property","duration"]`)
	if err != nil {
		return 0, err
	}
	var resp struct {
		Data float64 `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, err
	}
	return time.Duration(resp.Data * float64(time.Second)), nil
}

func (p *MPV) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *MPV) ipc(cmd string) error {
	_, err := p.ipcQuery(cmd)
	return err
}

func (p *MPV) ipcQuery(cmd string) ([]byte, error) {
	p.mu.Lock()
	path := p.ipcPath
	running := p.running
	p.mu.Unlock()
	if !running || path == "" {
		return nil, fmt.Errorf("mpv not running")
	}
	conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(800 * time.Millisecond))
	if _, err := fmt.Fprintf(conn, "%s\n", cmd); err != nil {
		return nil, err
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}
