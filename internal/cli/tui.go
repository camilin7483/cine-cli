package cli

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/cam/cine-cli/internal/core"
	tea "github.com/charmbracelet/bubbletea"
)

type screen int

const (
	screenSearch screen = iota
	screenResults
	screenSeasons
	screenEpisodes
	screenPlaying
	screenHelp
	screenBrowser
	screenTrending
	screenResolving
	screenSources
	screenFavorites
	screenWatchlist
	screenHistory
)

const visibleItems = 15

var chars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// streamCandidate is a resolved stream ready for user selection.
type streamCandidate struct {
	Provider string
	Stream   *core.Stream
	Score    int
}

type model struct {
	app         *App
	screen      screen
	prevScreen  screen
	err         string
	search      string
	results     []core.Media
	trending    []core.Media
	favs        []core.Favorite
	watchlist   []core.WatchlistItem
	history     []core.HistoryEntry
	continueW   []core.ContinueWatching
	selected    *core.Media
	seasons     []core.Season
	episodes    []core.Episode
	season      int
	episode     int
	cursor      int
	scrollIdx   int
	loading     bool
	detail      bool
	width       int
	height      int
	isFav       bool
	sidebarOpen bool
	sidebarIdx  int
	spinnerIdx  int
	viewMode    string
	themeSet    bool
	audioLang   string
	subsLang    string
	sources     []streamCandidate
	statusMsg   string
}

var audioLangs = []string{"en", "es", "ja", "pt", "fr", "de", "it", "ko", "zh", "ar", "ru", "hi", "original"}

func NewTUI(app *App) *tea.Program {
	initStyles(app.Config.ThemeMode)
	lang := app.Config.SmartSelection.PreferredLang
	if lang == "" {
		lang = "en"
	}
	subs := app.Config.SubtitlesLanguage
	if subs == "" {
		subs = lang
	}
	m := &model{
		app:        app,
		screen:     screenSearch,
		viewMode:   "list",
		sidebarIdx: 0,
		audioLang:  lang,
		subsLang:   subs,
	}
	return tea.NewProgram(m, tea.WithAltScreen())
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.loadTrending(), m.loadContinueWatching(), m.spinnerTick())
}

type trendingMsg []core.Media
type continueWatchingMsg []core.ContinueWatching
type resolvingMsg struct {
	err     string
	sources []streamCandidate
}
type sourcesPlayMsg struct{ err string }
type spinnerTick struct{}
type favoritesMsg []core.Favorite
type watchlistMsg []core.WatchlistItem
type historyMsg []core.HistoryEntry
type downloadResultMsg struct {
	title string
	err   string
}

func (m *model) spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return spinnerTick{}
	})
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = minInt(msg.Width, 120)
		m.height = msg.Height
		if !m.themeSet {
			initStyles(m.app.Config.ThemeMode)
			m.themeSet = true
		}
		return m, nil

	case trendingMsg:
		m.trending = msg
		return m, nil

	case continueWatchingMsg:
		m.continueW = msg
		return m, nil

	case favoritesMsg:
		m.favs = msg
		return m, nil

	case watchlistMsg:
		m.watchlist = msg
		return m, nil

	case historyMsg:
		m.history = msg
		return m, nil

	case downloadResultMsg:
		if msg.err != "" {
			m.err = msg.err
		} else {
			m.err = ""
		}
		return m, nil

	case spinnerTick:
		m.spinnerIdx = (m.spinnerIdx + 1) % len(chars)
		if m.loading {
			return m, m.spinnerTick()
		}
		return m, nil

	case resolvingMsg:
		m.loading = false
		if msg.err != "" {
			m.err = msg.err
			m.statusMsg = msg.err
			m.screen = screenResults
			return m, nil
		}
		if len(msg.sources) == 0 {
			m.err = "no streams found"
			m.screen = screenResults
			return m, nil
		}
		// Sort by quality score desc
		srcs := msg.sources
		for i := 0; i < len(srcs); i++ {
			for j := i + 1; j < len(srcs); j++ {
				if srcs[j].Score > srcs[i].Score {
					srcs[i], srcs[j] = srcs[j], srcs[i]
				}
			}
		}
		m.sources = srcs
		m.cursor = 0
		m.scrollIdx = 0
		if len(srcs) == 1 {
			// Auto-play single source
			return m, m.playCandidate(srcs[0])
		}
		m.screen = screenSources
		m.statusMsg = fmt.Sprintf("%d sources found", len(srcs))
		return m, nil

	case sourcesPlayMsg:
		m.loading = false
		if msg.err != "" {
			m.err = msg.err
			m.statusMsg = msg.err
			m.screen = screenSources
			return m, nil
		}
		m.screen = screenPlaying
		m.statusMsg = "playing"
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *model) loadTrending() tea.Cmd {
	return func() tea.Msg {
		if m.app.Metadata == nil {
			return trendingMsg(nil)
		}
		movies, _ := m.app.Metadata.GetTrending(context.Background(), core.MediaTypeMovie, 1)
		series, _ := m.app.Metadata.GetTrending(context.Background(), core.MediaTypeSeries, 1)
		all := append(movies, series...)
		return trendingMsg(deduplicate(all))
	}
}

func (m *model) loadContinueWatching() tea.Cmd {
	return func() tea.Msg {
		items, err := m.app.DB.ListContinueWatching(context.Background(), 5)
		if err != nil || items == nil {
			return continueWatchingMsg(nil)
		}
		return continueWatchingMsg(items)
	}
}

func (m *model) loadFavorites() tea.Cmd {
	return func() tea.Msg {
		favs, err := m.app.DB.ListFavorites(context.Background())
		if err != nil || favs == nil {
			return favoritesMsg(nil)
		}
		return favoritesMsg(favs)
	}
}

func (m *model) loadWatchlist() tea.Cmd {
	return func() tea.Msg {
		items, err := m.app.DB.ListWatchlist(context.Background())
		if err != nil || items == nil {
			return watchlistMsg(nil)
		}
		return watchlistMsg(items)
	}
}

func (m *model) loadHistory() tea.Cmd {
	return func() tea.Msg {
		entries, err := m.app.DB.List(context.Background(), 50, 0)
		if err != nil || entries == nil {
			return historyMsg(nil)
		}
		return historyMsg(entries)
	}
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if key == "ctrl+c" {
		_ = m.app.Player.Stop()
		return m, tea.Quit
	}
	if key == "q" && m.screen != screenSearch {
		_ = m.app.Player.Stop()
		return m, tea.Quit
	}
	if key == "esc" {
		if m.sidebarOpen {
			m.sidebarOpen = false
			return m, nil
		}
		if m.screen == screenSearch {
			return m, tea.Quit
		}
		if m.screen == screenPlaying {
			_ = m.app.Player.Stop()
		}
		m.goBack()
		return m, nil
	}

	if m.sidebarOpen {
		return m.handleSidebarKey(key)
	}

	if m.screen == screenSearch {
		switch key {
		case "enter":
			if len(strings.TrimSpace(m.search)) > 0 {
				return m, m.doSearch()
			}
			return m, nil
		case "backspace":
			if len(m.search) > 0 {
				m.search = m.search[:len(m.search)-1]
			}
			return m, nil
		case "?":
			m.prevScreen = m.screen
			m.screen = screenHelp
			return m, nil
		case "T":
			m.screen = screenTrending
			m.cursor = 0
			m.scrollIdx = 0
			return m, m.loadTrending()
		case "S":
			m.sidebarOpen = true
			return m, nil
		case "L":
			m.cycleLanguage()
			return m, nil
		default:
			if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
				m.search += strings.ToLower(key)
			}
			return m, nil
		}
	}

	// Normalizar mayúsculas para soportar Shift+key
	lkey := strings.ToLower(key)

	switch lkey {
	case "enter":
		return m.handleEnter()

	case "up", "k":
		m.moveCursor(-1)
		return m, nil

	case "down", "j":
		m.moveCursor(1)
		return m, nil

	case "tab", " ":
		if m.screen == screenResults && m.selected != nil {
			m.detail = !m.detail
			if m.detail {
				m.viewMode = "split"
			} else {
				m.viewMode = "list"
			}
		}
		return m, nil

	case "f":
		if m.screen == screenResults && m.selected != nil {
			return m, m.toggleFavorite()
		}
		return m, nil

	case "d":
		if m.screen == screenResults && m.selected != nil {
			return m, m.startDownload()
		}
		return m, nil

	case "b":
		if m.screen == screenResults || m.screen == screenSeasons || m.screen == screenEpisodes || m.screen == screenPlaying {
			return m, m.openBrowser()
		}
		return m, nil

	case "t":
		if m.screen == screenResults || m.screen == screenSeasons || m.screen == screenEpisodes {
			m.screen = screenTrending
			m.cursor = 0
			m.scrollIdx = 0
			return m, m.loadTrending()
		}
		return m, nil

	case "s":
		m.sidebarOpen = !m.sidebarOpen
		return m, nil

	case "v":
		if m.screen == screenFavorites {
			if len(m.favs) == 0 || m.cursor >= len(m.favs) {
				return m, nil
			}
			fav := m.favs[m.cursor]
			m.selected = &core.Media{
				ID:        fav.MediaID,
				Title:     fav.Title,
				MediaType: fav.MediaType,
				PosterURL: fav.PosterURL,
			}
			return m, m.startResolve()
		}
		return m, nil

	case "l":
		m.cycleLanguage()
		return m, nil

	case "?":
		if m.screen == screenHelp {
			m.screen = m.prevScreen
		} else {
			m.prevScreen = m.screen
			m.screen = screenHelp
		}
		return m, nil

	case "backspace":
		return m, nil

	default:
		if m.screen == screenPlaying || m.screen == screenBrowser {
			m.screen = screenResults
			m.cursor = 0
			m.scrollIdx = 0
			return m, nil
		}
	}
	return m, nil
}

func (m *model) handleSidebarKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		m.sidebarIdx--
		if m.sidebarIdx < 0 {
			m.sidebarIdx = 0
		}
		return m, nil
	case "down", "j":
		maxIdx := 5
		if m.sidebarIdx < maxIdx {
			m.sidebarIdx++
		}
		return m, nil
	case "enter":
		m.sidebarOpen = false
		switch m.sidebarIdx {
		case 0:
			m.screen = screenSearch
			m.cursor = 0
			m.scrollIdx = 0
		case 1:
			m.screen = screenTrending
			m.cursor = 0
			m.scrollIdx = 0
			return m, m.loadTrending()
		case 2:
			m.screen = screenFavorites
			m.cursor = 0
			m.scrollIdx = 0
			return m, m.loadFavorites()
		case 3:
			m.screen = screenWatchlist
			m.cursor = 0
			m.scrollIdx = 0
			return m, m.loadWatchlist()
		case 4:
			m.screen = screenHistory
			m.cursor = 0
			m.scrollIdx = 0
			return m, m.loadHistory()
		case 5:
			m.screen = screenHelp
			m.prevScreen = screenSearch
		}
		return m, nil
	case "esc":
		m.sidebarOpen = false
		return m, nil
	}
	return m, nil
}

func (m *model) visibleCount() int {
	h := m.height
	if h <= 0 {
		h = 24
	}
	n := h - 8
	if n < 5 {
		n = 5
	}
	if n > 30 {
		n = 30
	}
	return n
}

func (m *model) moveCursor(delta int) {
	maxLen := m.listLen()
	if maxLen == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= maxLen {
		m.cursor = maxLen - 1
	}
	vis := m.visibleCount()
	if m.cursor < m.scrollIdx {
		m.scrollIdx = m.cursor
	}
	if m.cursor >= m.scrollIdx+vis {
		m.scrollIdx = m.cursor - vis + 1
	}
}

func (m *model) goBack() {
	switch m.screen {
	case screenResults, screenHelp, screenTrending, screenFavorites, screenWatchlist, screenHistory:
		m.screen = screenSearch
		m.detail = false
		m.viewMode = "list"
		m.err = ""
		m.cursor = 0
		m.scrollIdx = 0
	case screenSeasons:
		m.screen = screenResults
		m.cursor = 0
		m.scrollIdx = 0
	case screenEpisodes:
		m.screen = screenSeasons
		m.cursor = 0
		m.scrollIdx = 0
	case screenSources:
		if m.selected != nil && m.selected.MediaType == core.MediaTypeSeries && m.season > 0 {
			m.screen = screenEpisodes
		} else {
			m.screen = screenResults
		}
		m.cursor = 0
		m.scrollIdx = 0
		m.sources = nil
	case screenPlaying, screenBrowser, screenResolving:
		m.screen = screenResults
		m.cursor = 0
		m.scrollIdx = 0
	}
}

func (m *model) listLen() int {
	switch m.screen {
	case screenResults:
		return len(m.results)
	case screenSeasons:
		return len(m.seasons)
	case screenEpisodes:
		return len(m.episodes)
	case screenTrending:
		return len(m.trending)
	case screenFavorites:
		return len(m.favs)
	case screenWatchlist:
		return len(m.watchlist)
	case screenHistory:
		return len(m.history)
	case screenSources:
		return len(m.sources)
	}
	return 0
}

func (m *model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenSearch:
		if len(strings.TrimSpace(m.search)) > 0 {
			return m, m.doSearch()
		}
		return m, nil

	case screenResults:
		if len(m.results) == 0 || m.cursor >= len(m.results) {
			return m, nil
		}
		sel := &m.results[m.cursor]
		m.selected = sel
		m.isFav = m.checkFavorite(sel.ID)
		m.detail = false
		m.viewMode = "list"
		if sel.MediaType == core.MediaTypeSeries {
			return m, m.loadSeasons()
		}
		return m, m.startResolve()

	case screenTrending:
		if len(m.trending) == 0 || m.cursor >= len(m.trending) {
			return m, nil
		}
		sel := &m.trending[m.cursor]
		m.selected = sel
		m.isFav = m.checkFavorite(sel.ID)
		m.cursor = 0
		m.scrollIdx = 0
		if sel.MediaType == core.MediaTypeSeries {
			return m, m.loadSeasons()
		}
		return m, m.startResolve()

	case screenSeasons:
		if len(m.seasons) == 0 || m.cursor >= len(m.seasons) {
			return m, nil
		}
		m.season = m.seasons[m.cursor].SeasonNumber
		m.cursor = 0
		m.scrollIdx = 0
		return m, m.loadEpisodes()

	case screenEpisodes:
		if len(m.episodes) == 0 || m.cursor >= len(m.episodes) {
			return m, nil
		}
		m.episode = m.episodes[m.cursor].EpisodeNumber
		return m, m.startResolve()

	case screenFavorites:
		if len(m.favs) == 0 || m.cursor >= len(m.favs) {
			return m, nil
		}
		fav := m.favs[m.cursor]
		m.selected = &core.Media{
			ID:        fav.MediaID,
			Title:     fav.Title,
			MediaType: fav.MediaType,
			PosterURL: fav.PosterURL,
		}
		return m, m.startResolve()

	case screenWatchlist:
		if len(m.watchlist) == 0 || m.cursor >= len(m.watchlist) {
			return m, nil
		}
		item := m.watchlist[m.cursor]
		m.selected = &core.Media{
			ID:        item.MediaID,
			Title:     item.Title,
			MediaType: item.MediaType,
		}
		return m, m.startResolve()

	case screenHistory:
		if len(m.history) == 0 || m.cursor >= len(m.history) {
			return m, nil
		}
		entry := m.history[m.cursor]
		m.selected = &core.Media{
			ID:        entry.MediaID,
			Title:     entry.Title,
			MediaType: entry.MediaType,
		}
		return m, m.startResolve()

	case screenSources:
		if len(m.sources) == 0 || m.cursor >= len(m.sources) {
			return m, nil
		}
		return m, m.playCandidate(m.sources[m.cursor])
	}
	return m, nil
}

func (m *model) checkFavorite(mediaID string) bool {
	exists, _ := m.app.DB.FavoriteExists(context.Background(), mediaID)
	return exists
}

func (m *model) cycleLanguage() {
	for i, l := range audioLangs {
		if l == m.audioLang {
			m.audioLang = audioLangs[(i+1)%len(audioLangs)]
			m.subsLang = m.audioLang
			m.app.Config.SmartSelection.PreferredLang = m.audioLang
			m.app.Config.SubtitlesLanguage = m.subsLang
			_ = m.app.Config.Save()
			if m.audioLang == "original" {
				m.statusMsg = "idioma: original (sin preferencia forzada)"
			} else {
				m.statusMsg = fmt.Sprintf("preferencia audio/subs: %s — si el stream no lo trae, MPV usa el disponible", m.audioLang)
			}
			return
		}
	}
	m.audioLang = audioLangs[0]
	m.subsLang = audioLangs[0]
	m.statusMsg = fmt.Sprintf("preferencia audio/subs: %s", m.audioLang)
}

func (m *model) toggleFavorite() tea.Cmd {
	return func() tea.Msg {
		if m.selected == nil {
			return nil
		}
		exists, _ := m.app.DB.FavoriteExists(context.Background(), m.selected.ID)
		if exists {
			_ = m.app.DB.RemoveFavorite(context.Background(), m.selected.ID)
		} else {
			_ = m.app.DB.AddFavorite(context.Background(), core.Favorite{
				MediaID:   m.selected.ID,
				Title:     m.selected.Title,
				MediaType: m.selected.MediaType,
				PosterURL: m.selected.PosterURL,
			})
		}
		m.isFav = !exists
		return nil
	}
}

func (m *model) startDownload() tea.Cmd {
	return func() tea.Msg {
		if m.selected == nil {
			return downloadResultMsg{err: "no media selected"}
		}

		tmdbID := fmt.Sprintf("%d", m.selected.TMDBID)
		providerID := tmdbID
		if m.selected.MediaType == core.MediaTypeSeries {
			providerID = fmt.Sprintf("%s/%d/%d", tmdbID, m.season, m.episode)
		}

		providers := m.app.Manager.ListProviders()
		if m.app.Config.Provider != "" {
			providers = append([]string{m.app.Config.Provider}, providers...)
		}

		var streamURL, referer, ua, usedProvider string
		for _, pname := range providers {
			ref := core.MediaRef{
				ProviderName: pname,
				ProviderID:   providerID,
				Title:        m.selected.Title,
				MediaType:    m.selected.MediaType,
			}
			s, err := m.app.Manager.ResolveStream(context.Background(), ref)
			if err == nil && s != nil && s.URL != "" && isStreamURL(s.URL) {
				streamURL = s.URL
				referer = s.Referer
				ua = s.UserAgent
				usedProvider = pname
				break
			}
		}

		if streamURL == "" {
			return downloadResultMsg{err: "Could not resolve stream for download"}
		}

		dl := core.Download{
			MediaID:   m.selected.ID,
			Title:     m.selected.Title,
			MediaType: m.selected.MediaType,
			Season:    m.season,
			Episode:   m.episode,
			URL:       streamURL,
			Referer:   referer,
			UserAgent: ua,
			Quality:   m.app.Config.DefaultQuality,
			Provider:  usedProvider,
		}
		if err := m.app.Downloads.Enqueue(context.Background(), dl); err != nil {
			return downloadResultMsg{err: fmt.Sprintf("Download error: %v", err)}
		}
		return downloadResultMsg{title: m.selected.Title}
	}
}

func (m *model) doSearch() tea.Cmd {
	return func() tea.Msg {
		query := strings.TrimSpace(m.search)
		if len(query) == 0 {
			return nil
		}
		m.loading = true
		results, err := m.app.Metadata.Search(context.Background(), core.SearchFilter{Query: query})
		if err != nil {
			m.err = err.Error()
			m.loading = false
			return nil
		}
		m.results = deduplicate(results)
		m.cursor = 0
		m.scrollIdx = 0
		m.loading = false
		m.screen = screenResults
		return nil
	}
}

func (m *model) loadSeasons() tea.Cmd {
	return func() tea.Msg {
		m.loading = true
		seasons, err := m.app.Metadata.GetSeasons(context.Background(), m.selected.TMDBID)
		if err != nil {
			m.err = err.Error()
			m.loading = false
			return nil
		}
		var filtered []core.Season
		for _, s := range seasons {
			if s.SeasonNumber > 0 {
				filtered = append(filtered, s)
			}
		}
		m.seasons = filtered
		m.cursor = 0
		m.scrollIdx = 0
		m.loading = false
		m.screen = screenSeasons
		return nil
	}
}

func (m *model) loadEpisodes() tea.Cmd {
	return func() tea.Msg {
		m.loading = true
		episodes, err := m.app.Metadata.GetEpisodes(context.Background(), m.selected.TMDBID, m.season)
		if err != nil {
			m.err = err.Error()
			m.loading = false
			return nil
		}
		m.episodes = episodes
		m.cursor = 0
		m.scrollIdx = 0
		m.loading = false
		m.screen = screenEpisodes
		return nil
	}
}

func (m *model) startResolve() tea.Cmd {
	m.screen = screenResolving
	m.loading = true
	m.err = ""
	m.sources = nil
	m.statusMsg = "resolving..."
	return m.resolveStream()
}

func (m *model) resolveStream() tea.Cmd {
	return func() tea.Msg {
		tmdbID := fmt.Sprintf("%d", m.selected.TMDBID)
		providerID := tmdbID
		if m.selected.MediaType == core.MediaTypeSeries {
			providerID = fmt.Sprintf("%s/%d/%d", tmdbID, m.season, m.episode)
		}

		providers := m.app.Manager.ListProviders()
		if m.app.Config.Provider != "" {
			seen := map[string]bool{m.app.Config.Provider: true}
			ordered := []string{m.app.Config.Provider}
			for _, p := range providers {
				if !seen[p] {
					ordered = append(ordered, p)
					seen[p] = true
				}
			}
			providers = ordered
		}

		type result struct {
			name string
			s    *core.Stream
		}
		ch := make(chan result, len(providers))
		var wg sync.WaitGroup
		overall, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		for _, pname := range providers {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				pctx, pcancel := context.WithTimeout(overall, 40*time.Second)
				defer pcancel()
				ref := core.MediaRef{
					ProviderName: name,
					ProviderID:   providerID,
					Title:        m.selected.Title,
					MediaType:    m.selected.MediaType,
				}
				s, err := m.app.Manager.ResolveStream(pctx, ref)
				if err != nil || s == nil || s.URL == "" {
					return
				}
				if !isStreamURL(s.URL) && !strings.HasPrefix(s.URL, "http") {
					return
				}
				ch <- result{name: name, s: s}
			}(pname)
		}
		go func() {
			wg.Wait()
			close(ch)
		}()

		var candidates []streamCandidate
		seenURL := map[string]bool{}
		for r := range ch {
			if seenURL[r.s.URL] {
				continue
			}
			seenURL[r.s.URL] = true
			candidates = append(candidates, streamCandidate{
				Provider: r.name,
				Stream:   r.s,
				Score:    core.QualityScore(r.s.Quality),
			})
		}

		if len(candidates) == 0 {
			browserURL := buildBrowserURL(m.selected, m.season, m.episode)
			return resolvingMsg{err: fmt.Sprintf("No se pudo extraer stream. Pulsa b para abrir en navegador: %s", browserURL)}
		}
		return resolvingMsg{sources: candidates}
	}
}

func (m *model) playCandidate(c streamCandidate) tea.Cmd {
	m.loading = true
	m.screen = screenResolving
	return func() tea.Msg {
		stream := c.Stream
		subs := stream.Subtitles
		if m.app.Config.SubtitlesEnabled && m.app.Subs != nil && m.selected != nil {
			if extra, err := m.app.Subs.Find(context.Background(), m.selected.TMDBID, m.selected.MediaType, m.season, m.episode); err == nil && len(extra) > 0 {
				subs = append(subs, extra...)
			}
		}
		opts := core.PlayOptions{
			StreamURL:     stream.URL,
			Referer:       stream.Referer,
			UserAgent:     stream.UserAgent,
			Subtitles:     subs,
			Title:         m.selected.Title,
			Player:        m.app.Config.Player,
			ExtraArgs:     append([]string{}, m.app.Config.MPVArgs...),
			PreferredLang: m.audioLang,
			SubsLang:      m.subsLang,
		}

		progress, _ := m.app.DB.GetProgress(context.Background(), m.selected.ID, m.season, m.episode)
		if progress != nil && !progress.Completed && progress.Position > 5 {
			opts.ExtraArgs = append(opts.ExtraArgs, fmt.Sprintf("--start=%.1f", progress.Position))
		}

		if err := m.app.Player.Play(context.Background(), opts); err != nil {
			browserURL := buildBrowserURL(m.selected, m.season, m.episode)
			return sourcesPlayMsg{err: fmt.Sprintf("Reproducción falló (%v). Pulsa b para navegador: %s", err, browserURL)}
		}

		_ = m.app.DB.Add(context.Background(), core.HistoryEntry{
			MediaID:   m.selected.ID,
			Title:     m.selected.Title,
			MediaType: m.selected.MediaType,
			Season:    m.season,
			Episode:   m.episode,
			Provider:  c.Provider,
			StreamURL: stream.URL,
		})

		// Track progress in background while player runs
		go m.app.trackProgress(context.Background(), m.selected.ID, m.selected.Title,
			m.selected.MediaType, m.season, m.episode, c.Provider, stream.URL)

		return sourcesPlayMsg{}
	}
}

func isStreamURL(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, ".m3u8") ||
		strings.Contains(lower, ".mp4") ||
		strings.Contains(lower, ".mkv") ||
		strings.Contains(lower, "/video/") ||
		strings.Contains(lower, "/stream/")
}

func buildBrowserURL(media *core.Media, season, episode int) string {
	tmdbIDStr := fmt.Sprintf("%d", media.TMDBID)
	if media.MediaType == core.MediaTypeSeries {
		return fmt.Sprintf("https://vidsrc.to/embed/tv/%s/%d/%d", tmdbIDStr, season, episode)
	}
	return fmt.Sprintf("https://vidsrc.to/embed/movie/%s", tmdbIDStr)
}

func (m *model) openBrowser() tea.Cmd {
	return func() tea.Msg {
		url := buildBrowserURL(m.selected, m.season, m.episode)
		_ = exec.Command("xdg-open", url).Start()
		m.screen = screenBrowser
		return nil
	}
}

func deduplicate(media []core.Media) []core.Media {
	seen := make(map[string]bool)
	var result []core.Media
	for _, m := range media {
		key := m.Title
		if m.Year > 0 {
			key = fmt.Sprintf("%s-%d", m.Title, m.Year)
		}
		if !seen[key] {
			seen[key] = true
			result = append(result, m)
		}
	}
	return result
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
