# BlackCyclone (Android)

Bản Android của chế độ **Cloudflare WARP** trong [RedCyclone VPN](../redcyclone-vpn) (desktop).
Kotlin + Jetpack Compose làm giao diện / `VpnService`, lõi Go (`wireguard-go` userspace) build bằng `gomobile`.

## Vì sao không dùng thư viện WireGuard Android chính thức

WARP dùng 3 byte "reserved" trong header WireGuard làm `client_id` để cân bằng tải
(Cloudflare edge băm theo `client_id` thay vì cổng nguồn). Client WireGuard chuẩn luôn gửi 0,
nên hay rớt kết nối khi NAT đổi cổng / đổi Wi-Fi ↔ 4G, có nơi không bắt tay được.
Lõi ở đây bọc `conn.Bind` của `wireguard-go` (`core/tunnel/bind.go`): ghi `client_id` vào mọi gói gửi đi
và xoá về 0 ở gói nhận trước khi `wireguard-go` xử lý (cùng cách sing-box làm).

## Tính năng (giống chế độ WARP bản desktop)

- Đăng ký WARP miễn phí ngay trong app, nhập key WARP+, xuất / nhập tài khoản (JSON giống `settings.Warp` của desktop).
- Quét endpoint bằng handshake WireGuard thật, chọn IP:port còn sống và nhanh nhất.
- Bỏ qua mạng LAN, chia tuyến theo ứng dụng (loại trừ app / chỉ app đã chọn).
- Tự kết nối lại, tự rebind khi đổi mạng, ô Cài đặt nhanh (Quick Settings), hỗ trợ Always-on VPN.
- Kill switch: trong lúc kết nối lại / khi lỗi, app giữ giao diện VPN và chặn mọi kết nối (không lộ IP thật) đến khi nối lại được hoặc bấm Ngắt. Để kín cả khi app bị tắt hẳn hay lúc khởi động máy, bật *Luôn bật VPN* + *Chặn kết nối không qua VPN* của Android (app cảnh báo nếu chưa bật).

## Build

Cần: Go, JDK 17, Android SDK + NDK (mặc định `D:\Android\Sdk`).

```powershell
powershell -ExecutionPolicy Bypass -File build-apk.ps1          # test Go, build lõi .aar, ra APK release
powershell -ExecutionPolicy Bypass -File build-apk.ps1 -SkipCore # chỉ build lại app Android
```

APK: `android\app\build\outputs\apk\release\`.

Ký APK: `android\keystore\release.jks` + `android\keystore.properties` (không commit).
**Hãy sao lưu 2 file này** — mất khoá thì bản sau không cài đè lên bản cũ được.

## Cấu trúc

```
core/warp          đăng ký WARP, key WARP+, quét endpoint (chép từ desktop internal/warp)
core/tunnel        wireguard-go trên fd TUN của Android + Bind ghi byte reserved
core/rcvcore       API cho Kotlin qua gomobile (lớp Java rcvcore.Rcvcore)
android/           app Kotlin / Compose
build-apk.ps1      build tất cả
```

## Test

```powershell
cd core; go test ./...
$env:RCV_LIVE=1; go test ./warp   # gọi API WARP thật + quét endpoint
```
