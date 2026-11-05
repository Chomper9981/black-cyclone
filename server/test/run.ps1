# Kiểm tra server/ trên một "VPS giả" chạy trong Docker: Ubuntu 24.04 khởi động
# systemd, có SSH, không có sẵn curl, bật ufw. Deploy bằng server\deploy.ps1 y như
# với VPS thật (user thường + sudo, khoá SSH), rồi kết nối thử bằng link sinh ra:
# link được đọc bằng bộ đọc link của app, đi qua sing-box trong bin\.
#
# Cần: Docker Desktop (engine Linux), Go, OpenSSH client, curl.exe,
#      bin\sing-box.exe (chạy windows\fetch-bins.ps1).
# Dùng: powershell -ExecutionPolicy Bypass -File server\test\run.ps1 [-Keep]
#       -Keep: giữ container sau khi test (vào xem: docker exec -it bc-vps-test bash)
param([switch]$Keep)

# Không dùng 'Stop': PowerShell 5.1 coi stderr của docker/ssh là lỗi. Mã thoát được kiểm tra riêng.
$ErrorActionPreference = 'Continue'

$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$image = 'blackcyclone-vps-test'
$name = 'bc-vps-test'
$sshPort = 2222
$vpnPort = 18443
$socksPort = 18080
$singbox = Join-Path $repo 'bin\sing-box.exe'
$tmp = Join-Path $env:TEMP ('bc-vps-test-' + [guid]::NewGuid())
New-Item -ItemType Directory $tmp | Out-Null
$key = Join-Path $tmp 'id_ed25519'
$results = New-Object System.Collections.Generic.List[object]

function Step([string]$msg) { Write-Host "`n==> $msg" -ForegroundColor Cyan }

function Check([string]$test, [bool]$ok, [string]$detail = '') {
    $results.Add([pscustomobject]@{ Test = $test; Result = $(if ($ok) { 'PASS' } else { 'FAIL' }); Detail = $detail })
    $color = if ($ok) { 'Green' } else { 'Red' }
    Write-Host ("   [{0}] {1} {2}" -f $(if ($ok) { 'PASS' } else { 'FAIL' }), $test, $detail) -ForegroundColor $color
}

function Must([string]$what) {
    if ($LASTEXITCODE -ne 0) { throw "$what thất bại (mã $LASTEXITCODE)" }
}

# Vps runs a shell command inside the container as root and returns its output.
# Keep commands free of double quotes (PowerShell 5.1 mangles them for native exes).
function Vps([string]$cmd) { docker exec $name bash -c $cmd 2>&1 | ForEach-Object { "$_" } }

function Get-Link([string]$user) {
    Vps "bcserver link $user" | Where-Object { $_ -like 'vless://*' } | Select-Object -First 1
}

# Test-Link connects through the server with a client built from the link and
# returns the HTTP status of a request made through it ('204' = works).
function Test-Link([string]$link) {
    if (-not $link) { return 'no-link' }
    $cfg = Join-Path $tmp 'client.json'
    & (Join-Path $tmp 'linkclient.exe') $link $socksPort $cfg
    if ($LASTEXITCODE -ne 0) { return 'parse-error' }
    $p = Start-Process $singbox -ArgumentList @('run', '-c', "`"$cfg`"") -PassThru -NoNewWindow `
        -RedirectStandardError (Join-Path $tmp 'client.err') -RedirectStandardOutput (Join-Path $tmp 'client.out')
    try {
        Start-Sleep -Seconds 2
        $code = & curl.exe -s -m 10 -o NUL -w '%{http_code}' -x "socks5h://127.0.0.1:$socksPort" https://www.gstatic.com/generate_204
        return "$code"
    }
    finally {
        Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
        $p.WaitForExit(5000) | Out-Null
    }
}

function Wait-Vps {
    for ($i = 0; $i -lt 60; $i++) {
        $state = (Vps 'systemctl is-system-running 2>/dev/null; true' | Select-Object -First 1)
        if ($state -eq 'running' -or $state -eq 'degraded') { return $true }
        Start-Sleep -Seconds 1
    }
    return $false
}

function Deploy {
    & (Join-Path $repo 'server\deploy.ps1') -Server 127.0.0.1 -SshPort $sshPort -User ubuntu -Identity $key `
        -Port $vpnPort -Address 127.0.0.1 -Name tester `
        -SshOption @('StrictHostKeyChecking=no', "UserKnownHostsFile=$tmp\known_hosts", 'LogLevel=ERROR')
}

$exitCode = 0
try {
    Step 'Kiểm tra công cụ'
    foreach ($tool in 'docker', 'go', 'ssh', 'scp', 'ssh-keygen', 'curl.exe') {
        if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) { throw "thiếu $tool" }
    }
    if (-not (Test-Path $singbox)) { throw "thiếu ${singbox}: chạy windows\fetch-bins.ps1" }
    Push-Location $repo
    & go build -o (Join-Path $tmp 'linkclient.exe') ./server/test/linkclient
    $built = $LASTEXITCODE
    Pop-Location
    if ($built -ne 0) { throw 'build linkclient thất bại' }

    Step 'Build image VPS giả'
    docker build -q -t $image $PSScriptRoot
    Must 'docker build'

    Step 'Khởi động VPS giả'
    docker rm -f $name 2>&1 | Out-Null
    docker run -d --name $name --hostname fake-vps --privileged --cgroupns=host `
        -v /sys/fs/cgroup:/sys/fs/cgroup:rw --tmpfs /run --tmpfs /run/lock `
        -p "127.0.0.1:${sshPort}:22" -p "127.0.0.1:${vpnPort}:${vpnPort}" $image | Out-Null
    Must 'docker run'
    if (-not (Wait-Vps)) { throw 'systemd trong container không khởi động được' }

    Step 'Cấp khoá SSH, bật ufw (chỉ mở SSH)'
    & ssh-keygen -q -t ed25519 -N '""' -C bc-vps-test -f $key
    Must 'ssh-keygen'
    docker cp "$key.pub" "${name}:/home/ubuntu/.ssh/authorized_keys"
    Must 'docker cp'
    Vps 'chown ubuntu:ubuntu /home/ubuntu/.ssh/authorized_keys && chmod 600 /home/ubuntu/.ssh/authorized_keys' | Out-Null
    $ufw = Vps 'ufw allow 22/tcp >/dev/null && ufw --force enable >/dev/null && ufw status | head -1'
    $ufwOn = ($ufw -join ' ') -match 'Status: active'
    if (-not $ufwOn) { Write-Host "   ufw không bật được trong container, bỏ qua phần kiểm tra tường lửa: $ufw" -ForegroundColor Yellow }
    Check 'VPS giả chưa có curl' (-not ((Vps 'command -v curl') -join '')) ''

    Step 'Deploy lần 1 (deploy.ps1, user ubuntu + sudo)'
    Deploy
    Check 'deploy.ps1 thành công' ($LASTEXITCODE -eq 0) "mã $LASTEXITCODE"
    $link1 = Get-Link 'tester'
    Check 'Có link cho tester' ([bool]$link1) $link1

    Step 'Kiểm tra trạng thái trên VPS'
    Check 'Service đang chạy' ((Vps 'systemctl is-active blackcyclone') -contains 'active')
    $owner = (Vps 'stat -c %U /proc/$(pidof sing-box)') -join ''
    Check 'Service chạy bằng user blackcyclone (không phải root)' ($owner -eq 'blackcyclone') $owner
    $perms = (Vps 'stat -c %n=%a:%U:%G /etc/blackcyclone/config.json /etc/blackcyclone/server.conf /etc/blackcyclone/users') -join ' '
    Check 'Quyền file cấu hình' ($perms -match 'config.json=640:root:blackcyclone' -and $perms -match 'server.conf=600:root:root' -and $perms -match 'users=600:root:root') $perms
    Check 'Lệnh bcserver đã cài' ((Vps 'command -v bcserver') -contains '/usr/local/bin/bcserver')
    if ($ufwOn) {
        Check "ufw đã mở cổng $vpnPort" (((Vps 'ufw status') -join ' ') -match "$vpnPort/tcp")
    }
    $cc = (Vps 'sysctl -n net.ipv4.tcp_congestion_control') -join ''
    Write-Host "   (thông tin) TCP congestion control trong container: $cc"

    Step 'Kết nối qua server bằng link'
    $code = Test-Link $link1
    Check 'Kết nối bằng link tester' ($code -eq '204') "HTTP $code"

    Step 'Deploy lần 2 (cập nhật): khoá và link phải giữ nguyên'
    Deploy
    Check 'Deploy lại thành công' ($LASTEXITCODE -eq 0) "mã $LASTEXITCODE"
    $link1b = Get-Link 'tester'
    Check 'Link không đổi sau khi deploy lại' ($link1b -eq $link1)
    $code = Test-Link $link1b
    Check 'Vẫn kết nối được' ($code -eq '204') "HTTP $code"

    Step 'Thêm / xoá người dùng'
    Vps 'bcserver add phone' | Out-Null
    $link2 = Get-Link 'phone'
    Check 'Có link cho phone' ([bool]$link2)
    Vps 'bcserver remove tester' | Out-Null
    $code = Test-Link $link1
    Check 'Link tester bị chặn sau khi remove' ($code -ne '204') "HTTP $code"
    $code = Test-Link $link2
    Check 'Link phone vẫn dùng được' ($code -eq '204') "HTTP $code"
    $out = (Vps 'bcserver remove phone; echo rc=$?') -join ' '
    Check 'Không xoá được người dùng cuối cùng' ($out -match 'rc=1') $out

    Step 'Khởi động lại VPS: service phải tự chạy lại'
    docker restart $name | Out-Null
    Must 'docker restart'
    Wait-Vps | Out-Null
    Start-Sleep -Seconds 2
    Check 'Service tự chạy sau reboot' ((Vps 'systemctl is-active blackcyclone') -contains 'active')
    $code = Test-Link $link2
    Check 'Kết nối được sau reboot' ($code -eq '204') "HTTP $code"

    Step 'Gỡ cài đặt'
    Vps 'bcserver uninstall --yes' | Out-Null
    $left = (Vps 'ls -d /etc/blackcyclone /opt/blackcyclone /usr/local/bin/bcserver /etc/systemd/system/blackcyclone.service 2>/dev/null; id blackcyclone 2>/dev/null') -join ' '
    Check 'Gỡ sạch' (-not $left.Trim()) $left
    $code = Test-Link $link2
    Check 'Server không còn nhận kết nối' ($code -ne '204') "HTTP $code"
}
catch {
    Write-Host "`nLỖI: $_" -ForegroundColor Red
    $exitCode = 2
}
finally {
    if ($Keep) {
        Write-Host "`nGiữ container $name (docker rm -f $name để xoá). Khoá SSH: $key"
    }
    else {
        docker rm -f $name 2>&1 | Out-Null
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }
}

Write-Host ''
$results | Format-Table -AutoSize | Out-String -Width 200 | Write-Host
$failed = @($results | Where-Object { $_.Result -eq 'FAIL' }).Count
if ($exitCode -eq 0 -and $failed -gt 0) { $exitCode = 1 }
if ($exitCode -eq 0) { Write-Host "Tất cả $($results.Count) kiểm tra đều đạt." -ForegroundColor Green }
else { Write-Host "$failed kiểm tra lỗi." -ForegroundColor Red }
exit $exitCode
