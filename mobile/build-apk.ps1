# Builds the Go core (gomobile -> android/app/libs/rcvcore.aar) and the release APK.
# Usage: powershell -ExecutionPolicy Bypass -File build-apk.ps1 [-SkipCore] [-Debug]
param([switch]$SkipCore, [switch]$Debug)
$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot

if (-not $env:ANDROID_HOME) { $env:ANDROID_HOME = 'D:\Android\Sdk' }
if (-not $env:ANDROID_NDK_HOME) {
    $ndk = Get-ChildItem "$env:ANDROID_HOME\ndk" -Directory | Sort-Object Name -Descending | Select-Object -First 1
    if (-not $ndk) { throw "Không tìm thấy NDK trong $env:ANDROID_HOME\ndk" }
    $env:ANDROID_NDK_HOME = $ndk.FullName
}
$env:PATH = "$env:USERPROFILE\go\bin;$env:PATH"

if (-not $SkipCore) {
    if (-not (Get-Command gomobile -ErrorAction SilentlyContinue)) {
        go install golang.org/x/mobile/cmd/gomobile@latest
        go install golang.org/x/mobile/cmd/gobind@latest
    }
    Push-Location "$root\core"
    try {
        go test ./...
        if ($LASTEXITCODE) { throw 'go test thất bại' }
        gomobile bind -target android/arm64,android/arm,android/amd64 -androidapi 26 `
            -ldflags '-s -w' -trimpath -o ..\android\app\libs\rcvcore.aar ./rcvcore
        if ($LASTEXITCODE) { throw 'gomobile bind thất bại' }
    } finally { Pop-Location }
}

Push-Location "$root\android"
try {
    $task = if ($Debug) { 'assembleDebug' } else { 'assembleRelease' }
    .\gradlew.bat $task --no-daemon
    if ($LASTEXITCODE) { throw "gradle $task thất bại" }
} finally { Pop-Location }

$kind = if ($Debug) { 'debug' } else { 'release' }
Get-ChildItem "$root\android\app\build\outputs\apk\$kind\*.apk" | ForEach-Object { Write-Host "APK: $($_.FullName)" }

if (-not $Debug) {
    $ver = (Select-String -Path "$root\android\app\build.gradle.kts" -Pattern 'versionName = "(.+)"').Matches[0].Groups[1].Value
    New-Item -ItemType Directory -Force "$root\dist" | Out-Null
    $out = "$root\dist\BlackCyclone-$ver.apk"
    Copy-Item "$root\android\app\build\outputs\apk\release\app-release.apk" $out -Force
    Write-Host "Đã chép: $out"
}
