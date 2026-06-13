param(
  [string]$Dist = "dist"
)

$ErrorActionPreference = "Stop"

if (-not (Get-Command scoop -ErrorAction SilentlyContinue)) {
  Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force
  Invoke-RestMethod -Uri https://get.scoop.sh | Invoke-Expression
}

$manifestPath = Join-Path $Dist "scoop/bucket/inferctl.json"
if (-not (Test-Path $manifestPath)) {
  throw "missing Scoop manifest: $manifestPath"
}

$zip = Get-ChildItem -Path $Dist -Filter "*windows_amd64*.zip" | Select-Object -First 1
if ($null -eq $zip) {
  throw "missing windows amd64 zip artifact under $Dist"
}

$hash = (Get-FileHash -Path $zip.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
$uri = ([System.Uri]$zip.FullName).AbsoluteUri
$manifest = Get-Content -Path $manifestPath -Raw | ConvertFrom-Json

if ($manifest.architecture -and $manifest.architecture.'64bit') {
  $manifest.architecture.'64bit'.url = $uri
  $manifest.architecture.'64bit'.hash = $hash
} else {
  $manifest.url = $uri
  $manifest.hash = $hash
}

$localManifest = Join-Path ([System.IO.Path]::GetTempPath()) "inferctl-local-scoop.json"
$manifest | ConvertTo-Json -Depth 20 | Set-Content -Path $localManifest -Encoding UTF8

scoop uninstall inferctl 2>$null | Out-Null
scoop install $localManifest

$version = infer version --json | ConvertFrom-Json
if (-not $version.ok) {
  throw "infer version returned ok=false"
}
if ($version.data.binary -ne "infer") {
  throw "unexpected binary name: $($version.data.binary)"
}

scoop uninstall inferctl
Write-Host "scoop smoke ok"
