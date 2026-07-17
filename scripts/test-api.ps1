<#
.SYNOPSIS
    Run the Go API test suite inside Docker (Linux toolchain) instead of natively.

.DESCRIPTION
    On this Windows host, Application Control (AppLocker) intermittently blocks
    natively-compiled `go test` binaries from C:\tmp\go-build... with
    "An Application Control policy has blocked this file." Running the tests
    inside a Linux container sidesteps the policy entirely — the test binaries
    execute in the container, not on the Windows host.

    Unit tests are the default. Pass -Integration to also run the
    `//go:build integration` tests (testcontainers-go); that mode mounts the
    Docker socket so the tests can start sibling Postgres containers.

    The Go image is read from api/Dockerfile's GO_IMAGE ARG, so it always
    matches the build image (currently a digest-pinned golang:1.26-alpine).
    Build/module caches persist in named volumes for fast reruns.

.EXAMPLE
    ./scripts/test-api.ps1
    ./scripts/test-api.ps1 ./internal/digest/
    ./scripts/test-api.ps1 -- -run TestBuildTodayDigest ./internal/digest/
    ./scripts/test-api.ps1 -Integration
#>
[CmdletBinding()]
param(
    [switch]$Integration,
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$GoTestArgs
)

$ErrorActionPreference = 'Stop'

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot '..')
$apiPath = Join-Path $repoRoot 'api'

# Read the digest-pinned Go image from the Dockerfile so this never drifts.
$dockerfile = Join-Path $apiPath 'Dockerfile'
$goImageLine = Select-String -Path $dockerfile -Pattern '^\s*ARG\s+GO_IMAGE=(.+)$' | Select-Object -First 1
if (-not $goImageLine) { throw "Could not find 'ARG GO_IMAGE=' in $dockerfile" }
$goImage = $goImageLine.Matches[0].Groups[1].Value.Trim()

$dockerArgs = @(
    'run', '--rm',
    '-v', "${apiPath}:/app",
    '-v', 'vigil-go-build-cache:/root/.cache/go-build',
    '-v', 'vigil-go-mod-cache:/go/pkg/mod',
    '-w', '/app'
)

if ($Integration) {
    # testcontainers-go needs the Docker socket to start sibling containers.
    $dockerArgs += @('-v', '/var/run/docker.sock:/var/run/docker.sock')
}

$goCmd = @('go', 'test')
if ($Integration) {
    $goCmd += @('-tags=integration')
    if (-not $GoTestArgs) { $GoTestArgs = @('./internal/database/') }
}
if (-not $GoTestArgs) { $GoTestArgs = @('./...') }

$full = $dockerArgs + $goImage + $goCmd + $GoTestArgs
Write-Host "→ docker $($full -join ' ')" -ForegroundColor Cyan
& docker @full
exit $LASTEXITCODE
