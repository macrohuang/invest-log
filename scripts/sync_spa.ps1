$ErrorActionPreference = 'Stop'

$root = Resolve-Path (Join-Path $PSScriptRoot '..')
$src = Join-Path $root 'static'
$dest = Join-Path $root 'ios/App/App/public'

if (-not (Test-Path $src)) {
  Write-Error "Source static/ not found: $src"
}
if (-not (Test-Path $dest)) {
  Write-Error "Destination not found: $dest"
}

# Mirror contents (removes old files in dest).
Get-ChildItem -Path $dest -Force | Remove-Item -Recurse -Force
Copy-Item -Path (Join-Path $src '*') -Destination $dest -Recurse -Force

Write-Host "Synced $src -> $dest"
