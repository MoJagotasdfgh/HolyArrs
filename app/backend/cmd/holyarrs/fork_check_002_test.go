package main

// HolyArrs fork, check for patch 002 (private defaults): a fresh database
// gets Your own keys, no install count and no autoplaying trailers; an
// owner's own choice stays; main() seeds before it reads; the update check
// reads the fork's releases.

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/HolyArrs/holyarrs/app/backend/internal/core/database"
	"github.com/HolyArrs/holyarrs/app/backend/internal/health"
	"github.com/HolyArrs/holyarrs/app/backend/internal/metadata"
)

func TestForkCheck002FreshDatabase(t *testing.T) {
	ctx := context.Background()
	st, err := database.Open(t.TempDir(), database.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := forkSeedPrivate(ctx, st); err != nil {
		t.Fatal(err)
	}
	if m, ok := metadata.StoredMode(ctx, st.Read()); !ok || m != metadata.ModeOwn {
		t.Fatalf("stored mode %q %v, want own", m, ok)
	}
	modes := metadata.NewModes(func(string) string { return "" })
	modes.LoadMode(ctx, st.Read())
	if modes.Current() != metadata.ModeOwn {
		t.Fatalf("mode in force %q, want own (the metadata server is never contacted)", modes.Current())
	}
	if health.CountInstall(ctx, st.Read()) {
		t.Fatal("Count this install is on")
	}
	if v, _ := database.Setting(ctx, st.Read(), "this_app", "autoplay_trailers"); v != "off" {
		t.Fatalf("autoplay trailers %q", v)
	}
}

func TestForkCheck002OwnerChoiceKept(t *testing.T) {
	ctx := context.Background()
	st, err := database.Open(t.TempDir(), database.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	_ = st.Write(ctx, func(tx *sql.Tx) error {
		return database.PutSetting(tx, "this_app", health.SettingCountryCount, "on", database.System, time.Now())
	})
	if err := forkSeedPrivate(ctx, st); err != nil {
		t.Fatal(err)
	}
	if !health.CountInstall(ctx, st.Read()) {
		t.Fatal("the owner's choice was overwritten")
	}
}

func TestForkCheck002Wiring(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	seed, load, count := strings.Index(s, "forkHosted(ctx, s, vault)"), strings.Index(s, "hosted.Modes.LoadMode("), strings.Index(s, "health.CountInstall(")
	if seed < 0 || load < 0 || count < 0 || seed > load || seed > count {
		t.Fatalf("seed at %d, mode read at %d, count read at %d: the seed must come first", seed, load, count)
	}
	if !strings.Contains(s, `env("HOLYARRS_RELEASE_FEED", forkReleaseFeed)`) || !strings.Contains(forkReleaseFeed, "/repos/MoJagotasdfgh/HolyArrs/") {
		t.Fatal("the update check does not read the fork's releases")
	}
}
