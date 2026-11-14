package sources

// HolyArrs fork (MoJagotasdfgh/HolyArrs), patch 001: the four built-in site
// readers are taken out and this file is an empty plug-in slot. A site
// reader is a Go file of the owner's in this package that calls
// RegisterSite from init(); the fork ships none. The Source format "site",
// its routes and its stored rows are unchanged, so a reader plugged in
// later works with Sources added before. Undo: revert this patch's commit.

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	domain "github.com/HolyArrs/holyarrs/app/backend/internal/core/model"
	"github.com/HolyArrs/holyarrs/app/backend/internal/core/releasename"
)

// Site readers: a Source with format "site" reads one plugged-in site
// directly, with no Prowlarr or Jackett in between. Extra["site"] names it.
const extraSiteKind = "site"

// Site describes one plugged-in site for the add form.
type Site struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Private bool   `json:"private"`
	// Kinds are the media types the site serves.
	Kinds []string `json:"kinds"`
	// Limits are polite defaults: one call at a time with a gap, a cap per hour.
	GapMS     int `json:"gap_ms"`
	PerHour   int `json:"per_hour"`
	TimeoutMS int `json:"timeout_ms"`
	// NoFeed means the site only searches: it has no newest list to watch.
	NoFeed bool `json:"no_feed,omitempty"`
}

// SiteReader is one site plugged into the slot.
type SiteReader struct {
	Site Site
	// Search runs one search; text "" asks for the newest list. base is the
	// Source's address without a trailing slash; c.send and c.wait are there
	// for the reader's calls.
	Search func(ctx context.Context, c *Client, base, text string, q Query) ([]Item, error)
	// Test, when set, is the query a Source test runs; nil reads the newest list.
	Test *Query
	// FileHeader, when set, gives the headers the site needs to hand out a
	// release file, from the Source's key. It is never put in a link.
	FileHeader func(key string) http.Header
}

// Sites lists the plugged-in sites, in the order they were registered.
// Empty as shipped.
var Sites = []Site{}

var siteReaders = map[string]SiteReader{}

// RegisterSite plugs a reader into the slot. Call it from init(); an empty
// id, a missing Search or a second reader with the same id stops the start.
func RegisterSite(r SiteReader) {
	if r.Site.ID == "" || r.Search == nil {
		panic("sources: a site reader needs an id and a Search")
	}
	if _, dup := siteReaders[r.Site.ID]; dup {
		panic("sources: site reader registered twice: " + r.Site.ID)
	}
	siteReaders[r.Site.ID] = r
	Sites = append(Sites, r.Site)
}

// SiteByID finds a plugged-in site.
func SiteByID(id string) (Site, bool) {
	r, ok := siteReaders[id]
	return r.Site, ok
}

func (c *Client) siteCaps() (Caps, error) {
	s, ok := SiteByID(c.cfg.Site)
	if !ok {
		return Caps{}, &Error{Kind: KindBroken, Msg: "unknown built-in site"}
	}
	caps := Caps{Search: true, TVSearch: slices.Contains(s.Kinds, "tv"), MovieSearch: slices.Contains(s.Kinds, "movie"),
		MusicSearch: slices.Contains(s.Kinds, "music"), Params: map[string][]string{"search": {"q"}}}
	for _, k := range s.Kinds {
		caps.Categories = append(caps.Categories, DefaultCategories[k]...)
	}
	return caps, nil
}

// siteText is the words a built-in site is searched with: the title and, for
// a show, the season and episode (S01E02, S01, or a daily date); for a movie
// the year; for an album the artist and album.
func siteText(q Query) string {
	if q.feed {
		return ""
	}
	if q.Music {
		return strings.TrimSpace(q.Artist + " " + q.Album)
	}
	t := textQuery(Query{Title: q.Title})
	if q.TV {
		if q.Words != "" {
			return strings.TrimSpace(t + " " + q.Words)
		}
		switch {
		case strings.Contains(q.Episode, "/"):
			return strings.TrimSpace(fmt.Sprintf("%s %d %s", t, q.Season, strings.ReplaceAll(q.Episode, "/", " ")))
		case q.Episode != "" && q.Season > 0:
			if n, err := strconv.Atoi(q.Episode); err == nil {
				return fmt.Sprintf("%s S%02dE%02d", t, q.Season, n)
			}
		case q.Season > 0:
			return fmt.Sprintf("%s S%02d", t, q.Season)
		}
		return t
	}
	if q.Year > 0 {
		return fmt.Sprintf("%s %d", t, q.Year)
	}
	return t
}

func (c *Client) siteSearch(ctx context.Context, q Query, observer func(string, time.Duration, error)) ([]Item, error) {
	op := "search"
	if q.feed {
		op = "feed"
	}
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	base := strings.TrimRight(c.cfg.URL, "/")
	text := siteText(q)
	start := time.Now()
	items, err := c.siteCall(ctx, base, text, q)
	if observer != nil && ctx.Err() == nil {
		observer(op, time.Since(start), err)
	}
	return items, err
}

func (c *Client) siteCall(ctx context.Context, base, text string, q Query) ([]Item, error) {
	r, ok := siteReaders[c.cfg.Site]
	if !ok {
		return nil, &Error{Kind: KindBroken, Msg: "unknown built-in site"}
	}
	return r.Search(ctx, c, base, text, q)
}

// magnetHash reads the info hash of a magnet's xt, in hex or base32.
func magnetHash(xt string) string {
	h, ok := strings.CutPrefix(strings.ToLower(xt), "urn:btih:")
	if !ok {
		return ""
	}
	if releasename.ValidInfoHash(h) {
		return h
	}
	if len(h) == 32 {
		const alpha = "abcdefghijklmnopqrstuvwxyz234567"
		var bits, n uint
		var out []byte
		for _, r := range h {
			i := strings.IndexRune(alpha, r)
			if i < 0 {
				return ""
			}
			bits = bits<<5 | uint(i)
			n += 5
			if n >= 8 {
				n -= 8
				out = append(out, byte(bits>>n))
				bits &= 1<<n - 1
			}
		}
		return fmt.Sprintf("%x", out)
	}
	return ""
}

// siteTest reads the site's newest list, or runs the reader's own test query.
func (c *Client) siteTest(ctx context.Context) (int, error) {
	q := Query{feed: true}
	if r, ok := siteReaders[c.cfg.Site]; ok && r.Test != nil {
		q = *r.Test
	}
	items, err := c.siteSearch(ctx, q, c.cfg.Observe)
	return len(items), err
}

// FileHeader gives the headers src's own site needs to hand out a Release
// file, when its reader has any (a sign-in cookie, say). Nil for every other
// Source, and the header is never put in a link.
func FileHeader(ctx context.Context, src domain.Source, sec SecretReader) (http.Header, error) {
	r, ok := siteReaders[src.Extra[extraSiteKind]]
	if !ok || r.FileHeader == nil || src.Format != "site" {
		return nil, nil
	}
	cfg, err := ConfigFor(ctx, src, sec)
	if err != nil {
		return nil, err
	}
	if h := r.FileHeader(cfg.APIKey); len(h) > 0 {
		return h, nil
	}
	return nil, nil
}

// FileHeaders is FileHeader reading keys from sec, as the grab step asks.
func FileHeaders(sec SecretReader) func(context.Context, domain.Source) (http.Header, error) {
	return func(ctx context.Context, src domain.Source) (http.Header, error) { return FileHeader(ctx, src, sec) }
}
