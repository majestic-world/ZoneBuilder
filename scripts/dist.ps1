<#
.SYNOPSIS
Zips the contents of bin/ into "dist/Zone Builder By Mk v<version>.zip".

.DESCRIPTION
The version is APP_VERSION from .env at the repository root, the same one
build.ps1 links into the executable. The files sit at the root of the zip.
Every zip already in dist/ is deleted first, so only the current one
remains. Run build.ps1 first (make dist does).

.EXAMPLE
./scripts/dist.ps1
#>
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $root 'bin'
$dist = Join-Path $root 'dist'

. (Join-Path $PSScriptRoot 'appversion.ps1')
$version = Get-AppVersion $root

if (-not (Test-Path (Join-Path $bin 'Zone Builder.exe'))) {
    throw "Zone Builder.exe not found in $bin (run scripts/build.ps1)"
}
New-Item -ItemType Directory -Force -Path $dist | Out-Null
Get-ChildItem -Path $dist -Filter '*.zip' -File | Remove-Item -Force
$zip = Join-Path $dist "Zone Builder By Mk v$version.zip"
Compress-Archive -Path (Join-Path $bin '*') -DestinationPath $zip -Force
Write-Host "Packed $zip"
