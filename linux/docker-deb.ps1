# Builds the Linux .deb packages from Windows, in an Ubuntu 22.04 container per
# architecture (arm64 runs under Docker Desktop's QEMU emulation, which is slow).
# Build the frontend first (wails build, or npm run build in frontend\).
# Usage: powershell -ExecutionPolicy Bypass -File linux\docker-deb.ps1 [-Arch amd64,arm64]
param([string[]]$Arch = @('amd64', 'arm64'))
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$out = Join-Path $root 'dist'
New-Item -ItemType Directory -Force $out | Out-Null
if (-not (Test-Path "$root\frontend\dist\index.html")) { throw 'frontend\dist chưa build: chạy wails build trước' }

foreach ($a in $Arch) {
    $image = "blackcyclone-deb:$a"
    docker build --platform "linux/$a" -t $image "$PSScriptRoot\docker"
    if ($LASTEXITCODE) { throw "docker build ($a) thất bại" }
    # Work on a copy of the source so the Windows bin\ and build\ stay untouched.
    $script = 'set -e; mkdir /src; cd /repo; tar --exclude=./bin --exclude=./build/bin --exclude=./dist --exclude=./.git --exclude=./frontend/node_modules --exclude=./mobile -cf - . | tar -xf - -C /src; cd /src; sh linux/package-deb.sh; cp dist/*.deb /out/'
    docker run --rm --platform "linux/$a" `
        -v "${root}:/repo:ro" -v "${out}:/out" -v "blackcyclone-gomod-${a}:/root/go/pkg/mod" `
        $image sh -c $script
    if ($LASTEXITCODE) { throw "đóng gói .deb ($a) thất bại" }
}
Get-ChildItem "$out\*.deb" | ForEach-Object { Write-Host "DEB: $($_.FullName)" }
