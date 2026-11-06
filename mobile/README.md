# BlackCyclone (Android)

Bản Android của [Black Cyclone](../README.md), có hai chế độ giống bản máy tính: **WARP** và **Server riêng**.
Kotlin + Jetpack Compose làm giao diện và `VpnService`. Lõi Go build bằng `gomobile`, chạy **sing-box** ngay trong app.

## Cùng một lõi trên cả ba nền tảng

Windows, Linux và Android đều dùng sing-box 1.14.3, cùng phiên bản với server cài bằng [`server/`](../server/README.md).
Lõi Android dùng lại các package Go của bản máy tính:

| Package | Việc |
|---|---|
| `internal/singbox` | sinh config sing-box từ cài đặt (WARP / Server riêng, DNS, chia tuyến) |
| `internal/share` | đọc link `vless://`, `vmess://`, `trojan://`, `ss://`, `hysteria2://`, `tuic://` |
| `internal/warp` | đăng ký WARP, key WARP+, quét endpoint |
| `internal/clash` | thống kê lưu lượng, đổi server khi đang kết nối |

Khác biệt duy nhất nằm ở cách mở card mạng ảo. App tạo TUN bằng `VpnService.Builder`, gồm địa chỉ, route, DNS và chia tuyến theo ứng dụng, rồi giao file descriptor cho sing-box (`core/rcvcore/platform.go`). Danh sách mạng và mạng mặc định do phía Kotlin báo cho lõi, vì Android 11+ không cho app đọc trực tiếp.

## Tính năng

- **WARP:** đăng ký miễn phí ngay trong app, nhập key WARP+, xuất / nhập tài khoản (JSON giống `settings.Warp` của desktop), quét endpoint bằng handshake WireGuard thật.
- **Server riêng:** dán một hoặc nhiều link (ví dụ link do `bcserver` in ra), chọn server hoặc **Tự động** (server có độ trễ thấp nhất). Đổi server khi đang kết nối không làm rớt VPN. Link được mã hoá bằng Android Keystore như tài khoản WARP.
- Việt Nam đi thẳng (trang `.vn`, IP Việt Nam), bỏ qua mạng LAN, chia tuyến theo ứng dụng (loại trừ app / chỉ app đã chọn).
- Kiểm tra kết nối thật: cứ 20 giây lõi gửi một yêu cầu HTTPS qua đường hầm, đo độ trễ và cảnh báo nếu không thông.
- Tự kết nối lại, tự chuyển mạng khi đổi Wi-Fi ↔ 4G, ô Cài đặt nhanh, hỗ trợ Always-on VPN.
- Kill switch: trong lúc kết nối lại hoặc khi lỗi, app giữ giao diện VPN và chặn mọi kết nối (không lộ IP thật) đến khi nối lại được hoặc bấm Ngắt. Để kín cả khi app bị tắt hẳn hay lúc khởi động máy, bật *Luôn bật VPN* + *Chặn kết nối không qua VPN* của Android (app cảnh báo nếu chưa bật).

## Build

Cần: Go, JDK 17, Android SDK + NDK (mặc định `D:\Android\Sdk`).

```powershell
powershell -ExecutionPolicy Bypass -File build-apk.ps1          # test Go, build lõi .aar, ra APK release
powershell -ExecutionPolicy Bypass -File build-apk.ps1 -Debug   # APK debug
powershell -ExecutionPolicy Bypass -File build-apk.ps1 -SkipCore # chỉ build lại phần Kotlin
```

Nên chạy `windows\fetch-bins.ps1` trước để có `bin\geoip-vn.srs`: file này được cho vào APK. Không có thì sing-box tự tải lần kết nối đầu.

APK: `android\app\build\outputs\apk\release\`, bản release được chép ra `dist\`.

Ký APK: `android\keystore\release.jks` + `android\keystore.properties` (không commit).
**Hãy sao lưu 2 file này**: mất khoá thì bản sau không cài đè lên bản cũ được.

## Giấy phép

App Android nhúng sing-box (GPL-3.0) vào lõi, nên **APK được phát hành theo GNU GPL 3.0**. Mã nguồn tương ứng là repo này ở đúng tag phát hành. Mã nguồn Black Cyclone vẫn theo MIT. App hiện các giấy phép trong *Cài đặt → Giới thiệu → Giấy phép mã nguồn mở*. Chi tiết: [THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md).

## Cấu trúc

```
core/rcvcore       API cho Kotlin qua gomobile (lớp Java rcvcore.Rcvcore): chạy sing-box,
                   platform.go nối sing-box với VpnService
core/go.mod        module riêng (kéo sing-box), dùng package internal/ của repo qua replace
android/           app Kotlin / Compose
build-apk.ps1      build tất cả (build tags của sing-box nằm ở đây)
```

## Test

```powershell
cd core
go test -tags with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api,badlinkname,tfogo_checklinkname0 -ldflags=-checklinkname=0 ./...
```

`TestConfigsLoad` sinh config WARP và Server riêng rồi cho sing-box nạp thử. Test này báo lỗi nếu thiếu build tag cần thiết, ví dụ thiếu uTLS thì Reality không chạy.
Test gọi API WARP thật: `cd ..\..; $env:RCV_LIVE=1; go test ./internal/warp`.
