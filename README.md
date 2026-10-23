# RedCyclone VPN

Ứng dụng Windows giúp vượt chặn của nhà mạng. Go + Wails làm giao diện / điều khiển, `sing-box` làm lõi TUN, `GoodbyeDPI` chống DPI.

## Chế độ

| Chế độ | Làm gì | Đổi IP? |
|---|---|---|
| **Vượt chặn** (`bypass`) | Toàn bộ DNS qua DoH (TUN + hijack DNS) + GoodbyeDPI `-9` chống chặn SNI/DPI | Không |
| **Cloudflare WARP** (`warp`) | Đăng ký WARP miễn phí ngay trong app, toàn bộ traffic qua WireGuard | Có |
| **Server riêng** (`custom`) | Dán link `vless://` (Reality / TLS, tcp/ws/grpc) từ VPS chạy Xray hoặc sing-box | Có |

## Cài đặt môi trường

```powershell
winget install GoLang.Go
go install github.com/wailsapp/wails/v2/cmd/wails@latest
powershell -ExecutionPolicy Bypass -File scripts\fetch-bins.ps1   # tải sing-box + GoodbyeDPI vào bin\
```

## Chạy / build

```powershell
wails dev      # chạy thử (mở terminal bằng quyền Administrator)
wails build    # ra build\bin\redcyclone-vpn.exe (manifest yêu cầu Administrator)
```

Khi phát hành, đặt thư mục `bin\` cạnh `redcyclone-vpn.exe`. Ở chế độ dev, app tự tìm `bin\` ở gốc project.

## Cấu trúc

```
app.go                     API cho frontend: Connect/Disconnect/GetStatus/Settings/RegisterWarp
internal/settings          lưu cài đặt ở %APPDATA%\RedCyclone\settings.json
internal/singbox           sinh config sing-box (test chạy `sing-box check` thật)
internal/runner            chạy/dừng sing-box.exe, goodbyedpi.exe, đẩy log lên UI
internal/warp              đăng ký Cloudflare WARP (API giống wgcf)
internal/vless             parse link vless://
frontend/                  UI (vanilla JS + Vite)
scripts/fetch-bins.ps1     tải binary phụ trợ
```

## Test

```powershell
go test ./internal/...                       # kiểm tra config với sing-box check
$env:RCV_LIVE=1; go test ./internal/warp     # gọi API WARP thật
```
