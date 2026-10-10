<#
.SYNOPSIS
Builds "bin/Zone Builder.exe" and copies ANGLE's libEGL.dll and libGLESv2.dll
next to it.

.DESCRIPTION
The app loads ANGLE at run time (no cgo, no C toolchain). The two DLLs live
in third_party/angle (64-bit, BSD; see its README for the version and the
extensions the app needs). The app logs the ANGLE version and the
extensions it found at startup and refuses to start without the required
ones.

The version in the window title is APP_VERSION from .env at the repository
root, linked into main.version.

The executable is linked with -H windowsgui, so Windows opens no console
window with it; started from a terminal, the app attaches to that terminal
and logs there (cmd/zonebuilder/console_windows.go).

.EXAMPLE
./scripts/build.ps1
& './bin/Zone Builder.exe'
#>
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $root 'bin'
$AngleDir = Join-Path $root 'third_party\angle'

$dlls = 'libEGL.dll', 'libGLESv2.dll'
foreach ($dll in $dlls) {
    if (-not (Test-Path (Join-Path $AngleDir $dll))) {
        throw "$dll not found in $AngleDir"
    }
}

. (Join-Path $PSScriptRoot 'appversion.ps1')
$version = Get-AppVersion $root

New-Item -ItemType Directory -Force -Path $bin | Out-Null
$env:CGO_ENABLED = '0'
Push-Location $root
try {
    go build -ldflags "-H windowsgui -X main.version=$version" -o (Join-Path $bin 'Zone Builder.exe') ./cmd/zonebuilder
    if ($LASTEXITCODE -ne 0) { throw "go build failed ($LASTEXITCODE)" }
} finally {
    Pop-Location
}
# Skip DLLs already in place: a running Zone Builder.exe keeps them locked,
# and an identical copy is all a rebuild needs.
foreach ($dll in $dlls) {
    $src = Join-Path $AngleDir $dll
    $dst = Join-Path $bin $dll
    if ((Test-Path $dst) -and (Get-FileHash $src).Hash -eq (Get-FileHash $dst).Hash) {
        continue
    }
    Copy-Item -Force $src $bin
}
Write-Host "Built $bin\Zone Builder.exe (v$version)"
