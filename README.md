# Black Cyclone

Ứng dụng giúp vượt chặn của nhà mạng, cho Windows, Linux và Android. Go + Wails làm giao diện / điều khiển, `sing-box` làm lõi TUN, traffic đi qua Cloudflare WARP hoặc server riêng.

Tác giả: **Red Cyclone** (GitHub: Chomper9981). Mã nguồn phát hành theo giấy phép [MIT](LICENSE).

- Bản Linux: xem [`linux/README.md`](linux/README.md) (build, cài, cấp quyền, khác biệt với bản Windows).
- Bản Android (cả WARP lẫn Server riêng, cùng lõi sing-box): xem [`mobile/README.md`](mobile/README.md).
- Tự dựng server riêng trên một VPS trống (một lệnh, không cần tên miền hay dịch vụ trả phí): xem [`server/README.md`](server/README.md).

Phần dưới đây mô tả bản Windows; phần lớn tính năng giống hệt trên Linux.

## Chế độ

| Chế độ | Làm gì | Đổi IP? |
|---|---|---|
| **Cloudflare WARP** (`warp`) | Đăng ký WARP miễn phí ngay trong app, toàn bộ traffic qua WireGuard. Hỗ trợ key WARP+, quét endpoint (handshake WireGuard thật) để tìm IP:port còn sống và nhanh nhất | Có |
| **Server riêng** (`custom`) | Nhiều server từ link `vless://` `vmess://` `trojan://` `ss://` `hysteria2://` `tuic://` hoặc subscription URL. Chọn tay hoặc **Tự động** (urltest chọn server nhanh nhất), đổi server khi đang kết nối không cần ngắt | Có |

## Tính năng

- **Chia tuyến**: web Việt Nam (`.vn` + dải IP VN) đi thẳng, danh sách domain đi thẳng / bắt buộc qua VPN, chương trình đi thẳng (vd. `steam.exe`), LAN đi thẳng.
- **Kill switch**: khi lõi chết, app giữ TUN và chặn mọi kết nối để không lộ IP thật, đến khi kết nối lại được hoặc bạn ngắt.
- **Tự kết nối lại** (2s → 30s, 5 lần) và thông báo Windows khi rớt / nối lại.
- Tốc độ và lưu lượng realtime (qua Clash API của sing-box), đo ping server, **kiểm tra kết nối** (IP công khai + truy cập thử các trang hay bị chặn).
- Icon khay hệ thống, đóng cửa sổ thì thu xuống khay, khởi động cùng Windows (Task Scheduler, quyền cao nhất), tự kết nối khi mở, chỉ chạy một bản.
- Tiến trình con gắn vào Job Object: app crash thì sing-box cũng tắt theo; dừng êm bằng Ctrl-Break để sing-box gỡ route sạch.
- `settings.json` được mã hoá bằng DPAPI (khoá WireGuard, link server); `config.json` bị xoá ngay sau khi sing-box khởi động.

## Cài đặt môi trường

```powershell
winget install GoLang.Go
go install github.com/wailsapp/wails/v2/cmd/wails@latest
powershell -ExecutionPolicy Bypass -File windows\fetch-bins.ps1   # tải sing-box (phiên bản cố định, kiểm SHA-256) vào bin\
```

## Chạy / build

```powershell
wails dev                 # chạy thử (mở terminal bằng quyền Administrator)
wails build               # ra build\bin\BlackCyclone.exe (manifest yêu cầu Administrator)
wails build -nsis         # thêm bộ cài, đã gói kèm bin\
```

Khi phát hành không dùng bộ cài, đặt thư mục `bin\` cạnh `BlackCyclone.exe`. Ở chế độ dev, app tự tìm `bin\` ở gốc project.

Nâng cấp sing-box: sửa URL + SHA-256 trong `windows\fetch-bins.ps1`, chạy lại rồi `go test ./internal/...` (test chạy `sing-box check` thật với mọi kiểu config).

## Cấu trúc

```
app.go                     vòng đời kết nối: Connect/Disconnect, tự nối lại, kill switch, thống kê, khay
tools.go                   API phụ cho UI: server/subscription, WARP+/quét endpoint, ping, kiểm tra kết nối
internal/settings          cài đặt ở %APPDATA%\BlackCyclone\settings.json (mã hoá DPAPI)
internal/singbox           sinh config sing-box (test chạy `sing-box check` thật)
internal/share             parse link vless/vmess/trojan/ss/hysteria2/tuic → outbound sing-box
internal/subscription      tải + gộp subscription
internal/warp              đăng ký WARP, key WARP+, quét endpoint bằng handshake WireGuard
internal/clash             client Clash API của sing-box (lưu lượng, đổi server, đo delay)
internal/nettest           IP công khai, kiểm tra trang web, TCP ping
internal/runner            chạy/dừng sing-box (Windows: Job Object + Ctrl-Break; Linux: PDEATHSIG + SIGTERM), đẩy log lên UI
internal/tray              icon khay hệ thống
internal/autostart         khởi động cùng máy (Windows: Task Scheduler; Linux: XDG autostart)
frontend/                  UI (vanilla JS + Vite)
windows/                   script tải binary phụ trợ cho Windows
linux/                     script build / tải binary / cài đặt cho Linux
mobile/                    bản Android (Kotlin + Go core qua gomobile)
server/                    script dựng và quản lý server VLESS + Reality trên VPS cho chế độ Server riêng
```

Code riêng cho từng hệ điều hành nằm trong file `*_windows.go` / `*_linux.go` cạnh phần dùng chung; Go tự chọn file theo hệ điều hành lúc build. Thư mục `windows/` và `linux/` chỉ chứa script build / cài đặt / đóng gói của từng nền tảng; `mobile/` là bản Android riêng.

## Test

```powershell
go test ./internal/...                       # gồm kiểm tra config với sing-box check thật
$env:RCV_LIVE=1; go test ./internal/warp     # gọi API WARP thật + quét endpoint
```

## Giấy phép

Mã nguồn của Black Cyclone phát hành theo giấy phép [MIT](LICENSE), © 2026 Red Cyclone.
sing-box (tải về bằng script `fetch-bins`, kèm trong bộ cài) là phần mềm của SagerNet, theo giấy phép GPL-3.0-or-later.
App Android nhúng sing-box vào lõi nên APK được phát hành theo GPL-3.0 (xem [`mobile/README.md`](mobile/README.md#giấy-phép)).
Danh sách đầy đủ phần mềm bên thứ ba, nguồn gốc và giấy phép: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) (thư mục [`licenses/`](licenses)).
