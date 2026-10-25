# Black Cyclone cho Linux

Bản Linux dùng chung code với bản Windows. Các phần khác biệt theo hệ điều hành nằm trong file `*_linux.go` / `*_windows.go` cạnh nhau trong từng package. Thư mục này chỉ chứa script build, cài đặt và đóng gói.

| File | Làm gì |
|---|---|
| `build.sh` | `wails build`, tự chọn WebKitGTK 4.1 hoặc 4.0 → `build/bin/BlackCyclone` |
| `fetch-bins.sh` | tải sing-box bản Linux (phiên bản cố định, kiểm SHA-256) + `geoip-vn.srs` vào `bin/` |
| `install.sh` | cài vào `/opt/blackcyclone`, cấp quyền mạng, thêm vào menu ứng dụng; `--uninstall` để gỡ |
| `blackcyclone.desktop`, `blackcyclone.png` | mục menu và icon |

## Cài môi trường build

Debian / Ubuntu (24.04 trở lên):

```sh
sudo apt install golang-go build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev nodejs npm libcap2-bin
```

Fedora: `sudo dnf install golang gtk3-devel webkit2gtk4.1-devel nodejs npm libcap`
Arch: `sudo pacman -S go gtk3 webkit2gtk-4.1 nodejs npm libcap`

Cần Go đúng phiên bản trong `go.mod`. Nếu distro có bản cũ hơn, tải Go tại https://go.dev/dl. Sau đó cài Wails:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0
```

## Build và cài

```sh
linux/fetch-bins.sh
linux/build.sh
sudo linux/install.sh
```

Mở “Black Cyclone” từ menu ứng dụng, hoặc chạy `blackcyclone`. Gỡ cài đặt: `sudo linux/install.sh --uninstall`.

Chạy thử khi đang phát triển, không cần cài: `wails dev -tags webkit2_41`. Muốn kết nối được thì phải cấp quyền cho file app tạm mà Wails tạo ra (`sudo setcap cap_net_admin,cap_net_raw+ep build/bin/BlackCyclone`), hoặc chạy cả lệnh bằng `sudo -E`.

## Quyền: không cần chạy bằng root

Để tạo card mạng ảo (TUN) và đổi bảng định tuyến, sing-box cần quyền `CAP_NET_ADMIN`. Chạy cả giao diện webview bằng root thì không an toàn, nên app dùng cách sau:

- `install.sh` cấp `cap_net_admin,cap_net_raw` cho **riêng file app** bằng `setcap`. App chạy với user bình thường.
- Khi khởi động sing-box, app chuyển hai quyền này cho nó qua *ambient capabilities*. File sing-box không cần `setcap`.
- sing-box được gắn `PDEATHSIG=SIGTERM`, thay cho Job Object trên Windows: app bị tắt hay crash, kể cả bị `kill -9`, thì sing-box nhận SIGTERM, tự gỡ route rồi thoát. Lần mở app sau, các tiến trình sing-box còn sót (nếu có) cũng được dọn.

Lưu ý bảo mật: mọi user chạy được `/opt/blackcyclone/BlackCyclone` đều dùng được hai quyền mạng đó, vì app giữ chúng. Đây là cùng một mô hình với các client VPN khác dùng `setcap`. Thư mục `/opt/blackcyclone` thuộc root để không ai tráo được file sing-box.

## Khác bản Windows

| | Windows | Linux |
|---|---|---|
| Chống DPI (chế độ Vượt chặn) | GoodbyeDPI (WinDivert) | Phân mảnh TLS có sẵn trong sing-box (`tls_fragment` / `tls_record_fragment`) + chặn QUIC. Không có tuỳ chỉnh tham số GoodbyeDPI. |
| Mã hoá `settings.json` | DPAPI | XChaCha20-Poly1305, khoá ngẫu nhiên ở `~/.config/BlackCyclone/settings.key` (quyền 0600). Chống lộ khi copy riêng `settings.json` đi đâu đó, nhưng không chống được người đọc được cả thư mục. |
| Khởi động cùng máy | Task Scheduler | `~/.config/autostart/blackcyclone.desktop` |
| Dữ liệu | `%APPDATA%\BlackCyclone` | `~/.config/BlackCyclone` |
| Ứng dụng đi thẳng | `steam.exe` | tên tiến trình, ví dụ `steam` |

Ánh xạ kiểu chống DPI: `9` → chia gói TCP, `9q` → chia gói TCP + chặn QUIC, `5` → chia bản ghi TLS (nhẹ). Bản Windows giữ nguyên GoodbyeDPI. Settings dùng chung tên preset nên copy cấu hình qua lại vẫn được.

## Khay hệ thống

Icon khay dùng giao thức StatusNotifierItem: KDE, XFCE, Cinnamon, MATE hiện sẵn. GNOME cần cài extension **AppIndicator and KStatusNotifierItem Support** (Ubuntu đã cài sẵn). Không có khay thì đừng bật “Ẩn xuống khay khi khởi động”, và tắt “Bấm X thì thu nhỏ xuống khay” để nút đóng thoát hẳn app.

## Test

```sh
go test ./internal/...      # cần bin/sing-box (linux/fetch-bins.sh) cho các test chạy sing-box thật
TRAY_SMOKE=0 go test ./internal/tray   # khi không có phiên desktop / D-Bus
```
