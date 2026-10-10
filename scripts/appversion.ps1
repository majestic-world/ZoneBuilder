<#
.SYNOPSIS
Defines Get-AppVersion, which reads APP_VERSION from .env at the repository
root. Dot-source it: . (Join-Path $PSScriptRoot 'appversion.ps1')
#>
function Get-AppVersion([string]$root) {
    $envFile = Join-Path $root '.env'
    if (-not (Test-Path $envFile)) {
        throw ".env not found in $root (it must set APP_VERSION)"
    }
    $version = $null
    foreach ($line in Get-Content $envFile) {
        if ($line -match '^\s*APP_VERSION\s*=\s*"?([^"#\s]+)"?\s*(#.*)?$') {
            $version = $Matches[1]
        }
    }
    if (-not $version) {
        throw "APP_VERSION not set in $envFile"
    }
    $version
}
