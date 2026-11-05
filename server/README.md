# Server riêng cho Black Cyclone

Biến một VPS Linux trống thành server cho chế độ **Server riêng** của app. Không cần tên miền, chứng chỉ, panel hay dịch vụ trả phí nào khác: chỉ cần thuê VPS.

Server chạy `sing-box` (cùng phiên bản với app) với **VLESS + Reality**. Nhà mạng chỉ thấy một kết nối HTTPS bình thường tới một trang lớn (mặc định `www.microsoft.com`). Ai dò thử server cũng nhận về đúng trang đó, nên server khó bị phát hiện để chặn.

## 1. Thuê VPS

- **Vị trí:** Singapore, Nhật hoặc Hồng Kông để ping từ Việt Nam thấp.
- **Cấu hình:** gói rẻ nhất là đủ (1 vCPU, 512 MB–1 GB RAM). Một server phục vụ thoải mái cả gia đình.
- **Hệ điều hành:** Ubuntu 22.04/24.04 hoặc Debian 12. Rocky/Alma cũng chạy được. Cần x86_64 hoặc arm64, có systemd.
- **Băng thông:** xem hạn mức traffic/tháng. Mọi thứ bạn duyệt web qua VPN đều tính vào đó.

Nhà cung cấp gửi cho bạn **IP** và **mật khẩu root** (hoặc user + khoá SSH).

## 2. Cài đặt (một lệnh, chạy từ máy bạn)

**Windows** (PowerShell, trong thư mục repo):

```powershell
powershell -ExecutionPolicy Bypass -File server\deploy.ps1 -Server 203.0.113.10
```

**Linux / macOS:**

```sh
server/deploy.sh root@203.0.113.10
```

Lệnh sẽ hỏi mật khẩu SSH hai lần: một lần khi chép script, một lần khi cài. Cài xong, màn hình in ra link dạng:

```
vless://…@203.0.113.10:443?…&security=reality&…#203.0.113.10-default
```

Mở Black Cyclone, chọn **Server riêng**, dán link rồi kết nối.

Tuỳ chọn hay dùng:

| Windows | Linux / macOS | Ý nghĩa |
|---|---|---|
| `-User ubuntu` | `ubuntu@IP` | User SSH không phải root (script tự dùng `sudo`) |
| `-Identity ~\.ssh\id_ed25519` | `-i ~/.ssh/id_ed25519` | Đăng nhập bằng khoá SSH |
| `-SshPort 2222` | `-p 2222` | Cổng SSH khác 22 |
| `-Port 8443` | `--port 8443` | Cổng server (mặc định 443) |
| `-Sni www.apple.com` | `--sni www.apple.com` | Trang mà server giả làm |
| `-Address vpn.example.com` | `--host vpn.example.com` | Ghi tên miền vào link thay cho IP |
| `-Name dien-thoai` | `--name dien-thoai` | Tên người dùng đầu tiên (mặc định `default`) |

Không muốn dùng script deploy thì chép `bcserver.sh` lên VPS rồi chạy `sudo bash bcserver.sh install`.

### Script làm những gì trên VPS

1. Tải sing-box bản cố định, kiểm SHA-256, đặt vào `/opt/blackcyclone/`.
2. Tạo khoá Reality, short id và người dùng đầu tiên, lưu ở `/etc/blackcyclone/` (chỉ root đọc được).
3. Chạy server thành service systemd `blackcyclone` dưới user riêng không có quyền root, chỉ được mở cổng thấp. Service tự khởi động cùng VPS và tự chạy lại nếu bị lỗi.
4. Bật **TCP BBR**: tốc độ ổn định hơn nhiều trên đường truyền xa, hay rớt gói (Việt Nam ↔ nước ngoài).
5. Mở cổng trên tường lửa của VPS (ufw, firewalld, hoặc iptables như ảnh Ubuntu của Oracle Cloud).
6. Chặn client đi vào mạng nội bộ của VPS (`10.x`, `192.168.x`, metadata của nhà cung cấp…).
7. Cài lệnh quản lý `bcserver`.

Script tìm IP công khai của VPS bằng `api.ipify.org`, dự phòng `icanhazip.com` và `ifconfig.me`. Dùng `--host` để bỏ qua bước này.

## 3. Quản lý

SSH vào VPS rồi dùng `bcserver` (thêm `sudo` nếu không phải root):

```sh
bcserver add laptop-me      # thêm người dùng, in link của người đó
bcserver list               # liệt kê người dùng
bcserver link               # in lại link của mọi người (bcserver link laptop-me: chỉ một người)
bcserver remove laptop-me   # xoá: link của người đó hết hiệu lực ngay
bcserver status             # trạng thái, địa chỉ, số người dùng
bcserver logs -f            # xem log
bcserver restart
bcserver uninstall          # gỡ hoàn toàn
```

Nên tạo **một tên cho mỗi người / thiết bị**. Khi lộ link hay muốn cắt quyền ai, chỉ cần `remove` đúng tên đó, những người khác không bị ảnh hưởng.

**Cập nhật:** chạy lại lệnh deploy ở bước 2. Khoá và người dùng được giữ nguyên, link cũ vẫn dùng được. Truyền `-Port` / `-Sni` / `-Address` khi chạy lại để đổi các giá trị đó; khi đổi, chạy `bcserver link` rồi dán lại link mới vào app.

## Xử lý sự cố

- **App báo lỗi kết nối / timeout:** nhà cung cấp thường có tường lửa riêng trên trang quản lý (Security Group, Firewall, Security List…). Mở cổng 443/tcp, hoặc cổng bạn chọn, ở đó.
- **`cổng 443 đang bị chương trình khác dùng`:** VPS đã chạy sẵn web server. Tắt nó đi, hoặc cài với `--port 8443`.
- **`VPS không truy cập được https://… bằng TLS 1.3`:** chọn `--sni` khác, ví dụ `www.apple.com`, `dl.google.com`, `www.amazon.com`. Trang được chọn phải hỗ trợ TLS 1.3 và truy cập được từ VPS.
- **Đang dùng tốt rồi đột nhiên không vào được:** có thể IP VPS đã bị chặn. Thử đổi cổng trước (`-Port 8443`). Nếu vẫn không được, thuê VPS mới (hoặc đổi IP nếu nhà cung cấp cho phép) rồi chạy lại deploy.
- **Chậm vào giờ cao điểm:** thường do đường truyền quốc tế của nhà mạng hoặc gói VPS giá rẻ quá tải. Thử VPS ở vị trí khác.
- Xem log chi tiết: `bcserver logs`.

## Bảo mật

- Link chứa đủ thông tin để dùng server: coi nó như mật khẩu, đừng đăng công khai.
- Nên đăng nhập SSH bằng khoá và tắt đăng nhập bằng mật khẩu. Đây là cấu hình của VPS, script không tự đổi.
- File cấu hình chỉ root và service đọc được; service chạy dưới user `blackcyclone` với quyền tối thiểu.
