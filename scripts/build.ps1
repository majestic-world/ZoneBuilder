<#
.SYNOPSIS
Builds bin/zonebuilder.exe and copies ANGLE's libEGL.dll and libGLESv2.dll
next to it.

.DESCRIPTION
The app loads ANGLE at run time (no cgo, no C toolchain). The two DLLs are
third-party binaries (~8 MB, BSD) and are not committed: take them from any
64-bit ANGLE build that exposes GL_EXT_clip_control and the DXT extensions
(docs/adr/0001, 0002), e.g. a Chromium/CEF install such as
  C:\Program Files\JetBrains\<IDE>\plugins\jcef-plugin\jcef
  C:\Program Files (x86)\Steam\bin\cef\cef.win64
  C:\Program Files\NVIDIA Corporation\NVIDIA App\CEF
The 32-bit ANGLE shipped in the Lineage II client's system folder does not
work. d3dcompiler_47.dll comes from System32 (Windows 10+). The app logs the
ANGLE version and the extensions it found at startup and refuses to start
without the required ones.

.PARAMETER AngleDir
Folder holding the 64-bit libEGL.dll and libGLESv2.dll. Defaults to the
ZB_ANGLE_DIR environment variable.

.EXAMPLE
$env:ZB_ANGLE_DIR = 'C:\Program Files (x86)\Steam\bin\cef\cef.win64'
./scripts/build.ps1
./bin/zonebuilder.exe
#>
param(
    [string]$AngleDir = $env:ZB_ANGLE_DIR
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $root 'bin'

if (-not $AngleDir) {
    throw 'Set -AngleDir or ZB_ANGLE_DIR to a folder with the 64-bit libEGL.dll and libGLESv2.dll.'
}
$dlls = 'libEGL.dll', 'libGLESv2.dll'
foreach ($dll in $dlls) {
    if (-not (Test-Path (Join-Path $AngleDir $dll))) {
        throw "$dll not found in $AngleDir"
    }
}

New-Item -ItemType Directory -Force -Path $bin | Out-Null
$env:CGO_ENABLED = '0'
Push-Location $root
try {
    go build -o (Join-Path $bin 'zonebuilder.exe') ./cmd/zonebuilder
    if ($LASTEXITCODE -ne 0) { throw "go build failed ($LASTEXITCODE)" }
} finally {
    Pop-Location
}
foreach ($dll in $dlls) {
    Copy-Item -Force (Join-Path $AngleDir $dll) $bin
}
Write-Host "Built $bin\zonebuilder.exe with ANGLE from $AngleDir"
