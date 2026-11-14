package sources

import "testing"

// HolyArrs fork, patch 001: the tests of the four built-in site readers left
// with the readers; the slot's own check is fork_check_001_test.go.

func TestSiteText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		q    Query
		want string
	}{
		{Query{Title: "Big Buck Bunny", Year: 2008}, "Big Buck Bunny 2008"},
		{Query{Title: "Show", TV: true, Season: 1, Episode: "2"}, "Show S01E02"},
		{Query{Title: "Show", TV: true, Season: 3}, "Show S03"},
		{Query{Title: "Daily", TV: true, Season: 2026, Episode: "11/12"}, "Daily 2026 11 12"},
		{Query{Music: true, Artist: "Artist", Album: "Album"}, "Artist Album"},
		{Query{feed: true, Title: "x"}, ""},
	}
	for _, c := range cases {
		if got := siteText(c.q); got != c.want {
			t.Errorf("siteText(%+v) = %q, want %q", c.q, got, c.want)
		}
	}
}

func TestMagnetHashBase32(t *testing.T) {
	t.Parallel()
	if got := magnetHash("urn:btih:VLJ3XGIWPFSZPFC3WRVIL2UCEQFHAW2B"); len(got) != 40 {
		t.Fatalf("hash %q", got)
	}
	if got := magnetHash("urn:btih:DD8255ECDC7CA55FB0BBF81323D87062DB1F6D1C"); got != "dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c" {
		t.Fatalf("hash %q", got)
	}
}
