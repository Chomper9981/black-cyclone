# Downloads the runtime binary (sing-box) into <project>\bin.
# Versions are pinned and verified by SHA-256: sing-box changes its config format
# between releases, so bump these together with internal/singbox, its tests
# and linux/fetch-bins.sh.
# Usage: powershell -ExecutionPolicy Bypass -File windows\fetch-bins.ps1
$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$assets = @(
    @{
        Name   = 'sing-box'
        Url    = 'https://github.com/SagerNet/sing-box/releases/download/v1.14.3/sing-box-1.14.3-windows-amd64.zip'
        Sha256 = 'd8b86ef58936dba9230ec6a57ffa4243933f3c14ef4809828b6c553440ed1518'
    }
)
# Vietnamese IP ranges for "Web Việt Nam đi thẳng"; refreshed weekly upstream, so not pinned.
# Optional: without it sing-box downloads the same file at connect time.
$geoipUrl = 'https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-vn.srs'

$root = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $root 'bin'
$tmp = Join-Path $env:TEMP ('rcv-fetch-' + [guid]::NewGuid())
New-Item -ItemType Directory -Force $bin, $tmp | Out-Null

function Get-Verified($a) {
    $zip = Join-Path $tmp ([IO.Path]::GetFileName($a.Url))
    Write-Host "Downloading $($a.Name): $($a.Url)"
    Invoke-WebRequest $a.Url -OutFile $zip -UseBasicParsing
    $hash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
    if ($hash -ne $a.Sha256) { throw "$($a.Name): SHA-256 mismatch (got $hash, want $($a.Sha256))" }
    $dir = Join-Path $tmp ([IO.Path]::GetFileNameWithoutExtension($zip))
    Expand-Archive $zip -DestinationPath $dir -Force
    return $dir
}

try {
    # sing-box: the windows-amd64 build ships with an embedded wintun driver.
    $dir = Get-Verified $assets[0]
    Get-ChildItem $dir -Recurse -Filter 'sing-box.exe' | Copy-Item -Destination $bin -Force

    try {
        Invoke-WebRequest $geoipUrl -OutFile (Join-Path $bin 'geoip-vn.srs') -UseBasicParsing
    } catch {
        Write-Warning "geoip-vn.srs download failed (optional): $_"
    }

    Get-ChildItem $bin | Select-Object Name, Length | Format-Table -AutoSize
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
