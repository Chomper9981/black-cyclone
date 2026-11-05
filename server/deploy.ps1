# Dựng (hoặc cập nhật) server Black Cyclone lên một VPS, chạy từ máy Windows.
# Dùng OpenSSH có sẵn trong Windows 10/11: chép server\bcserver.sh lên VPS rồi chạy
# "bcserver install". Cuối cùng in ra link để dán vào app (chế độ Server riêng).
#
# Ví dụ:
#   powershell -ExecutionPolicy Bypass -File server\deploy.ps1 -Server 203.0.113.10
#   powershell -ExecutionPolicy Bypass -File server\deploy.ps1 -Server 203.0.113.10 -User ubuntu -Identity ~\.ssh\id_ed25519
#   powershell -ExecutionPolicy Bypass -File server\deploy.ps1 -Server 203.0.113.10 -Port 8443 -Sni www.apple.com
#
# Chạy lại bất cứ lúc nào để cập nhật: khoá và người dùng trên VPS được giữ nguyên.
param(
    [Parameter(Mandatory = $true)][string]$Server, # IP hoặc tên miền của VPS
    [string]$User = 'root',                         # user SSH (không phải root thì cần sudo)
    [int]$SshPort = 22,
    [string]$Identity = '',                         # file khoá SSH (-i), nếu có
    [int]$Port = 0,                                 # cổng server (mặc định 443)
    [string]$Sni = '',                              # trang Reality giả làm (mặc định www.microsoft.com)
    [string]$Address = '',                          # địa chỉ ghi vào link (mặc định: IP công khai của VPS)
    [string]$Name = '',                             # tên người dùng đầu tiên (mặc định "default")
    [string[]]$SshOption = @()                      # thêm tuỳ chọn -o cho ssh/scp, vd. 'StrictHostKeyChecking=accept-new'
)
$ErrorActionPreference = 'Stop'

foreach ($tool in 'ssh', 'scp') {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
        throw "Không tìm thấy $tool. Cài 'OpenSSH Client' trong Settings > System > Optional features."
    }
}

function Assert-Match([string]$value, [string]$pattern, [string]$what) {
    if ($value -and $value -notmatch $pattern) { throw "$what không hợp lệ: $value" }
}
# Các giá trị được ghép vào lệnh chạy trên VPS: chỉ cho phép ký tự an toàn.
Assert-Match $Server '^[A-Za-z0-9.:-]+$' 'Server'
Assert-Match $User '^[A-Za-z_][A-Za-z0-9._-]*$' 'User'
Assert-Match $Sni '^[A-Za-z0-9.-]+$' 'Sni'
Assert-Match $Address '^[A-Za-z0-9.:-]+$' 'Address'
Assert-Match $Name '^[A-Za-z0-9._-]{1,32}$' 'Name'
if ($Port -lt 0 -or $Port -gt 65535) { throw "Port không hợp lệ: $Port" }

# bash không chạy được file có CRLF: gửi bản LF nếu git đã đổi xuống dòng.
$src = Join-Path $PSScriptRoot 'bcserver.sh'
$text = [IO.File]::ReadAllText($src)
$upload = $src
if ($text.Contains("`r`n")) {
    $upload = Join-Path $env:TEMP ('bcserver-' + [guid]::NewGuid() + '.sh')
    [IO.File]::WriteAllText($upload, $text.Replace("`r`n", "`n"), (New-Object Text.UTF8Encoding $false))
}

$sshOpts = @('-p', "$SshPort")
$scpOpts = @('-P', "$SshPort")
if ($Identity) {
    $sshOpts += @('-i', $Identity)
    $scpOpts += @('-i', $Identity)
}
foreach ($o in $SshOption) {
    $sshOpts += @('-o', $o)
    $scpOpts += @('-o', $o)
}
$target = "$User@$Server"
$scpTarget = if ($Server.Contains(':')) { "$User@[$Server]" } else { $target }

$installArgs = @('install')
if ($Port) { $installArgs += @('--port', "$Port") }
if ($Sni) { $installArgs += @('--sni', $Sni) }
if ($Address) { $installArgs += @('--host', $Address) }
if ($Name) { $installArgs += @('--name', $Name) }
$sudo = if ($User -eq 'root') { '' } else { 'sudo ' }
$remote = "${sudo}bash ~/bcserver.sh $($installArgs -join ' '); rc=`$?; rm -f ~/bcserver.sh; exit `$rc"

try {
    Write-Host "==> Chép bcserver.sh lên $Server"
    & scp @scpOpts $upload "${scpTarget}:bcserver.sh"
    if ($LASTEXITCODE -ne 0) { throw "scp thất bại (mã $LASTEXITCODE)" }

    Write-Host "==> Cài đặt trên VPS"
    & ssh -t @sshOpts $target $remote
    if ($LASTEXITCODE -ne 0) { throw "cài đặt trên VPS thất bại (mã $LASTEXITCODE), xem thông báo ở trên" }
}
finally {
    if ($upload -ne $src) { Remove-Item -Force $upload -ErrorAction SilentlyContinue }
}

Write-Host ''
Write-Host 'Xong. Copy link vless:// ở trên, mở Black Cyclone > Server riêng > dán link.'
Write-Host "Quản lý về sau: ssh $target, rồi chạy ${sudo}bcserver help"
