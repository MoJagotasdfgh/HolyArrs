# HolyArrs fork recipe (MoJagotasdfgh/HolyArrs)

This fork is **a recipe applied to a pinned upstream release**, not a fork
that drifts: small numbered patches on top of an untouched HolyArrs tag,
re-applied by a script when upstream moves (the way Morphe and ReVanced
patch apps). HolyArrs is by the HolyArrs authors
(https://github.com/HolyArrs/HolyArrs), AGPL-3.0; the licence file at the
top of this repository applies to everything here, the recipe included.

## Branches

- `upstream`: the pristine upstream tag the recipe is pinned to (now 1.1.4, `002c680`).
- `main`: upstream's main at the pin (`9b6c0af`: the tag plus README-only
  commits) + this `fork/` folder + one commit per patch. It can be rebuilt
  from the recipe at any time; the patch files are the source of truth, and
  `apply.ps1` always starts from the tag itself.

## What the patches do

| Patch | What | Its check |
|---|---|---|
| `000-fork-base` | `fork-info` command and the build stamp; runs from an init in `zz_fork_init.go`, no edit to `main.go` | `cmd/holyarrs/fork_check_000_test.go` |
| `001-sites-plugin-slot` | the four built-in site readers out; `sites.go` is an empty plug-in slot (`RegisterSite`); undo = revert one commit | `internal/sources/fork_check_001_test.go` + address scan of the program |
| `002-private-defaults` | metadata from your own keys, install count off, trailers off, update check reads this fork | `cmd/holyarrs/fork_check_002_test.go` |
| `003-windows-service` | `holyarrs.exe service -name N -config D` under the Service Control Manager, 127.0.0.1 by default | `cmd/holyarrs/fork_check_003_test.go` |

No sources, trackers, sites or scrapers are added by this recipe.

## Running it (Windows, PowerShell 5.1+, Git, Go, Node/npm)

```powershell
.\fork\apply.ps1 -Upstream 1.1.4 -Go <path to go.exe>     # full run
.\fork\apply.ps1 -Upstream 1.1.5 -PatchOnly               # does the recipe still apply?
.\fork\apply.ps1 -Upstream 1.1.4 -Skip 002                # pick patches: all but 002
```

It checks out the tag in a scratch worktree, builds **holyarrs-stock.exe**
from the untouched tag, applies the patches in order with a 3-way merge
(`git am -3`) and stops at the first one that fails, naming it, builds
**holyarrs-mod.exe**, runs the fork checks (the patches' "fingerprints":
a patch that applies but no longer does its job fails here) and then
upstream's Go tests. Output: `<BuildRoot>\recipe\<tag>\` with both programs,
`build-info.json`, `checks.txt`, `upstream-tests.txt`. Exit codes: 0 ok,
2 bad input, 3 a patch failed, 4 a build failed, 5 a check failed,
6 upstream tests failed.

Both programs carry the upstream version (`-X main.version=<tag>`); the mod
also carries `-X main.forkBuild=mod+r<recipe commit>` (see `holyarrs-mod.exe fork-info`).

## Picking patches (like Morphe, at build time)

`-Skip 002` (or `-Skip 001,002`) leaves patches out. Each patch message
carries a `Needs:` line (`none`, or the patches it builds on: 002 and 003
need 000); a choice that drops a needed patch is refused (exit 2). A run
with `-Skip` writes to `recipe\<tag>-no<NNN>\` and stamps the mod
`mod+r<commit>-no<NNN>`, so it never overwrites the full build. Live
on/off switches inside the running app come later (a Fork page in Settings).

## Rules that keep the recipe cheap

- Add new files rather than edit upstream ones; an edit to an upstream file
  is a one-line hook where possible, and each patch header lists its edits.
- Upstream's own structure checks are part of the tests: `main.go` is capped
  at 800 lines (it is at 800, so change lines in place, never add), and the
  JSON keys the server uses are a snapshot a patch must update.
- No patch touches database migrations. Stock and mod never share a config
  folder (stock must never open a database the mod wrote, and the reverse).
- One patch, one job, one check test named `fork_check_NNN_test.go`, and a
  `Needs:` line in its message.
- Private stays private: settings, keys (`holyarrs.key`), databases and
  backups never go into this repository.

## Refreshing a patch after an upstream release

Run `-PatchOnly` on the new tag. If patch NNN fails: check out the tag,
apply the patches before NNN with `git am -3`, apply NNN by hand, re-run its
check, then `git format-patch -1 --zero-commit` it back over
`fork/patches/NNN-*.patch`. Then run the full recipe.
