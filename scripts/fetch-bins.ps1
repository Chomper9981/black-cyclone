# Downloads the runtime binaries (sing-box, GoodbyeDPI) into <project>\bin.
# Usage: powershell -ExecutionPolicy Bypass -File scripts\fetch-bins.ps1
$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$root = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $root 'bin'
$tmp = Join-Path $env:TEMP ('rcv-fetch-' + [guid]::NewGuid())
New-Item -ItemType Directory -Force $bin, $tmp | Out-Null

function Get-LatestAsset($repo, $pattern) {
    $rel = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest" -Headers @{ 'User-Agent' = 'redcyclone-vpn' }
    $asset = $rel.assets | Where-Object { $_.name -match $pattern } | Select-Object -First 1
    if (-not $asset) { throw "No asset matching '$pattern' in $repo $($rel.tag_name)" }
    Write-Host "$repo $($rel.tag_name): $($asset.name)"
    return $asset
}

function Expand-Asset($asset) {
    $zip = Join-Path $tmp $asset.name
    Invoke-WebRequest $asset.browser_download_url -OutFile $zip -UseBasicParsing
    $dir = Join-Path $tmp ([IO.Path]::GetFileNameWithoutExtension($asset.name))
    Expand-Archive $zip -DestinationPath $dir -Force
    return $dir
}

try {
    # sing-box: the windows-amd64 build ships with an embedded wintun driver.
    $dir = Expand-Asset (Get-LatestAsset 'SagerNet/sing-box' '^sing-box-[\d.]+-windows-amd64\.zip$')
    Get-ChildItem $dir -Recurse -Filter 'sing-box.exe' | Copy-Item -Destination $bin -Force

    # GoodbyeDPI: x86_64 build + WinDivert driver.
    $dir = Expand-Asset (Get-LatestAsset 'ValdikSS/GoodbyeDPI' '^goodbyedpi-[\d.]+\.zip$')
    $x64 = Get-ChildItem $dir -Recurse -Directory -Filter 'x86_64' | Select-Object -First 1
    Copy-Item (Join-Path $x64.FullName '*') -Destination $bin -Force

    Get-ChildItem $bin | Select-Object Name, Length | Format-Table -AutoSize
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
