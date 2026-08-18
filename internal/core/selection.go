package core

type SelectionRule struct {
	Name     string `json:"name"`
	Field    string `json:"field"`    // "quality", "language", "subtitles", "provider"
	Operator string `json:"operator"` // "eq", "contains", "gt", "lt", "prefer"
	Value    string `json:"value"`
}

type SelectionConfig struct {
	Rules          []SelectionRule `json:"rules"`
	PreferredQuality string        `json:"preferred_quality"`
	PreferredLang    string        `json:"preferred_lang"`
	AutoSubtitles    bool          `json:"auto_subtitles"`
	MinBandwidth     int           `json:"min_bandwidth"`
}

func DefaultSelectionConfig() SelectionConfig {
	return SelectionConfig{
		PreferredQuality: "best",
		PreferredLang:    "en",
		AutoSubtitles:    true,
		MinBandwidth:     0,
	}
}

var QualityOrder = []string{"4k", "2160p", "1080p", "720p", "480p", "360p"}

func QualityScore(q string) int {
	for i, v := range QualityOrder {
		if v == q {
			return len(QualityOrder) - i
		}
	}
	return 0
}
