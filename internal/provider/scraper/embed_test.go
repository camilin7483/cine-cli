package scraper

import "testing"

func TestParseSE(t *testing.T) {
	s, e := parseSE("123/2/5")
	if s != 2 || e != 5 {
		t.Fatalf("got %d %d", s, e)
	}
	s, e = parseSE("99")
	if s != 1 || e != 1 {
		t.Fatalf("defaults got %d %d", s, e)
	}
}

func TestTmdbPart(t *testing.T) {
	if tmdbPart("555/1/2") != "555" {
		t.Fatal(tmdbPart("555/1/2"))
	}
}
