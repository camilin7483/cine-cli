package stats

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cam/cine-cli/internal/config"
	"github.com/cam/cine-cli/internal/database"
)

type Dashboard struct {
	TotalHoursWatched float64
	TotalMovies       int
	TotalEpisodes     int
	TotalShows        int
	FavoriteGenres    map[string]int
	TopActors         map[string]int
	MostUsedProviders map[string]int
	WeeklyActivity    map[string]int
	Favorites         int
	Watchlist         int
	Downloads         int
	ContinueWatching  int
	StreakDays        int
	MostWatched       string

	db  *database.Store
	cfg *config.Config
}

func NewDashboard(db *database.Store, cfg *config.Config) *Dashboard {
	return &Dashboard{
		db:                db,
		cfg:               cfg,
		FavoriteGenres:    make(map[string]int),
		TopActors:         make(map[string]int),
		MostUsedProviders: make(map[string]int),
		WeeklyActivity:    make(map[string]int),
	}
}

func (d *Dashboard) Collect(ctx context.Context) error {
	stats, err := d.db.Stats(ctx)
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}
	d.TotalMovies = stats.TotalMovies
	d.TotalShows = stats.TotalShows
	d.TotalEpisodes = stats.TotalEpisodes

	var totalSeconds float64
	err = d.db.DB().QueryRowContext(ctx,
		`SELECT COALESCE(SUM(position), 0) FROM history WHERE position > 0`,
	).Scan(&totalSeconds)
	if err != nil {
		return fmt.Errorf("hours: %w", err)
	}
	d.TotalHoursWatched = totalSeconds / 3600

	provRows, err := d.db.DB().QueryContext(ctx,
		`SELECT provider, COUNT(*) as cnt FROM history WHERE provider != '' GROUP BY provider ORDER BY cnt DESC`,
	)
	if err != nil {
		return fmt.Errorf("providers: %w", err)
	}
	for provRows.Next() {
		var provider string
		var cnt int
		if err := provRows.Scan(&provider, &cnt); err != nil {
			provRows.Close()
			return fmt.Errorf("providers scan: %w", err)
		}
		d.MostUsedProviders[provider] = cnt
	}
	provRows.Close()
	if err := provRows.Err(); err != nil {
		return fmt.Errorf("providers iter: %w", err)
	}

	weekRows, err := d.db.DB().QueryContext(ctx,
		`SELECT CAST(strftime('%w', watched_at) AS INTEGER), COUNT(*) FROM history GROUP BY 1`,
	)
	if err != nil {
		return fmt.Errorf("weekly: %w", err)
	}
	weekdayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	for weekRows.Next() {
		var dow int
		var cnt int
		if err := weekRows.Scan(&dow, &cnt); err != nil {
			weekRows.Close()
			return fmt.Errorf("weekly scan: %w", err)
		}
		if dow >= 0 && dow < 7 {
			d.WeeklyActivity[weekdayNames[dow]] = cnt
		}
	}
	weekRows.Close()
	if err := weekRows.Err(); err != nil {
		return fmt.Errorf("weekly iter: %w", err)
	}

	var title string
	var watchCount int
	err = d.db.DB().QueryRowContext(ctx,
		`SELECT title, COUNT(*) as cnt FROM history GROUP BY title ORDER BY cnt DESC LIMIT 1`,
	).Scan(&title, &watchCount)
	if err != nil {
		d.MostWatched = ""
	} else {
		d.MostWatched = title
	}

	d.StreakDays = d.calculateStreak(ctx)

	favs, err := d.db.CountFavorites(ctx)
	if err != nil {
		return fmt.Errorf("favorites: %w", err)
	}
	d.Favorites = favs

	wl, err := d.db.CountWatchlist(ctx)
	if err != nil {
		return fmt.Errorf("watchlist: %w", err)
	}
	d.Watchlist = wl

	err = d.db.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM downloads`,
	).Scan(&d.Downloads)
	if err != nil {
		return fmt.Errorf("downloads: %w", err)
	}

	cw, err := d.db.ListContinueWatching(ctx, 0)
	if err != nil {
		return fmt.Errorf("continue: %w", err)
	}
	d.ContinueWatching = len(cw)

	return nil
}

func (d *Dashboard) calculateStreak(ctx context.Context) int {
	rows, err := d.db.DB().QueryContext(ctx,
		`SELECT DISTINCT date(watched_at) FROM history ORDER BY date(watched_at) DESC`,
	)
	if err != nil {
		return 0
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err != nil {
			return 0
		}
		dates = append(dates, date)
	}
	if len(dates) == 0 {
		return 0
	}

	today := time.Now().Truncate(24 * time.Hour)
	recentDate, err := time.Parse("2006-01-02", dates[0])
	if err != nil {
		return 0
	}
	recentDate = recentDate.Truncate(24 * time.Hour)

	if today.Sub(recentDate).Hours()/24 > 1 {
		return 0
	}

	streak := 1
	prevDate := recentDate
	for i := 1; i < len(dates); i++ {
		d, err := time.Parse("2006-01-02", dates[i])
		if err != nil {
			break
		}
		d = d.Truncate(24 * time.Hour)
		if prevDate.Sub(d).Hours()/24 == 1 {
			streak++
			prevDate = d
		} else {
			break
		}
	}
	return streak
}

func (d *Dashboard) Format() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("  \u2554\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2557\n")
	sb.WriteString("  \u2551          CINE CLI \u2014 STATISTICS           \u2551\n")
	sb.WriteString("  \u255a\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u255d\n")
	sb.WriteString("\n")

	sb.WriteString("  \u2500\u2500 Overview \u2500\u2500\n")
	fmt.Fprintf(&sb, "  Total Hours Watched: %.1f h\n", d.TotalHoursWatched)
	fmt.Fprintf(&sb, "  Movies Watched:      %d\n", d.TotalMovies)
	fmt.Fprintf(&sb, "  TV Episodes Watched: %d\n", d.TotalEpisodes)
	fmt.Fprintf(&sb, "  TV Shows Watched:    %d\n", d.TotalShows)
	fmt.Fprintf(&sb, "  Day Streak:          %d days\n", d.StreakDays)
	if d.MostWatched != "" {
		fmt.Fprintf(&sb, "  Most Watched:        %s\n", d.MostWatched)
	}
	sb.WriteString("\n")

	sb.WriteString("  \u2500\u2500 Collections \u2500\u2500\n")
	fmt.Fprintf(&sb, "  Favorites:          %d\n", d.Favorites)
	fmt.Fprintf(&sb, "  Watchlist:          %d\n", d.Watchlist)
	fmt.Fprintf(&sb, "  Downloads:          %d\n", d.Downloads)
	fmt.Fprintf(&sb, "  Continue Watching:  %d\n", d.ContinueWatching)
	sb.WriteString("\n")

	if len(d.MostUsedProviders) > 0 {
		sb.WriteString("  \u2500\u2500 Most Used Providers \u2500\u2500\n")
		sorted := sortedPairs(d.MostUsedProviders)
		for _, item := range sorted {
			fmt.Fprintf(&sb, "  %s: %d\n", item.key, item.value)
		}
		sb.WriteString("\n")
	}

	if len(d.WeeklyActivity) > 0 {
		sb.WriteString("  \u2500\u2500 Weekly Activity \u2500\u2500\n")
		dowOrder := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
		maxCount := 0
		for _, v := range d.WeeklyActivity {
			if v > maxCount {
				maxCount = v
			}
		}
		for _, dow := range dowOrder {
			count := d.WeeklyActivity[dow]
			bar := buildBar(count, maxCount, 20)
			fmt.Fprintf(&sb, "  %s %s %d\n", dow, bar, count)
		}
		sb.WriteString("\n")
	}

	if len(d.FavoriteGenres) > 0 {
		sb.WriteString("  \u2500\u2500 Favorite Genres \u2500\u2500\n")
		sorted := sortedPairs(d.FavoriteGenres)
		for _, item := range sorted {
			fmt.Fprintf(&sb, "  %s: %d\n", item.key, item.value)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

type pair struct {
	key   string
	value int
}

func sortedPairs(m map[string]int) []pair {
	result := make([]pair, 0, len(m))
	for k, v := range m {
		result = append(result, pair{k, v})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].value > result[j].value })
	return result
}

func buildBar(value, max, width int) string {
	if max == 0 {
		return strings.Repeat(" ", width+2)
	}
	filled := int(float64(value) / float64(max) * float64(width))
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", width-filled) + "]"
}
