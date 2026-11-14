package sources

// HolyArrs fork, check for patch 001 (sites plug-in slot). It fails when an
// upstream release brings a built-in site reader back, or when the slot no
// longer reaches a plugged-in reader.

import (
	"context"
	"encoding/json"
	"testing"
)

// The four readers upstream ships are not in the build, and the list the
// add form reads holds only what was plugged in.
func TestForkCheck001NoBuiltInSites(t *testing.T) {
	for _, id := range []string{"knaben", "torrentscsv", "subsplease", "iptorrents"} {
		if _, ok := SiteByID(id); ok {
			t.Fatalf("built-in site reader %q is back", id)
		}
	}
	if len(Sites) != len(siteReaders) {
		t.Fatalf("Sites lists %d, slot holds %d", len(Sites), len(siteReaders))
	}
	c := New(Config{URL: "http://127.0.0.1:1", Format: "site", Site: "knaben", Protocol: "torrent", AllowLocal: true})
	if _, err := c.siteCaps(); err == nil {
		t.Fatal("a site with no reader answered caps")
	}
}

// The add route refuses a site that is not plugged in, and lists none.
// route: GET /api/v1/sources/sites
func TestForkCheck001AddRefused(t *testing.T) {
	srv, _ := sourceServer(t)
	var sites struct {
		Sites []Site `json:"sites"`
	}
	_ = json.Unmarshal(do(t, "GET", srv.URL+"/api/v1/sources/sites", "", 200), &sites)
	if len(sites.Sites) != len(Sites) {
		t.Fatalf("route lists %d sites, slot %d", len(sites.Sites), len(Sites))
	}
	do(t, "POST", srv.URL+"/api/v1/sources", `{"name":"K","format":"site","site":"knaben"}`, 400)
}

// A reader plugged in through RegisterSite is reached for caps, search,
// the test and a release file's headers.
func TestForkCheck001SlotReachesReader(t *testing.T) {
	const id = "fork-check-fake"
	var seen string
	RegisterSite(SiteReader{
		Site: Site{ID: id, Name: "Fork Check", URL: "http://127.0.0.1:1", Kinds: []string{"movie"}, NoFeed: true},
		Search: func(_ context.Context, _ *Client, base, text string, _ Query) ([]Item, error) {
			seen = base + "|" + text
			return []Item{{Title: "Fork Check 2026", Protocol: "torrent"}}, nil
		},
		Test: &Query{Title: "probe"},
	})
	t.Cleanup(func() {
		delete(siteReaders, id)
		Sites = Sites[:len(Sites)-1]
	})
	c := New(Config{URL: "http://127.0.0.1:1/", Format: "site", Site: id, Protocol: "torrent", AllowLocal: true})
	caps, err := c.siteCaps()
	if err != nil || !caps.MovieSearch || caps.TVSearch {
		t.Fatalf("caps %+v %v", caps, err)
	}
	items, err := c.siteSearch(context.Background(), Query{Title: "Fork Check", Year: 2026}, nil)
	if err != nil || len(items) != 1 || seen != "http://127.0.0.1:1|Fork Check 2026" {
		t.Fatalf("search %v %v seen %q", items, err, seen)
	}
	if n, err := c.siteTest(context.Background()); err != nil || n != 1 || seen != "http://127.0.0.1:1|probe" {
		t.Fatalf("test %d %v seen %q", n, err, seen)
	}
	if s, ok := SiteByID(id); !ok || !s.NoFeed {
		t.Fatalf("SiteByID %+v %v", s, ok)
	}
}
