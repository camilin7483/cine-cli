package player

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cam/cine-cli/internal/core"
)

type VLC struct {
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	mu      sync.Mutex
	running bool
	args    []string
	rcHost  string
	rcPort  int
}

func NewVLC(extraArgs []string) *VLC {
	return &VLC{args: extraArgs, rcPort: 4212}
}

func (p *VLC) Name() string { return "vlc" }

func (p *VLC) Play(ctx context.Context, opts core.PlayOptions) error {
	p.mu.Lock()
	if p.running {
		p.stopLocked()
		time.Sleep(300 * time.Millisecond)
	}

	p.rcPort = 4212 + int(time.Now().UnixNano()%100)
	p.rcHost = fmt.Sprintf("127.0.0.1:%d", p.rcPort)

	args := []string{
		"--play-and-exit",
		"--no-video-title-show",
		"--extraintf", "rc",
		"--rc-host", p.rcHost,
		"--rc-quiet",
	}

	if opts.Referer != "" {
		args = append(args, fmt.Sprintf("--http-referrer=%s", opts.Referer))
	}
	if opts.PreferredLang != "" {
		args = append(args, fmt.Sprintf("--audio-language=%s", opts.PreferredLang))
	}
	if opts.SubsLang != "" {
		args = append(args, fmt.Sprintf("--sub-language=%s", opts.SubsLang))
	}
	for _, sub := range opts.Subtitles {
		if sub.URL != "" {
			args = append(args, "--sub-file="+sub.URL)
		}
	}
	for _, extra := range opts.ExtraArgs {
		if strings.HasPrefix(extra, "--start=") {
			sec := strings.TrimPrefix(extra, "--start=")
			args = append(args, "--start-time="+sec)
			continue
		}
		args = append(args, extra)
	}

	args = append(args, p.args...)
	args = append(args, opts.StreamURL)

	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	cmd := exec.CommandContext(ctx, "vlc", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil

	p.cmd = cmd
	p.running = true

	if err := cmd.Start(); err != nil {
		p.running = false
		p.mu.Unlock()
		return fmt.Errorf("vlc start: %w", err)
	}
	p.mu.Unlock()

	go func() {
		_ = cmd.Wait()
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !p.Running() {
			return fmt.Errorf("vlc exited immediately — stream may be invalid")
		}
		if _, err := p.rcCommand("status"); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

func (p *VLC) stopLocked() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Signal(os.Interrupt)
	}
}

func (p *VLC) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
	return nil
}

func (p *VLC) Pause() error {
	_, err := p.rcCommand("pause")
	return err
}

func (p *VLC) Resume() error {
	_, err := p.rcCommand("play")
	return err
}

func (p *VLC) Position() (time.Duration, error) {
	out, err := p.rcCommand("get_time")
	if err != nil {
		return 0, err
	}
	sec, err := parseRCInt(out)
	if err != nil {
		return 0, err
	}
	return time.Duration(sec) * time.Second, nil
}

func (p *VLC) Duration() (time.Duration, error) {
	out, err := p.rcCommand("get_length")
	if err != nil {
		return 0, err
	}
	sec, err := parseRCInt(out)
	if err != nil {
		return 0, err
	}
	return time.Duration(sec) * time.Second, nil
}

func (p *VLC) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *VLC) rcCommand(cmd string) (string, error) {
	p.mu.Lock()
	host := p.rcHost
	running := p.running
	p.mu.Unlock()

	if !running || host == "" {
		return "", fmt.Errorf("vlc not running or no RC")
	}

	conn, err := net.DialTimeout("tcp", host, 400*time.Millisecond)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(800 * time.Millisecond))

	if _, err := fmt.Fprintf(conn, "%s\n", cmd); err != nil {
		return "", err
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		buf := make([]byte, 256)
		n, _ := conn.Read(buf)
		if n > 0 {
			return strings.TrimSpace(string(buf[:n])), nil
		}
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func parseRCInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty rc response")
	}
	for i := len(fields) - 1; i >= 0; i-- {
		n, err := strconv.Atoi(fields[i])
		if err == nil {
			return n, nil
		}
	}
	return strconv.Atoi(s)
}

type PlayerSwitch struct {
	mpv *MPV
	vlc *VLC
}

func NewPlayerSwitch(mpvArgs, vlcArgs []string) *PlayerSwitch {
	return &PlayerSwitch{
		mpv: NewMPV(mpvArgs),
		vlc: NewVLC(vlcArgs),
	}
}

func (ps *PlayerSwitch) Play(ctx context.Context, opts core.PlayOptions) error {
	switch opts.Player {
	case "vlc":
		return ps.vlc.Play(ctx, opts)
	default:
		return ps.mpv.Play(ctx, opts)
	}
}

func (ps *PlayerSwitch) Stop() error {
	_ = ps.mpv.Stop()
	_ = ps.vlc.Stop()
	return nil
}

func (ps *PlayerSwitch) Name() string { return "player" }

func (ps *PlayerSwitch) Pause() error {
	if ps.vlc.Running() {
		return ps.vlc.Pause()
	}
	return ps.mpv.Pause()
}

func (ps *PlayerSwitch) Resume() error {
	if ps.vlc.Running() {
		return ps.vlc.Resume()
	}
	return ps.mpv.Resume()
}

func (ps *PlayerSwitch) Position() (time.Duration, error) {
	if ps.vlc.Running() {
		return ps.vlc.Position()
	}
	return ps.mpv.Position()
}

func (ps *PlayerSwitch) Duration() (time.Duration, error) {
	if ps.vlc.Running() {
		return ps.vlc.Duration()
	}
	return ps.mpv.Duration()
}

func (ps *PlayerSwitch) Running() bool {
	return ps.mpv.Running() || ps.vlc.Running()
}
