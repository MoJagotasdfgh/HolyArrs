package main

// HolyArrs fork (MoJagotasdfgh/HolyArrs), patch 002: private defaults.
//
// On every start, before the metadata mode and "Count this install" are
// read, the settings this fork keeps private are written when the owner has
// not set them yet; a choice made in Settings is never overwritten:
//   - metadata from Your own keys: the HolyArrs metadata server is never
//     contacted (upstream's default is the server);
//   - "Count this install" off (the server's per-country install total);
//   - trailers do not play by themselves, so the trailer pass stays quiet.
// The update check reads this fork's releases instead of upstream's.
// No migration: these are ordinary settings rows.

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/HolyArrs/holyarrs/app/backend/internal/core/database"
	"github.com/HolyArrs/holyarrs/app/backend/internal/health"
	"github.com/HolyArrs/holyarrs/app/backend/internal/metadata"
	"github.com/HolyArrs/holyarrs/app/backend/internal/core/secrets"
)

// forkReleaseFeed is where the update check looks for a newer release.
const forkReleaseFeed = "https://api.github.com/repos/MoJagotasdfgh/HolyArrs/releases/latest"

// forkPrivateSettings are the settings rows written when missing.
var forkPrivateSettings = []struct{ grp, key, value string }{
	{metadata.ModeGroup, metadata.ModeKey, string(metadata.ModeOwn)},
	{"this_app", health.SettingCountryCount, "off"},
	{"this_app", "autoplay_trailers", "off"},
}

func init() { forkPatches = append(forkPatches, "002-private-defaults") }

// forkHosted writes the private defaults, then makes the metadata server
// client as upstream does. main() calls it in place of metadata.NewHosted,
// right before the mode and "Count this install" are read.
func forkHosted(ctx context.Context, s *database.Store, vault *secrets.Vault) (*metadata.Hosted, error) {
	if err := forkPrivateDefaults(ctx, s); err != nil {
		return nil, fmt.Errorf("fork defaults: %w", err)
	}
	return metadata.NewHosted(vault)
}

// forkPrivateDefaults runs at start. Upstream's own tests start the server
// too and expect upstream's defaults, so under go test it does nothing;
// fork_check_002_test.go tests forkSeedPrivate itself.
func forkPrivateDefaults(ctx context.Context, s *database.Store) error {
	if testing.Testing() {
		return nil
	}
	return forkSeedPrivate(ctx, s)
}

// forkSeedPrivate writes the private settings that are not set yet.
func forkSeedPrivate(ctx context.Context, s *database.Store) error {
	return s.Write(ctx, func(tx *sql.Tx) error {
		now := time.Now()
		for _, x := range forkPrivateSettings {
			if err := database.PutSettingIfMissing(tx, x.grp, x.key, x.value, database.System, now); err != nil {
				return err
			}
		}
		return nil
	})
}
