package core

import "testing"

func TestQualityScore(t *testing.T) {
	if QualityScore("1080p") <= QualityScore("720p") {
		t.Fatal("1080p should score higher than 720p")
	}
	if QualityScore("4k") <= QualityScore("1080p") {
		t.Fatal("4k should score higher than 1080p")
	}
	if QualityScore("unknown") != 0 {
		t.Fatal("unknown quality should be 0")
	}
}
