<#
.SYNOPSIS
  HolyArrs fork recipe: builds stock and modded HolyArrs from one upstream tag.

.DESCRIPTION
  1. Clean checkout of the upstream tag in a scratch worktree.
  2. Builds the stock program first and keeps it as holyarrs-stock.exe.
  3. Applies fork\patches\NNN-*.patch in order with a 3-way merge (git am -3);
     stops at the first patch that fails and names it.
  4. Builds holyarrs-mod.exe (web page with npm, program with go).
  5. Runs the fork's checks (fork_check_*_test.go, the patches' "fingerprints")
     and program-level checks (fork-info, no built-in site addresses).
  6. Runs upstream's Go tests (skip with -SkipUpstreamTests).
  Both programs carry a version stamp (-X main.version=<tag>); the mod also
  carries -X main.forkBuild=mod+r<recipe commit>.
  Everything lands in <BuildRoot>\recipe\<tag>\ with build-info.json.
  Nothing here touches settings, keys, databases or services.

.EXAMPLE
  .\fork\apply.ps1 -Upstream 1.1.4 -Go D:\tools\go\bin\go.exe
.EXAMPLE
  .\fork\apply.ps1 -Upstream 1.1.5 -PatchOnly     # does the recipe still apply?

  Exit codes: 0 ok, 2 bad input, 3 a patch failed, 4 a build failed,
  5 a fork check failed, 6 upstream tests failed.
#>
param(
  [Parameter(Mandatory = $true)][string]$Upstream,
  [string]$Repo = '',
  [string]$BuildRoot = '',
  [string]$Go = 'go',
  [string]$Npm = 'npm.cmd',
  [string]$Git = 'git',
  [int]$TestParallel = 2,
  [switch]$PatchOnly,
  [switch]$SkipUpstreamTests
)
$ErrorActionPreference = 'Stop'
# Windows PowerShell 5.1 has no $PSScriptRoot in param defaults: fill them here.
if (-not $Repo) { $Repo = Split-Path -Parent $PSScriptRoot }
if (-not $BuildRoot) { $BuildRoot = Join-Path (Split-Path -Parent $Repo) 'HolyArrs-build' }
$RecipeStopWatch = [Diagnostics.Stopwatch]::StartNew()
function Write-Step([string]$StepText) { "[{0,5:N1} min] {1}" -f $RecipeStopWatch.Elapsed.TotalMinutes, $StepText }
function Invoke-Native([string]$Exe, [string[]]$ArgList, [string]$LogFile) {
  $PreviousPreference = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
  if ($LogFile) { & $Exe @ArgList *>&1 | ForEach-Object { "$_" } | Add-Content -Path $LogFile -Encoding UTF8 }
  else { & $Exe @ArgList *>&1 | ForEach-Object { "  $_" } }
  $NativeCode = $LASTEXITCODE; $ErrorActionPreference = $PreviousPreference
  return $NativeCode
}
function Stop-Recipe([int]$Code, [string]$Why) { Write-Step "STOP ($Code): $Why"; exit $Code }

$PatchFolder = Join-Path $PSScriptRoot 'patches'
$Patches = @(Get-ChildItem -Path $PatchFolder -Filter '*.patch' | Sort-Object Name)
if ($Patches.Count -eq 0) { Stop-Recipe 2 "no patches in $PatchFolder" }
$TagCommit = (& $Git -C $Repo rev-parse --verify --quiet "refs/tags/$Upstream^{commit}")
if (-not $TagCommit) { Stop-Recipe 2 "tag $Upstream not found (git fetch origin --tags)" }
$RecipeCommit = (& $Git -C $Repo rev-parse --short=12 HEAD)
$RecipeDirty = @(& $Git -C $Repo status --porcelain -- fork).Count -gt 0
$OutFolder = Join-Path $BuildRoot "recipe\$Upstream"
$SourceTree = Join-Path $OutFolder 'src'
New-Item -ItemType Directory -Force -Path $OutFolder, "$BuildRoot\gocache", "$BuildRoot\gomod" | Out-Null
$env:GOCACHE = "$BuildRoot\gocache"; $env:GOMODCACHE = "$BuildRoot\gomod"; $env:CGO_ENABLED = '0'
Write-Step "upstream $Upstream = $($TagCommit.Substring(0,12)); recipe $RecipeCommit$(if ($RecipeDirty) {' (fork folder has uncommitted changes)'}); $($Patches.Count) patches"

# 1. Clean checkout of the tag.
if (Test-Path $SourceTree) { [void](Invoke-Native $Git @('-C', $Repo, 'worktree', 'remove', '--force', $SourceTree)) }
[void](Invoke-Native $Git @('-C', $Repo, 'worktree', 'prune'))
if ((Invoke-Native $Git @('-C', $Repo, 'worktree', 'add', '--force', '--detach', $SourceTree, $TagCommit) "$OutFolder\git.txt") -ne 0) { Stop-Recipe 2 "checkout failed (git.txt)" }

function Build-HolyArrs([string]$Flavour, [string]$ExeName, [string]$LdFlags) {
  Write-Step "build $Flavour"
  $LogFile = "$OutFolder\build-$Flavour.txt"; Set-Content -Path $LogFile -Value "build $Flavour" -Encoding UTF8
  $Front = Join-Path $SourceTree 'app\frontend'
  if (-not (Test-Path "$Front\node_modules")) {
    if ((Invoke-Native $Npm @('--prefix', $Front, 'ci', '--no-audit', '--no-fund', '--loglevel=error') $LogFile) -ne 0) { Stop-Recipe 4 "npm ci failed ($Flavour, $LogFile)" }
  }
  if ((Invoke-Native $Npm @('--prefix', $Front, 'run', 'build') $LogFile) -ne 0) { Stop-Recipe 4 "web page build failed ($Flavour, $LogFile)" }
  Push-Location $SourceTree
  try { $GoCode = Invoke-Native $Go @('build', '-trimpath', "-ldflags=$LdFlags", '-o', "$OutFolder\$ExeName", './app/backend/cmd/holyarrs') $LogFile }
  finally { Pop-Location }
  if ($GoCode -ne 0) { Stop-Recipe 4 "go build failed ($Flavour, $LogFile)" }
  Write-Step "  $ExeName $([math]::Round((Get-Item "$OutFolder\$ExeName").Length/1MB,1)) MB"
}

# 2. Stock first, from the untouched tag.
if (-not $PatchOnly) { Build-HolyArrs 'stock' 'holyarrs-stock.exe' "-s -w -X main.version=$Upstream" }

# 3. The recipe, one patch at a time, 3-way merge.
$Applied = @()
foreach ($PatchFile in $Patches) {
  $AmArgs = @('-C', $SourceTree, '-c', 'user.name=HolyArrs fork recipe', '-c', 'user.email=recipe@invalid', '-c', 'core.autocrlf=false', 'am', '-3', '--keep-cr', '--committer-date-is-author-date', $PatchFile.FullName)
  if ((Invoke-Native $Git $AmArgs "$OutFolder\git.txt") -ne 0) {
    [void](Invoke-Native $Git @('-C', $SourceTree, 'am', '--abort') "$OutFolder\git.txt")
    Stop-Recipe 3 "patch $($PatchFile.Name) does not apply to $Upstream (applied before it: $($Applied -join ', '); details in git.txt)"
  }
  $Applied += $PatchFile.BaseName
  Write-Step "  applied $($PatchFile.Name)"
}
if ($PatchOnly) { Write-Step "all $($Patches.Count) patches apply to $Upstream (PatchOnly: nothing built)"; exit 0 }

# 4. The mod.
Build-HolyArrs 'mod' 'holyarrs-mod.exe' "-s -w -X main.version=$Upstream -X main.forkBuild=mod+r$RecipeCommit"

# 5. Our checks: the patches' fingerprint tests, then the programs themselves.
Write-Step "fork checks"
$CheckLog = "$OutFolder\checks.txt"; Set-Content -Path $CheckLog -Value "fork checks" -Encoding UTF8
$CheckPackages = @(Get-ChildItem -Path "$SourceTree\app" -Recurse -Filter 'fork_check_*_test.go' | ForEach-Object { './' + ($_.DirectoryName.Substring($SourceTree.Length + 1) -replace '\\', '/') } | Sort-Object -Unique)
Push-Location $SourceTree
try { $CheckCode = Invoke-Native $Go (@('test', '-count=1', '-run', '^TestForkCheck', '-v') + $CheckPackages) $CheckLog }
finally { Pop-Location }
$CheckLines = Get-Content $CheckLog
$CheckPassed = @($CheckLines | Where-Object { $_ -match '^--- PASS: TestForkCheck' }).Count
$CheckFailed = @($CheckLines | Where-Object { $_ -match '^--- FAIL' }).Count
$ProgramProblems = @()
$PreviousPreference = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
$ForkInfoText = (& "$OutFolder\holyarrs-mod.exe" fork-info 2>&1 | Out-String).Trim()
$ErrorActionPreference = $PreviousPreference
Add-Content -Path $CheckLog -Value "fork-info: $ForkInfoText"
try { $ForkInfo = $ForkInfoText | ConvertFrom-Json } catch { $ForkInfo = $null; $ProgramProblems += 'fork-info did not answer JSON' }
if ($ForkInfo) {
  if ($ForkInfo.version -ne $Upstream) { $ProgramProblems += "mod version is $($ForkInfo.version)" }
  foreach ($AppliedName in $Applied) { if (@($ForkInfo.patches) -notcontains $AppliedName -and $AppliedName -match '^(000|002|003)-') { $ProgramProblems += "patch $AppliedName not in the program" } }
  if (@($ForkInfo.site_readers).Count -gt 0) { $ProgramProblems += "site readers built in: $($ForkInfo.site_readers -join ', ')" }
}
$SiteHosts = @('api.knaben.org', 'torrents-csv.com', 'subsplease.org', 'iptorrents.com')
function Find-Hosts([string]$ExePath) {
  $ExeText = [Text.Encoding]::ASCII.GetString([IO.File]::ReadAllBytes($ExePath))
  return @($SiteHosts | Where-Object { $ExeText.Contains($_) })
}
$StockHosts = Find-Hosts "$OutFolder\holyarrs-stock.exe"
$ModHosts = Find-Hosts "$OutFolder\holyarrs-mod.exe"
Add-Content -Path $CheckLog -Value "site addresses in stock: $($StockHosts -join ', ')"
Add-Content -Path $CheckLog -Value "site addresses in mod: $($ModHosts -join ', ')"
if ($StockHosts.Count -eq 0) { $ProgramProblems += 'the address scan found nothing in stock (scan broken?)' }
if ($ModHosts.Count -gt 0) { $ProgramProblems += "mod still holds: $($ModHosts -join ', ')" }
$ProgramProblems | ForEach-Object { Add-Content -Path $CheckLog -Value "PROBLEM: $_" }
Write-Step "  fork checks: $CheckPassed passed, $CheckFailed failed (go test exit $CheckCode); program checks: $(if ($ProgramProblems.Count) { $ProgramProblems -join '; ' } else { 'ok' })"

function Write-BuildInfo([string]$TestSummary) {
  $Info = [ordered]@{
    upstream = $Upstream; upstream_commit = $TagCommit; recipe_commit = $RecipeCommit; recipe_dirty = $RecipeDirty
    patches = @($Patches | ForEach-Object { [ordered]@{ name = $_.Name; sha256 = (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLower() } })
    stock = [ordered]@{ exe = 'holyarrs-stock.exe'; sha256 = (Get-FileHash "$OutFolder\holyarrs-stock.exe").Hash.ToLower(); version = $Upstream }
    mod = [ordered]@{ exe = 'holyarrs-mod.exe'; sha256 = (Get-FileHash "$OutFolder\holyarrs-mod.exe").Hash.ToLower(); version = $Upstream; build = "mod+r$RecipeCommit" }
    fork_checks = [ordered]@{ passed = $CheckPassed; failed = $CheckFailed; program_problems = $ProgramProblems }
    upstream_tests = $TestSummary
    built_at = (Get-Date).ToString('s'); minutes = [math]::Round($RecipeStopWatch.Elapsed.TotalMinutes, 1)
  }
  $Info | ConvertTo-Json -Depth 5 | Set-Content -Path "$OutFolder\build-info.json" -Encoding UTF8
}
if ($CheckCode -ne 0 -or $CheckFailed -gt 0 -or $CheckPassed -eq 0 -or $ProgramProblems.Count -gt 0) { Write-BuildInfo 'not run'; Stop-Recipe 5 "fork checks failed ($CheckLog)" }

# 6. Upstream's tests on the patched tree.
if ($SkipUpstreamTests) { Write-BuildInfo 'skipped'; Write-Step "done (upstream tests skipped): $OutFolder"; exit 0 }
Write-Step "upstream tests (-p $TestParallel)"
$TestLog = "$OutFolder\upstream-tests.txt"; Set-Content -Path $TestLog -Value "upstream tests" -Encoding UTF8
Push-Location $SourceTree
try { $TestCode = Invoke-Native $Go @('test', '-p', "$TestParallel", '-timeout', '25m', './...') $TestLog }
finally { Pop-Location }
$TestLines = Get-Content $TestLog
$TestOk = @($TestLines | Where-Object { $_ -match '^ok\s' }).Count
$TestFail = @($TestLines | Where-Object { $_ -match '^FAIL\s' }).Count
$TestSummary = "$TestOk packages ok, $TestFail failed (exit $TestCode)"
Write-BuildInfo $TestSummary
Write-Step "  $TestSummary"
$TestLines | Where-Object { $_ -match '^\s*--- FAIL|^FAIL\s|panic:' } | Select-Object -First 20 | ForEach-Object { "  $_" }
if ($TestCode -ne 0) { Stop-Recipe 6 "upstream tests failed ($TestLog)" }
Write-Step "done: $OutFolder"
exit 0
