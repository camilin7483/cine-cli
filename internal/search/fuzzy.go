package search

import (
	"sort"
	"strings"
	"unicode"

	"github.com/cam/cine-cli/internal/core"
)

func FuzzyMatch(query, target string) float64 {
	query = strings.ToLower(strings.TrimSpace(query))
	target = strings.ToLower(strings.TrimSpace(target))

	if query == "" || target == "" {
		return 0.0
	}
	if query == target {
		return 1.0
	}

	levScore := 1.0 - normalizedLevenshtein(query, target)
	triScore := trigramSimilarity(query, target)

	if triScore < 0.15 {
		return levScore * 0.5
	}
	if triScore < 0.3 {
		return levScore*0.35 + triScore*0.35 + 0.3*containsBonus(query, target)
	}

	return levScore*0.4 + triScore*0.6
}

func LevenshteinDistance(a, b string) int {
	return levenshteinDistance([]rune(a), []rune(b))
}

func levenshteinDistance(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for i := 0; i <= lb; i++ {
		prev[i] = i
	}

	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min3(
				prev[j]+1,
				cur[j-1]+1,
				prev[j-1]+cost,
			)
		}
		prev, cur = cur, prev
	}

	return prev[lb]
}

func normalizedLevenshtein(a, b string) float64 {
	ra := []rune(a)
	rb := []rune(b)
	maxLen := len(ra)
	if len(rb) > maxLen {
		maxLen = len(rb)
	}
	if maxLen == 0 {
		return 0
	}
	dist := levenshteinDistance(ra, rb)
	return float64(dist) / float64(maxLen)
}

func TrigramSimilarity(a, b string) float64 {
	return trigramSimilarity(a, b)
}

func trigramSimilarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0.0
	}

	ta := buildTrigrams(a)
	tb := buildTrigrams(b)

	if len(ta) == 0 && len(tb) == 0 {
		return 0.0
	}
	if len(ta) == 0 || len(tb) == 0 {
		return 0.0
	}

	intersection := 0
	for t := range ta {
		if tb[t] {
			intersection++
		}
	}

	union := len(ta) + len(tb) - intersection
	if union == 0 {
		return 0.0
	}

	dice := 2.0 * float64(intersection) / float64(len(ta)+len(tb))

	containerScore := 0.0
	if strings.Contains(a, b) || strings.Contains(b, a) {
		shorter := len([]rune(a))
		longer := len([]rune(b))
		if len([]rune(b)) < shorter {
			shorter = len([]rune(b))
			longer = len([]rune(a))
		}
		if longer > 0 {
			containerScore = float64(shorter) / float64(longer) * 0.3
		}
	}

	result := dice*0.85 + containerScore
	if result > 1.0 {
		result = 1.0
	}
	return result
}

func buildTrigrams(s string) map[string]bool {
	trigrams := make(map[string]bool)
	runes := []rune(s)
	for i := 0; i < len(runes)-2; i++ {
		trigrams[string(runes[i:i+3])] = true
	}
	return trigrams
}

func containsBonus(query, target string) float64 {
	if strings.Contains(target, query) {
		qlen := float64(len([]rune(query)))
		tlen := float64(len([]rune(target)))
		if tlen > 0 {
			return qlen / tlen
		}
	}
	return 0.0
}

func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= a && b <= c {
		return b
	}
	return c
}

type scoredMedia struct {
	media core.Media
	score float64
}

func FuzzyFilter(results []core.Media, query string) []core.Media {
	if query == "" {
		return results
	}

	query = strings.TrimSpace(query)
	words := splitWords(query)

	scored := make([]scoredMedia, 0, len(results))
	for _, m := range results {
		score := scoreMedia(m, query, words)
		if score > 0.1 || query == "" {
			scored = append(scored, scoredMedia{media: m, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	filtered := make([]core.Media, len(scored))
	for i, s := range scored {
		filtered[i] = s.media
	}
	return filtered
}

func scoreMedia(m core.Media, query string, words []string) float64 {
	titleScore := FuzzyMatch(query, m.Title)

	origScore := 0.0
	if m.OriginalTitle != "" {
		origScore = FuzzyMatch(query, m.OriginalTitle) * 0.9
	}

	overviewScore := 0.0
	if m.Overview != "" {
		overviewScore = FuzzyMatch(query, m.Overview) * 0.3
	}

	var wordScore float64
	for _, w := range words {
		ws := FuzzyMatch(w, m.Title)
		if ws > wordScore {
			wordScore = ws
		}
	}
	wordScore *= 0.95

	best := titleScore
	if origScore > best {
		best = origScore
	}
	if wordScore > best {
		best = wordScore
	}
	if overviewScore > best {
		best = overviewScore
	}

	if strings.EqualFold(query, m.Title) {
		best = 1.0
	} else if strings.HasPrefix(strings.ToLower(m.Title), strings.ToLower(query)) {
		best += 0.1
		if best > 1.0 {
			best = 1.0
		}
	}

	return best
}

type scoredMediaRef struct {
	ref   core.MediaRef
	score float64
}

func FuzzyFilterRefs(results []core.MediaRef, query string) []core.MediaRef {
	if query == "" {
		return results
	}

	query = strings.TrimSpace(query)
	words := splitWords(query)

	scored := make([]scoredMediaRef, 0, len(results))
	for _, r := range results {
		score := scoreMediaRef(r, query, words)
		if score > 0.1 || query == "" {
			scored = append(scored, scoredMediaRef{ref: r, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	filtered := make([]core.MediaRef, len(scored))
	for i, s := range scored {
		filtered[i] = s.ref
	}
	return filtered
}

func scoreMediaRef(ref core.MediaRef, query string, words []string) float64 {
	titleScore := FuzzyMatch(query, ref.Title)

	var wordScore float64
	for _, w := range words {
		ws := FuzzyMatch(w, ref.Title)
		if ws > wordScore {
			wordScore = ws
		}
	}
	wordScore *= 0.95

	best := titleScore
	if wordScore > best {
		best = wordScore
	}

	if strings.EqualFold(query, ref.Title) {
		best = 1.0
	} else if strings.HasPrefix(strings.ToLower(ref.Title), strings.ToLower(query)) {
		best += 0.1
		if best > 1.0 {
			best = 1.0
		}
	}

	return best
}

type scoredString struct {
	value string
	score float64
}

func FuzzyMatchMulti(query string, targets []string) []string {
	if query == "" {
		return targets
	}

	query = strings.TrimSpace(query)

	scored := make([]scoredString, 0, len(targets))
	for _, t := range targets {
		score := FuzzyMatch(query, t)
		if score > 0.1 || query == "" {
			scored = append(scored, scoredString{value: t, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	result := make([]string, len(scored))
	for i, s := range scored {
		result[i] = s.value
	}
	return result
}

func splitWords(s string) []string {
	var words []string
	current := ""

	for _, r := range s {
		if unicode.IsSpace(r) {
			if current != "" {
				words = append(words, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		words = append(words, current)
	}
	return words
}
