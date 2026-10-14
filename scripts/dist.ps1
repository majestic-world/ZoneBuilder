<#
.SYNOPSIS
Zips the app files of bin/ into "dist/Zone Builder By Mk v<version>.zip".

.DESCRIPTION
The version is APP_VERSION from .env at the repository root, the same one
build.ps1 links into the executable. Only the 3 files the app needs are
packed, at the root of the zip: Zone Builder.exe and ANGLE's libEGL.dll
and libGLESv2.dll; anything else dropped in bin/ stays out. Every zip
already in dist/ is deleted first, so only the current one remains. Run
build.ps1 first (make dist does).

.EXAMPLE
./scripts/dist.ps1
#>
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $root 'bin'
$dist = Join-Path $root 'dist'

. (Join-Path $PSScriptRoot 'appversion.ps1')
$version = Get-AppVersion $root

$files = 'Zone Builder.exe', 'libEGL.dll', 'libGLESv2.dll' | ForEach-Object {
    $f = Join-Path $bin $_
    if (-not (Test-Path $f)) {
        throw "$_ not found in $bin (run scripts/build.ps1)"
    }
    $f
}
New-Item -ItemType Directory -Force -Path $dist | Out-Null
Get-ChildItem -Path $dist -Filter '*.zip' -File | Remove-Item -Force
$zip = Join-Path $dist "Zone Builder By Mk v$version.zip"
Compress-Archive -Path $files -DestinationPath $zip -Force
Write-Host "Packed $zip"
