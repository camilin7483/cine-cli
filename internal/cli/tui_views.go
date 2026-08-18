package cli

import (
	"fmt"
	"strings"

	"github.com/cam/cine-cli/internal/core"
	"github.com/cam/cine-cli/internal/i18n"
)

func (m *model) View() string {
	var body string
	if m.sidebarOpen {
		body = m.sidebarView() + "\n" + m.mainView()
	} else {
		body = m.mainView()
	}
	return body + "\n" + m.statusLine()
}

func (m *model) mainView() string {
	switch m.screen {
	case screenSearch:
		return m.searchView()
	case screenResults:
		return m.resultsView()
	case screenTrending:
		return m.trendingView()
	case screenSeasons:
		return m.seasonsView()
	case screenEpisodes:
		return m.episodesView()
	case screenResolving:
		return m.resolvingView()
	case screenSources:
		return m.sourcesView()
	case screenPlaying:
		return m.playingView()
	case screenBrowser:
		return m.browserView()
	case screenHelp:
		return m.helpView()
	case screenFavorites:
		return m.favoritesView()
	case screenWatchlist:
		return m.watchlistView()
	case screenHistory:
		return m.historyView()
	}
	return ""
}

func (m *model) statusLine() string {
	parts := []string{}
	if m.app.Player != nil && m.app.Player.Running() {
		parts = append(parts, "▶ playing")
	}
	if m.statusMsg != "" {
		parts = append(parts, m.statusMsg)
	}
	if m.err != "" {
		parts = append(parts, "⚠ "+m.err)
	}
	parts = append(parts, fmt.Sprintf("lang:%s", m.audioLang))
	parts = append(parts, "? help · esc back · q quit")
	line := " " + strings.Join(parts, "  ·  ") + " "
	return s.dim.Render(line)
}

func (m *model) sidebarView() string {
	var b strings.Builder
	items := []struct {
		label string
		icon  string
	}{
		{i18n.T("common.search"), "🔍"},
		{i18n.T("tui.trending"), "🔥"},
		{i18n.T("tui.favorites"), "♥"},
		{i18n.T("tui.watchlist"), "📋"},
		{i18n.T("tui.history"), "⏱"},
		{i18n.T("tui.help"), "?"},
	}

	b.WriteString(s.sidebar.Render(" ── " + i18n.T("tui.menu") + " ── "))
	b.WriteString("\n\n")

	for i, item := range items {
		prefix := "  "
		style := s.sidebar
		if i == m.sidebarIdx {
			prefix = "> "
			style = s.sidebarSel
		}
		b.WriteString(style.Render(fmt.Sprintf("%s%s %s", prefix, item.icon, item.label)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(s.dim.Render("  " + i18n.T("tui.navigate")))
	b.WriteString("\n")
	b.WriteString(s.dim.Render("  " + i18n.T("tui.enter_select")))
	b.WriteString("\n")
	b.WriteString(s.dim.Render("  " + i18n.T("tui.esc_close")))

	return s.border.Render(b.String())
}

func (m *model) searchView() string {
	var b strings.Builder

	b.WriteString(s.title.Render("🎬  cine-cli"))
	if m.app.Config.TMDBAPIKey != "" {
		b.WriteString(" " + s.badge.Render("TMDB"))
	}
	b.WriteString(s.separator.Render(" · "))
	b.WriteString(s.key.Render(fmt.Sprintf("🔊%s", m.audioLang)))
	b.WriteString(s.separator.Render(" · "))
	b.WriteString(s.dim.Render("s menu · ? help"))
	b.WriteString("\n")
	b.WriteString(s.dim.Render("   developed by "))
	b.WriteString(camiloDevStyle.Render("CamiloDev"))
	b.WriteString("\n\n")

	prompt := fmt.Sprintf("%s%s_", i18n.T("tui.search_prompt"), m.search)
	b.WriteString(s.item.Render(prompt))

	if m.app.Config.TMDBAPIKey == "" {
		b.WriteString("\n\n")
		b.WriteString(s.err.Render(i18n.T("tui.tmdb_required", "~/.config/cine-cli/config.yaml")))
	}

	if len(m.continueW) > 0 {
		b.WriteString("\n\n")
		b.WriteString(s.subtitle.Render(i18n.T("tui.continue_watching")))
		b.WriteString("\n")
		n := len(m.continueW)
		if n > 5 {
			n = 5
		}
		for i := 0; i < n; i++ {
			cw := m.continueW[i]
			pct := int(cw.Percentage)
			if pct > 100 {
				pct = 100
			}
			label := ""
			if cw.MediaType == core.MediaTypeSeries && cw.Season > 0 {
				label = fmt.Sprintf(" S%02dE%02d", cw.Season, cw.Episode)
			}
			line := fmt.Sprintf("  %s%s", cw.Title, label)
			b.WriteString(s.item.Render(line))
			b.WriteString("\n")
			b.WriteString("  " + progressBar(pct, 20) + "\n")
		}
	}

	// Home: show trending preview when not searching
	if len(m.search) == 0 && len(m.trending) > 0 {
		b.WriteString("\n")
		b.WriteString(s.subtitle.Render("🔥 " + i18n.T("tui.trending")))
		b.WriteString("\n")
		n := len(m.trending)
		if n > 6 {
			n = 6
		}
		for i := 0; i < n; i++ {
			it := m.trending[i]
			mt := "[M]"
			if it.MediaType == core.MediaTypeSeries {
				mt = "[TV]"
			}
			year := ""
			if it.Year > 0 {
				year = fmt.Sprintf(" (%d)", it.Year)
			}
			b.WriteString(s.item.Render(fmt.Sprintf("  %s %s%s", mt, it.Title, year)))
			b.WriteString("\n")
		}
		b.WriteString(s.dim.Render("  " + i18n.T("tui.home_trending_hint")))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if len(m.search) > 0 {
		b.WriteString(s.dim.Render(i18n.T("tui.search_active_hint")))
	} else {
		b.WriteString(s.dim.Render(i18n.T("tui.home_hint")))
	}

	return b.String()
}

func (m *model) trendingView() string {
	var b strings.Builder

	b.WriteString(s.title.Render("🔥 " + i18n.T("tui.trending")))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(s.spinner.Render(chars[m.spinnerIdx] + " " + i18n.T("tui.loading")))
		return b.String()
	}

	if len(m.trending) == 0 {
		b.WriteString(s.dim.Render(i18n.T("tui.no_trending")))
		b.WriteString("\n\n" + s.dim.Render(i18n.T("tui.esc_back")))
		return b.String()
	}

	m.renderMediaList(&b, m.trending)

	b.WriteString("\n" + s.dim.Render(m.scrollHint(len(m.trending))))
	b.WriteString("  " + s.dim.Render(i18n.T("tui.move")+" · "+i18n.T("tui.enter_play")+" · "+i18n.T("tui.esc_back")+" · "+i18n.T("tui.help_question")))
	return b.String()
}

func (m *model) resultsView() string {
	var b strings.Builder

	b.WriteString(s.title.Render(i18n.T("tui.results_for", m.search)))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(s.spinner.Render(chars[m.spinnerIdx] + " " + i18n.T("tui.searching")))
		return b.String()
	}

	if m.err != "" {
		b.WriteString(s.err.Render(m.err))
		b.WriteString("\n\n" + s.dim.Render(i18n.T("tui.esc_back")))
		return b.String()
	}

	if len(m.results) == 0 {
		b.WriteString(s.dim.Render(i18n.T("tui.no_results")))
		b.WriteString("\n\n" + s.dim.Render(i18n.T("tui.esc_back")+" · "+i18n.T("tui.help_question")))
		return b.String()
	}

	if m.viewMode == "split" && m.selected != nil {
		var split strings.Builder
		split.WriteString(m.renderSplitList())
		split.WriteString("\n")
		split.WriteString(m.detailBlock())
		b.WriteString(split.String())
	} else {
		m.renderMediaList(&b, m.results)
		b.WriteString("\n" + s.dim.Render(m.scrollHint(len(m.results))))
		b.WriteString("  " + s.dim.Render(i18n.T("tui.move")+" · "+i18n.T("tui.enter_play")+" · B browser · tab split · F fav · D download · "+i18n.T("tui.s_menu")+" · "+i18n.T("tui.esc_back")+" · "+i18n.T("tui.help_question")))
	}

	return b.String()
}

func (m *model) renderSplitList() string {
	var b strings.Builder
	total := len(m.results)
	half := m.visibleCount() / 2
	start := m.scrollIdx
	if start+half > total {
		start = total - half
		if start < 0 {
			start = 0
		}
	}
	end := start + half
	if end > total {
		end = total
	}

	for i := start; i < end; i++ {
		media := m.results[i]
		prefix := "  "
		style := s.item
		if i == m.cursor {
			prefix = "> "
			style = s.sel
		}
		mt := "[M]"
		if media.MediaType == core.MediaTypeSeries {
			mt = "[TV]"
		}
		fav := ""
		if i == m.cursor && m.isFav {
			fav = " ♥"
		}
		line := fmt.Sprintf("%s%s%s %s", prefix, mt, fav, truncate(media.Title, 25))
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}
	return b.String()
}

func (m *model) renderMediaList(b *strings.Builder, items []core.Media) {
	total := len(items)
	start := m.scrollIdx
	end := start + m.visibleCount()
	if end > total {
		end = total
	}

	for i := start; i < end; i++ {
		media := items[i]
		prefix := "  "
		style := s.item
		if i == m.cursor {
			prefix = "> "
			style = s.sel
		}

		mt := "[M]"
		if media.MediaType == core.MediaTypeSeries {
			mt = "[TV]"
		}

		fav := ""
		if i == m.cursor && m.isFav {
			fav = " ♥"
		}

		stars := ""
		if media.Rating > 0 && media.Rating <= 10 {
			stars = fmt.Sprintf(" ★%.1f", media.Rating)
		} else if media.Rating > 10 {
			stars = fmt.Sprintf(" #%d", int(media.Rating))
		}

		line := fmt.Sprintf("%s%s%s %s (%d)%s", prefix, mt, fav, media.Title, media.Year, stars)
		if len([]rune(line)) > 80 {
			r := []rune(line)
			line = string(r[:77]) + "..."
		}
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}
}

func (m *model) scrollHint(total int) string {
	if total <= visibleItems {
		return fmt.Sprintf("(%d results)  ", total)
	}
	return fmt.Sprintf("(%d-%d of %d)  ", m.scrollIdx+1, minInt(m.scrollIdx+visibleItems, total), total)
}

func (m *model) detailBlock() string {
	var b strings.Builder
	var lines []string

	titleLine := fmt.Sprintf("   %s", m.selected.Title)
	if m.isFav {
		titleLine += " ♥"
	}
	lines = append(lines, s.rating.Render(titleLine))

	if m.selected.Tagline != "" {
		lines = append(lines, s.dim.Render(fmt.Sprintf("   %s", m.selected.Tagline)))
	}

	lines = append(lines, s.item.Render(fmt.Sprintf("   ID: %d   Type: %s   Year: %d", m.selected.TMDBID, m.selected.MediaType, m.selected.Year)))

	if m.selected.Runtime > 0 {
		h := m.selected.Runtime / 60
		mins := m.selected.Runtime % 60
		runtime := fmt.Sprintf("%dh %dm", h, mins)
		lines = append(lines, s.info.Render(fmt.Sprintf("   Runtime: %s", runtime)))
	}

	if m.selected.Status != "" {
		lines = append(lines, s.info.Render(fmt.Sprintf("   Status: %s", m.selected.Status)))
	}

	if len(m.selected.Genres) > 0 {
		lines = append(lines, s.info.Render(fmt.Sprintf("   Genres: %s", strings.Join(m.selected.Genres, ", "))))
	}

	if m.selected.Rating > 0 {
		ratingStr := fmt.Sprintf("   Rating: %.1f/10", m.selected.Rating)
		lines = append(lines, s.rating.Render(ratingStr))
	}

	lines = append(lines, "")
	overview := m.selected.Overview
	if len([]rune(overview)) > 300 {
		r := []rune(overview)
		overview = string(r[:297]) + "..."
	}
	lines = append(lines, s.item.Render(fmt.Sprintf("   %s", overview)))

	b.WriteString(s.border.Render(strings.Join(lines, "\n")))
	return b.String()
}

func (m *model) seasonsView() string {
	var b strings.Builder

	b.WriteString(s.title.Render(fmt.Sprintf("%s — Seasons", m.selected.Title)))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(s.spinner.Render(chars[m.spinnerIdx] + " Loading..."))
		return b.String()
	}

	total := len(m.seasons)
	start := m.scrollIdx
	end := start + m.visibleCount()
	if end > total {
		end = total
	}

	for i := start; i < end; i++ {
		season := m.seasons[i]
		prefix := "  "
		style := s.item
		if i == m.cursor {
			prefix = "> "
			style = s.sel
		}
		line := fmt.Sprintf("%sSeason %d (%d eps)", prefix, season.SeasonNumber, season.EpisodeCount)
		if season.Name != "" && season.Name != fmt.Sprintf("Season %d", season.SeasonNumber) {
			line = fmt.Sprintf("%sSeason %d — %s (%d eps)", prefix, season.SeasonNumber, season.Name, season.EpisodeCount)
		}
		b.WriteString(style.Render(line))
		b.WriteString("\n")

		if season.Overview != "" {
			overview := truncate(season.Overview, 60)
			b.WriteString(s.dim.Render(fmt.Sprintf("     %s", overview)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n" + s.dim.Render(m.scrollHint(len(m.seasons))))
	b.WriteString("  " + s.dim.Render("move · enter select · esc back"))
	return b.String()
}

func (m *model) episodesView() string {
	var b strings.Builder

	// Header always pinned (title is written first, list is height-clipped)
	title := m.selected.Title
	if title == "" {
		title = "Series"
	}
	b.WriteString(s.title.Render(fmt.Sprintf("%s — S%02d", title, m.season)))
	b.WriteString(s.dim.Render(fmt.Sprintf("  %d episodios", len(m.episodes))))
	b.WriteString("\n")
	b.WriteString(s.dim.Render("  enter reproduce · ↑↓ mueve · esc atrás"))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(s.spinner.Render(chars[m.spinnerIdx] + " Loading..."))
		return b.String()
	}

	total := len(m.episodes)
	// Single-line rows → more episodes fit, header never pushed off-screen
	vis := m.visibleCount()
	start := m.scrollIdx
	end := start + vis
	if end > total {
		end = total
	}

	for i := start; i < end; i++ {
		ep := m.episodes[i]
		prefix := "  "
		style := s.item
		if i == m.cursor {
			prefix = "> "
			style = s.sel
		}
		name := ep.Name
		if name == "" {
			name = fmt.Sprintf("Episode %d", ep.EpisodeNumber)
		}
		// one line only — overview only for the selected row (compact)
		line := fmt.Sprintf("%sE%02d  %s", prefix, ep.EpisodeNumber, name)
		if len([]rune(line)) > 70 {
			r := []rune(line)
			line = string(r[:67]) + "..."
		}
		b.WriteString(style.Render(line))
		b.WriteString("\n")
		if i == m.cursor && ep.Overview != "" {
			b.WriteString(s.dim.Render("     " + truncate(ep.Overview, 64)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n" + s.dim.Render(m.scrollHint(len(m.episodes))))
	return b.String()
}

func (m *model) resolvingView() string {
	var b strings.Builder

	b.WriteString(s.title.Render(fmt.Sprintf("%s %s", chars[m.spinnerIdx], i18n.T("tui.resolving"))))
	b.WriteString("\n\n")

	if m.selected != nil {
		b.WriteString(s.item.Render("  " + m.selected.Title))
		if m.selected.MediaType == core.MediaTypeSeries {
			b.WriteString(s.item.Render(fmt.Sprintf("  S%02dE%02d", m.season, m.episode)))
		}
	}

	b.WriteString("\n\n")
	b.WriteString(s.dim.Render("  " + i18n.T("tui.trying_providers")))
	providers := m.app.Manager.ListProviders()
	seen := map[string]bool{}
	var ordered []string
	if m.app.Config.Provider != "" {
		ordered = append(ordered, m.app.Config.Provider)
		seen[m.app.Config.Provider] = true
	}
	for _, p := range providers {
		if !seen[p] {
			ordered = append(ordered, p)
			seen[p] = true
		}
	}
	b.WriteString("\n")
	for _, p := range ordered {
		b.WriteString(s.spinner.Render(fmt.Sprintf("    %s %s", chars[m.spinnerIdx], p)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(s.dim.Render("  " + i18n.T("tui.extracting")))
	return b.String()
}

func (m *model) sourcesView() string {
	var b strings.Builder

	b.WriteString(s.title.Render(i18n.T("tui.select_source")))
	b.WriteString("\n\n")

	if m.selected != nil {
		b.WriteString(s.item.Render("  " + m.selected.Title))
		if m.selected.MediaType == core.MediaTypeSeries && m.season > 0 {
			b.WriteString(s.item.Render(fmt.Sprintf("  S%02dE%02d", m.season, m.episode)))
		}
		b.WriteString("\n\n")
	}

	if len(m.sources) == 0 {
		b.WriteString(s.dim.Render(i18n.T("results.empty")))
		return b.String()
	}

	total := len(m.sources)
	start := m.scrollIdx
	end := start + m.visibleCount()
	if end > total {
		end = total
	}

	for i := start; i < end; i++ {
		c := m.sources[i]
		prefix := "  "
		style := s.item
		if i == m.cursor {
			prefix = "> "
			style = s.sel
		}
		q := c.Stream.Quality
		if q == "" {
			q = "?"
		}
		line := fmt.Sprintf("%s%-12s  %-8s  score:%d", prefix, c.Provider, q, c.Score)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n" + s.dim.Render(m.scrollHint(len(m.sources))))
	b.WriteString("  " + s.dim.Render(i18n.T("tui.source_hint")))
	return b.String()
}

func (m *model) playingView() string {
	var b strings.Builder

	b.WriteString(s.title.Render("▶ " + i18n.T("tui.now_playing")))
	b.WriteString("\n\n")

	if m.selected != nil {
		b.WriteString(s.success.Render("  " + m.selected.Title))
		if m.selected.MediaType == core.MediaTypeSeries && m.season > 0 {
			b.WriteString(s.item.Render(fmt.Sprintf("  S%02dE%02d", m.season, m.episode)))
		}
	}

	b.WriteString("\n\n")
	b.WriteString(s.info.Render(fmt.Sprintf("  Player: %s  ·  Audio: %s  ·  Subs: %s", m.app.Config.Player, m.audioLang, m.subsLang)))
	b.WriteString("\n\n")
	b.WriteString(s.dim.Render("  Press any key to return"))

	return b.String()
}

func (m *model) browserView() string {
	var b strings.Builder

	b.WriteString(s.title.Render(i18n.T("tui.browser")))
	b.WriteString("\n\n")

	if m.selected != nil {
		url := buildBrowserURL(m.selected, m.season, m.episode)
		b.WriteString(s.item.Render("  " + m.selected.Title))
		b.WriteString("\n\n")
		b.WriteString(s.dim.Render("  " + url))
		b.WriteString("\n\n")
		b.WriteString(s.dim.Render("  " + i18n.T("common.back")))
	}

	return b.String()
}

func (m *model) favoritesView() string {
	var b strings.Builder

	b.WriteString(s.title.Render("♥ " + i18n.T("favorites.title")))
	b.WriteString("\n\n")

	if len(m.favs) == 0 {
		b.WriteString(s.dim.Render(i18n.T("favorites.empty")))
		b.WriteString("\n\n" + s.dim.Render(i18n.T("favorites.hint")))
		return b.String()
	}

	total := len(m.favs)
	start := m.scrollIdx
	end := start + m.visibleCount()
	if end > total {
		end = total
	}

	for i := start; i < end; i++ {
		fav := m.favs[i]
		prefix := "  "
		style := s.item
		if i == m.cursor {
			prefix = "> "
			style = s.sel
		}
		mt := "[M]"
		if fav.MediaType == core.MediaTypeSeries {
			mt = "[TV]"
		}
		line := fmt.Sprintf("%s%s %s", prefix, mt, fav.Title)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n" + s.dim.Render(m.scrollHint(len(m.favs))))
	b.WriteString("  " + s.dim.Render("move · enter play · esc back"))
	return b.String()
}

func (m *model) watchlistView() string {
	var b strings.Builder

	b.WriteString(s.title.Render("📋 " + i18n.T("watchlist.title")))
	b.WriteString("\n\n")

	if len(m.watchlist) == 0 {
		b.WriteString(s.dim.Render(i18n.T("watchlist.empty")))
		b.WriteString("\n\n" + s.dim.Render(i18n.T("common.back")))
		return b.String()
	}

	total := len(m.watchlist)
	start := m.scrollIdx
	end := start + m.visibleCount()
	if end > total {
		end = total
	}

	for i := start; i < end; i++ {
		item := m.watchlist[i]
		prefix := "  "
		style := s.item
		if i == m.cursor {
			prefix = "> "
			style = s.sel
		}
		mt := "[M]"
		if item.MediaType == core.MediaTypeSeries {
			mt = "[TV]"
		}
		status := ""
		if item.Status != "" && item.Status != "plan_to_watch" {
			status = fmt.Sprintf(" (%s)", item.Status)
		}
		line := fmt.Sprintf("%s%s %s%s", prefix, mt, item.Title, status)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n" + s.dim.Render(m.scrollHint(len(m.watchlist))))
	b.WriteString("  " + s.dim.Render("move · enter play · esc back"))
	return b.String()
}

func (m *model) historyView() string {
	var b strings.Builder

	b.WriteString(s.title.Render("⏱ " + i18n.T("history.title")))
	b.WriteString("\n\n")

	if len(m.history) == 0 {
		b.WriteString(s.dim.Render(i18n.T("history.empty")))
		b.WriteString("\n\n" + s.dim.Render(i18n.T("common.back")))
		return b.String()
	}

	total := len(m.history)
	start := m.scrollIdx
	end := start + m.visibleCount()
	if end > total {
		end = total
	}

	for i := start; i < end; i++ {
		entry := m.history[i]
		prefix := "  "
		style := s.item
		if i == m.cursor {
			prefix = "> "
			style = s.sel
		}
		mt := "[M]"
		if entry.MediaType == core.MediaTypeSeries {
			mt = "[TV]"
		}
		ep := ""
		if entry.Season > 0 && entry.Episode > 0 {
			ep = fmt.Sprintf(" S%02dE%02d", entry.Season, entry.Episode)
		}
		when := ""
		if !entry.WatchedAt.IsZero() {
			when = " " + entry.WatchedAt.Format("Jan 2")
		}
		line := fmt.Sprintf("%s%s %s%s%s", prefix, mt, entry.Title, ep, when)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n" + s.dim.Render(m.scrollHint(len(m.history))))
	b.WriteString("  " + s.dim.Render("move · enter replay · esc back"))
	return b.String()
}

func (m *model) helpView() string {
	var b strings.Builder

	b.WriteString(s.title.Render(i18n.T("help.title")))
	b.WriteString("\n\n")

	global := []struct{ key, desc string }{
		{"s", i18n.T("help.key.sidebar")},
		{"t", i18n.T("help.key.trending")},
		{"l", i18n.T("help.key.lang")},
		{"?", i18n.T("help.key.help")},
		{"esc", i18n.T("help.key.esc")},
		{"q / ctrl+c", i18n.T("help.key.quit")},
	}

	navigation := []struct{ key, desc string }{
		{"↑/k", i18n.T("help.key.up")},
		{"↓/j", i18n.T("help.key.down")},
		{"enter", i18n.T("help.key.enter")},
	}

	actions := []struct{ key, desc string }{
		{"tab/space", i18n.T("help.key.split")},
		{"b", i18n.T("help.key.browser")},
		{"d", i18n.T("help.key.download")},
		{"f", i18n.T("help.key.favorite")},
		{"v", i18n.T("help.key.play_list")},
	}

	searchHelp := []struct{ key, desc string }{
		{"type + enter", i18n.T("help.key.type")},
		{"backspace", i18n.T("help.key.backspace")},
	}

	b.WriteString(s.subtitle.Render(" " + i18n.T("help.section.global")))
	b.WriteString("\n")
	for _, h := range global {
		fmt.Fprintf(&b, "  %-16s  %s\n", s.helpKey.Render(h.key), s.helpDesc.Render(h.desc))
	}

	b.WriteString("\n" + s.subtitle.Render(" "+i18n.T("help.section.nav")))
	b.WriteString("\n")
	for _, h := range navigation {
		fmt.Fprintf(&b, "  %-16s  %s\n", s.helpKey.Render(h.key), s.helpDesc.Render(h.desc))
	}

	b.WriteString("\n" + s.subtitle.Render(" "+i18n.T("help.section.search")))
	b.WriteString("\n")
	for _, h := range searchHelp {
		fmt.Fprintf(&b, "  %-16s  %s\n", s.helpKey.Render(h.key), s.helpDesc.Render(h.desc))
	}

	b.WriteString("\n" + s.subtitle.Render(" "+i18n.T("help.section.actions")))
	b.WriteString("\n")
	for _, h := range actions {
		fmt.Fprintf(&b, "  %-16s  %s\n", s.helpKey.Render(h.key), s.helpDesc.Render(h.desc))
	}

	b.WriteString("\n" + s.dim.Render(i18n.T("tui.press_close")))

	return b.String()
}

func progressBar(pct int, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := pct * width / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return s.progress.Render(bar) + " " + s.dim.Render(fmt.Sprintf("%d%%", pct))
}

func truncate(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
