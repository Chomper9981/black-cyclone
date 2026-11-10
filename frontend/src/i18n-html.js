// English for the static text of index.html, keyed by the Vietnamese source
// (innerHTML or attribute value, whitespace collapsed). i18n.js looks these up;
// window.__i18nMissing() in the dev console lists source texts with no entry.
export default {
    "Black <b>Cyclone</b>":
        "Black <b>Cyclone</b>",
    "Chưa kết nối":
        "Not connected",
    "Kết nối":
        "Connect",
    "Tốc độ tải xuống hiện tại / tổng đã tải từ lúc kết nối":
        "Current download speed / total downloaded since connecting",
    "0 B/s":
        "0 B/s",
    "0 B":
        "0 B",
    "Tốc độ tải lên hiện tại / tổng đã gửi từ lúc kết nối":
        "Current upload speed / total sent since connecting",
    "Kill switch đang chặn mạng":
        "The kill switch is blocking the network",
    "VPN bị rớt nên mọi kết nối Internet đang bị chặn để không lộ IP thật.":
        "The VPN dropped, so all Internet connections are blocked to keep your real IP hidden.",
    "Ngắt kết nối":
        "Disconnect",
    "Kết nối lại":
        "Reconnect",
    "Cài đặt đã thay đổi — Kết nối lại để áp dụng":
        "Settings changed — reconnect to apply them",
    "VPS":
        "VPS",
    "Định tuyến":
        "Routing",
    "Ứng dụng":
        "App",
    "Công cụ":
        "Tools",
    "Chọn đường đi cho traffic. Cài đặt riêng của chế độ đang chọn hiện ở bên dưới. Bấm <span class=\"help-icon\" aria-hidden=\"true\">?</span> cạnh mỗi mục để xem giải thích chi tiết.":
        "Choose the path your traffic takes. The settings of the selected mode appear below. Click <span class=\"help-icon\" aria-hidden=\"true\">?</span> next to an item for a detailed explanation.",
    "Chế độ":
        "Mode",
    "Giải thích":
        "Explain",
    "<b>Cloudflare WARP</b> — toàn bộ traffic đi qua mạng Cloudflare (miễn phí, mã hoá WireGuard). Website thấy IP Cloudflare thay vì IP của bạn. Một số trang chặn hoặc bắt captcha với IP WARP; tốc độ phụ thuộc giờ cao điểm.":
        "<b>Cloudflare WARP</b> — all traffic goes through Cloudflare's network (free, WireGuard encryption). Websites see a Cloudflare IP instead of yours. Some sites block WARP IPs or ask for a captcha; speed depends on the time of day.",
    "<b>Server riêng</b> — dùng VPS hoặc dịch vụ VPN bạn có (link chia sẻ hoặc subscription). Vượt được mọi kiểu chặn, chọn được quốc gia; tốc độ và độ ổn định tuỳ server.":
        "<b>Own server</b> — use a VPS or a VPN service you have (share links or a subscription). Gets past every kind of blocking and lets you pick the country; speed and stability depend on the server.",
    "Cloudflare WARP":
        "Cloudflare WARP",
    "Miễn phí, đổi IP, ẩn IP với website.":
        "Free, changes your IP, hides it from websites.",
    "Server riêng":
        "Own server",
    "VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC hoặc subscription.":
        "VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC or a subscription.",
    "Tài khoản WARP":
        "WARP account",
    "<b>Đăng ký</b> tạo một tài khoản WARP miễn phí, ẩn danh (không cần email). Khoá được lưu mã hoá trên máy này. <b>Đăng ký lại</b> tạo tài khoản mới (IP ảo mới) và bỏ tài khoản cũ.":
        "<b>Register</b> creates a free, anonymous WARP account (no email needed). The keys are stored encrypted on this computer. <b>Register again</b> creates a new account (new virtual IP) and drops the old one.",
    "<b>Key WARP+</b>: nếu bạn có key WARP+ (mua trong app 1.1.1.1 hoặc được chia sẻ), dán vào để dùng mạng ưu tiên, thường nhanh hơn bản miễn phí.":
        "<b>WARP+ key</b>: if you have a WARP+ key (bought in the 1.1.1.1 app or shared with you), paste it to use the priority network, usually faster than the free one.",
    "Đăng ký WARP":
        "Register WARP",
    "Đã có tài khoản":
        "Account ready",
    "đăng ký lại":
        "register again",
    "Key WARP+ (xxxxxxxx-xxxxxxxx-xxxxxxxx)":
        "WARP+ key (xxxxxxxx-xxxxxxxx-xxxxxxxx)",
    "Áp dụng":
        "Apply",
    "Endpoint":
        "Endpoint",
    "Mặc định":
        "Default",
    "Endpoint là địa chỉ máy chủ Cloudflare (IP:port) mà máy bạn kết nối WireGuard tới. Nhà mạng đôi khi chặn hoặc bóp một số IP / port, khiến WARP không kết nối được hoặc rất chậm.":
        "The endpoint is the Cloudflare server address (IP:port) your computer connects to with WireGuard. ISPs sometimes block or throttle certain IPs / ports, which leaves WARP unable to connect or very slow.",
    "<b>Quét endpoint</b> thử khoảng 250 địa chỉ bằng handshake WireGuard thật (mất vài giây) và liệt kê những cái trả lời nhanh nhất; bấm vào một dòng để dùng nó. Cần ngắt kết nối trước khi quét.":
        "<b>Scan endpoints</b> tries about 250 addresses with real WireGuard handshakes (a few seconds) and lists the fastest to answer; click a row to use it. Disconnect before scanning.",
    "Quét endpoint":
        "Scan endpoints",
    "Server":
        "Servers",
    "ping":
        "ping",
    "Bấm vào một server để dùng nó. <b>Tự động</b>: cứ 3 phút đo độ trễ của mọi server và dùng server nhanh nhất. Đổi server khi đang kết nối có hiệu lực ngay, không cần ngắt.":
        "Click a server to use it. <b>Auto</b>: every 3 minutes, measures the latency of every server and uses the fastest. Switching servers while connected takes effect at once, without disconnecting.",
    "<b>Ping</b>: khi chưa kết nối, đo thời gian mở kết nối TCP tới server (Hysteria2 / TUIC dùng UDP nên không đo được, hiện “—”). Khi đã kết nối, đo thời gian tải một trang thật qua từng server — chính xác hơn.":
        "<b>Ping</b>: while disconnected, measures the time to open a TCP connection to the server (Hysteria2 / TUIC use UDP and cannot be measured, shown as “—”). While connected, measures the time to load a real page through each server — more accurate.",
    "Đổi tên: bấm vào tên server. Xoá: bấm ✕ hai lần.":
        "Rename: click ✎. Remove: click ✕ twice.",
    "Thêm server":
        "Add servers",
    "Dán một hoặc nhiều link chia sẻ (mỗi dòng một link) lấy từ nhà cung cấp hoặc từ panel VPS của bạn (3x-ui, Marzban, Hiddify…). Link trùng sẽ được bỏ qua.":
        "Paste one or more share links (one per line) from your provider or your VPS panel (3x-ui, Marzban, Hiddify…). Duplicate links are skipped.",
    "Thêm":
        "Add",
    "Ngắt kết nối cũ khi đổi server<small>Mọi kết nối chuyển ngay sang server mới</small>":
        "Close existing connections when switching servers<small>Everything moves to the new server at once</small>",
    "<b>Bật</b>: khi đổi server (bằng tay hoặc do chế độ Tự động), các kết nối đang mở — tab đang tải, game, cuộc gọi — bị cắt và tự mở lại qua server mới, nên IP đổi ngay lập tức.":
        "<b>On</b>: when the server changes (by hand or by Auto mode), open connections — a loading tab, a game, a call — are cut and reopen through the new server, so your IP changes immediately.",
    "<b>Tắt</b>: kết nối cũ chạy tiếp trên server cũ cho tới khi tự đóng, chỉ kết nối mới dùng server mới. Không làm gián đoạn cuộc gọi hay file đang tải.":
        "<b>Off</b>: old connections keep running on the old server until they close by themselves; only new connections use the new server. Calls and downloads are not interrupted.",
    "Thay đổi mục này cần kết nối lại.":
        "Changing this needs a reconnect.",
    "Subscription":
        "Subscriptions",
    "cập nhật tất cả":
        "update all",
    "Subscription là một đường link (https://…) mà nhà cung cấp VPN đưa cho bạn. Link này chứa danh sách server và được nhà cung cấp cập nhật khi họ thêm / đổi server.":
        "A subscription is a link (https://…) your VPN provider gives you. It holds a list of servers that the provider updates when they add or change servers.",
    "<b>Cập nhật tất cả</b> tải lại mọi danh sách. Server bạn đang chọn được giữ nguyên nếu vẫn còn trong danh sách mới. Xoá một subscription sẽ xoá luôn các server của nó.":
        "<b>Update all</b> downloads every list again. The server you selected is kept if it is still in the new list. Removing a subscription also removes its servers.",
    "Thêm subscription":
        "Add a subscription",
    "https://…":
        "https://…",
    "Tên (tuỳ chọn)":
        "Name (optional)",
    "Quản lý server riêng trên VPS của bạn: cài đặt, thêm / xoá người dùng, lấy link. App điều khiển VPS qua SSH, không mở thêm cổng nào trên VPS.":
        "Manage your own server on your VPS: install it, add / remove users, get links. The app drives the VPS over SSH and opens no extra ports on it.",
    "VPS của bạn":
        "Your VPS",
    "<b>Quản lý và người dùng khác nhau ở chỗ nào?</b> Ai đăng nhập SSH được vào VPS (root hoặc user có sudo) thì là người quản lý. Người dùng chỉ có link <code>vless://</code>: link đó chỉ kết nối được VPN, không mở được SSH, nên không thể thêm / xoá ai hay xem log.":
        "<b>How do admins and users differ?</b> Whoever can log in to the VPS over SSH (root or a sudo user) is the admin. Users only hold a <code>vless://</code> link: it connects to the VPN and nothing else, it cannot open SSH, so they cannot add / remove anyone or read the logs.",
    "Thông tin đăng nhập SSH được mã hoá và chỉ lưu trên máy này. Đừng đưa mật khẩu SSH cho người dùng: gửi họ link của riêng họ là đủ.":
        "SSH login details are encrypted and stored only on this computer. Do not give users the SSH password: sending each of them their own link is enough.",
    "Lần đầu kết nối, app ghi nhớ khoá SSH của VPS (fingerprint). Nếu về sau khoá đổi, app sẽ từ chối kết nối để chống giả mạo.":
        "On first contact the app remembers the VPS's SSH key (fingerprint). If the key changes later, the app refuses to connect, to guard against impersonation.",
    "<b>Lưu lượng</b> (↓ tải xuống, ↑ tải lên) là số byte VPS trao đổi với Internet cho từng người, tính cả header gói tin nên hơi lớn hơn dung lượng file. Số liệu được giữ khi khởi động lại server; bấm “đặt lại” để đếm lại từ 0. <b>Online</b>: người đó đang có kết nối mở, kèm số địa chỉ IP. Một link dùng từ nhiều IP cùng lúc thường là đã bị chia sẻ: xoá người đó rồi tạo link mới.":
        "<b>Traffic</b> (↓ download, ↑ upload) is the number of bytes the VPS exchanged with the Internet for each user, packet headers included, so it is slightly more than file sizes. The numbers survive a server restart; click “reset” to count from 0 again. <b>Online</b>: that user has open connections right now, with the number of IP addresses. One link used from several IPs at once has usually been shared: remove that user and create a new link.",
    "Thêm VPS":
        "Add a VPS",
    "Nhập thông tin nhà cung cấp VPS gửi cho bạn. App sẽ thử đăng nhập trước khi lưu.":
        "Enter the details your VPS provider sent you. The app tries to log in before saving.",
    "Tên gợi nhớ":
        "Name",
    "vd. Singapore (tuỳ chọn)":
        "e.g. Singapore (optional)",
    "IP hoặc tên miền":
        "IP or domain",
    "Cổng SSH":
        "SSH port",
    "User":
        "User",
    "Đăng nhập bằng":
        "Log in with",
    "Mật khẩu":
        "Password",
    "File khoá SSH":
        "SSH key file",
    "File khoá (private key)":
        "Key file (private key)",
    "C:\\Users\\…\\.ssh\\id_ed25519":
        "C:\\Users\\…\\.ssh\\id_ed25519",
    "Chọn…":
        "Browse…",
    "huỷ":
        "cancel",
    "Kiểm tra &amp; lưu":
        "Check &amp; save",
    "làm mới":
        "refresh",
    "Quyết định traffic nào đi qua VPN và traffic nào đi thẳng ra mạng nhà mạng. Đi thẳng thì nhanh hơn và không tốn băng thông server, nhưng nhà mạng thấy được bạn truy cập đâu và có thể chặn.":
        "Decide which traffic goes through the VPN and which goes straight out through your ISP. Direct is faster and uses no server bandwidth, but your ISP can see where you go and may block it.",
    "Web Việt Nam đi thẳng (không qua VPN)<small>Tên miền .vn và mọi IP thuộc Việt Nam</small>":
        "Vietnamese sites go direct (not through the VPN)<small>.vn domains and every Vietnamese IP</small>",
    "Truy cập tới website Việt Nam đi thẳng, không vòng qua server nước ngoài: nhanh hơn, web ngân hàng / dịch vụ công không báo “truy cập từ nước ngoài”, và tiết kiệm băng thông server.":
        "Traffic to Vietnamese websites goes direct instead of round a foreign server: faster, banking / government sites do not report “access from abroad”, and it saves server bandwidth.",
    "Nhận biết bằng tên miền đuôi <code>.vn</code> và danh sách dải IP của Việt Nam. Trang Việt Nam dùng tên miền quốc tế (vd. <code>zalo.me</code>) vẫn được nhận ra nếu máy chủ đặt ở Việt Nam.":
        "Recognised by the <code>.vn</code> domain suffix and the list of Vietnamese IP ranges. A Vietnamese site on an international domain (e.g. <code>zalo.me</code>) is still recognised if its servers are in Vietnam.",
    "Tên miền đi thẳng <small>(mỗi dòng một tên miền)</small>":
        "Direct domains <small>(one domain per line)</small>",
    "Các tên miền này và mọi tên miền con của chúng đi thẳng, không qua VPN. Ví dụ <code>zalo.me</code> áp dụng cho cả <code>chat.zalo.me</code>. Có thể dán cả URL, app tự lấy phần tên miền.":
        "These domains and all their subdomains go direct, not through the VPN. For example <code>zalo.me</code> also covers <code>chat.zalo.me</code>. You can paste whole URLs; the app keeps the domain part.",
    "Tên miền luôn qua VPN <small>(ưu tiên hơn “Web Việt Nam”)</small>":
        "Domains always through the VPN <small>(takes priority over “Vietnamese sites”)</small>",
    "Các tên miền này luôn đi qua VPN, kể cả khi chúng có đuôi .vn, có IP Việt Nam, hay nằm trong danh sách đi thẳng. Dùng cho trang bị nhà mạng chặn nhưng lại có máy chủ ở Việt Nam.":
        "These domains always go through the VPN, even with a .vn suffix, a Vietnamese IP, or an entry in the direct list. Use it for sites your ISP blocks although their servers are in Vietnam.",
    "Ứng dụng đi thẳng <small><span class=\"os-win\">(tên file .exe)</span><span class=\"os-linux\">(tên tiến trình)</span></small>":
        "Direct apps <small><span class=\"os-win\">(.exe file name)</span><span class=\"os-linux\">(process name)</span></small>",
    "Mọi traffic của các chương trình này đi thẳng, bất kể tên miền. Hợp với game online (ping thấp hơn), Steam hay torrent (không tốn băng thông server).":
        "All traffic from these programs goes direct, whatever the domain. Good for online games (lower ping), Steam or torrents (no server bandwidth used).",
    "Nhập tên file, mỗi dòng một tên, ví dụ <code>steam.exe</code>. Xem tên file trong Task Manager → tab Details.":
        "Enter file names, one per line, e.g. <code>steam.exe</code>. Find the name in Task Manager → Details tab.",
    "Nhập tên tiến trình, mỗi dòng một tên, ví dụ <code>steam</code>. Xem tên bằng System Monitor hoặc lệnh <code>ps -e</code>.":
        "Enter process names, one per line, e.g. <code>steam</code>. Find the name in System Monitor or with <code>ps -e</code>.",
    "DNS &amp; mạng":
        "DNS &amp; network",
    "DoH server <small>(DNS mã hoá)</small>":
        "DoH server <small>(encrypted DNS)</small>",
    "DNS là bước đổi tên trang (vd. <code>facebook.com</code>) thành địa chỉ IP. Nhà mạng chặn web chủ yếu bằng cách trả sai ở bước này. App gửi mọi truy vấn DNS dưới dạng mã hoá qua HTTPS (DoH) tới server dưới đây, nên nhà mạng không đọc hay sửa được. Mục này luôn bật ở mọi chế độ.":
        "DNS turns a site name (e.g. <code>facebook.com</code>) into an IP address. ISPs mostly block sites by answering wrongly at this step. The app sends every DNS query encrypted over HTTPS (DoH) to the server below, so your ISP cannot read or change it. This is always on, in every mode.",
    "Gợi ý: <code>https://1.1.1.1/dns-query</code> (Cloudflare), <code>https://8.8.8.8/dns-query</code> (Google), <code>https://9.9.9.9/dns-query</code> (Quad9). Nên dùng địa chỉ IP thay vì tên miền để tránh bị chặn ngay từ bước tìm server DNS.":
        "Suggestions: <code>https://1.1.1.1/dns-query</code> (Cloudflare), <code>https://8.8.8.8/dns-query</code> (Google), <code>https://9.9.9.9/dns-query</code> (Quad9). Prefer an IP address over a domain, so the lookup of the DNS server itself cannot be blocked.",
    "https://1.1.1.1/dns-query":
        "https://1.1.1.1/dns-query",
    "DoH server":
        "DoH server",
    "Không đi qua VPN cho mạng LAN<small>Máy in, NAS, router… trong mạng nhà vẫn dùng được</small>":
        "Keep LAN traffic out of the VPN<small>Printers, NAS, routers… on your home network keep working</small>",
    "Kết nối tới thiết bị trong mạng nội bộ (địa chỉ 192.168.x.x, 10.x.x.x, 172.16–31.x.x) như máy in, NAS, router, chia sẻ file, Chromecast đi thẳng, không vào VPN.":
        "Connections to devices on your local network (addresses 192.168.x.x, 10.x.x.x, 172.16–31.x.x) such as printers, NAS, routers, file shares and Chromecast go direct, not into the VPN.",
    "Tắt mục này thì các thiết bị đó có thể không truy cập được khi bật VPN. Khi kill switch đang chặn, mạng LAN vẫn dùng được nếu mục này bật.":
        "With this off, those devices may be unreachable while the VPN is on. While the kill switch is blocking, the LAN still works if this is on.",
    "Cách ứng dụng phản ứng khi rớt mạng, khi bạn đóng cửa sổ và khi khởi động máy. Các mục ở đây có hiệu lực ngay, không cần kết nối lại.":
        "How the app reacts when the connection drops, when you close the window and when the computer starts. These take effect at once, no reconnect needed.",
    "Bảo vệ":
        "Protection",
    "Kill switch — chặn mạng khi VPN rớt<small>Không để lộ IP thật. Áp dụng cho WARP và Server riêng</small>":
        "Kill switch — block the network when the VPN drops<small>Never exposes your real IP. Applies to WARP and Own server</small>",
    "Bình thường khi VPN dừng bất ngờ, máy tự quay về mạng thường và mọi trang bạn đang mở sẽ thấy IP thật. Với kill switch, app giữ card mạng ảo và <b>chặn mọi kết nối Internet</b> thay vào đó.":
        "Normally, when the VPN stops unexpectedly, your computer falls back to the regular network and every site you have open sees your real IP. With the kill switch, the app keeps the virtual network adapter and <b>blocks all Internet connections</b> instead.",
    "Mạng bị chặn cho đến khi kết nối lại thành công hoặc bạn bấm <b>Ngắt kết nối</b>. Mạng LAN vẫn dùng được nếu bật mục LAN ở tab Định tuyến.":
        "The network stays blocked until a reconnect succeeds or you click <b>Disconnect</b>. The LAN keeps working if the LAN option in the Routing tab is on.",
    "Tự kết nối lại khi bị rớt<small>Thử lại 5 lần trong khoảng 1 phút</small>":
        "Reconnect automatically after a drop<small>Retries 5 times over about a minute</small>",
    "Khi lõi VPN dừng bất ngờ (server sập, mạng chập chờn, máy thức dậy sau khi ngủ), app tự khởi động lại tối đa 5 lần, cách nhau 2, 5, 10, 20 và 30 giây.":
        "When the VPN core stops unexpectedly (server down, flaky network, computer waking from sleep), the app restarts it up to 5 times, 2, 5, 10, 20 and 30 seconds apart.",
    "Nếu vẫn không được: có kill switch thì mạng tiếp tục bị chặn chờ bạn xử lý; không có kill switch thì app ngắt kết nối và báo lỗi.":
        "If that still fails: with the kill switch, the network stays blocked until you act; without it, the app disconnects and reports the error.",
    "Thông báo hệ thống<small>Báo khi VPN rớt, đã kết nối lại hoặc không kết nối được</small>":
        "System notifications<small>When the VPN drops, reconnects or cannot connect</small>",
    "Hiện thông báo ở góc màn hình khi: VPN bị rớt, kill switch bắt đầu chặn mạng, đã tự kết nối lại được, hoặc không tự kết nối được lúc mở app. Hữu ích khi app đang ẩn dưới khay.":
        "Shows a notification in the corner of the screen when: the VPN drops, the kill switch starts blocking, the app reconnected by itself, or auto-connect failed at launch. Useful while the app is hidden in the tray.",
    "Khởi động &amp; cửa sổ":
        "Startup &amp; window",
    "Tự kết nối khi mở ứng dụng<small>Dùng chế độ và server đã chọn lần trước</small>":
        "Connect when the app opens<small>Uses the mode and server from last time</small>",
    "Mở app là kết nối ngay, không cần bấm nút. Kết hợp với “Khởi động cùng <span class=\"os-win\">Windows</span><span class=\"os-linux\">máy</span>” để máy luôn có VPN từ lúc đăng nhập. Nếu kết nối thất bại, app báo lỗi (và hiện thông báo nếu bật).":
        "Opening the app connects right away, no button to press. Combine with “Start with <span class=\"os-win\">Windows</span><span class=\"os-linux\">the computer</span>” so the VPN is on from the moment you log in. If the connection fails, the app reports the error (and shows a notification if enabled).",
    "Khởi động cùng <span class=\"os-win\">Windows</span><span class=\"os-linux\">máy</span><small>Tự chạy khi đăng nhập<span class=\"os-win\">, không hỏi quyền Administrator</span><span class=\"os-linux\"></span></small>":
        "Start with <span class=\"os-win\">Windows</span><span class=\"os-linux\">the computer</span><small>Runs when you log in<span class=\"os-win\">, without the Administrator prompt</span><span class=\"os-linux\"></span></small>",
    "App tạo một tác vụ “Black Cyclone” trong Task Scheduler để chạy khi bạn đăng nhập, với quyền Administrator mà không hiện hộp thoại UAC. App mở khoảng 10 giây sau khi đăng nhập để Windows kịp khởi động xong.":
        "The app creates a “Black Cyclone” task in Task Scheduler that runs when you log in, with Administrator rights and no UAC prompt. The app opens about 10 seconds after login, to let Windows finish starting.",
    "Tắt mục này sẽ xoá tác vụ. Gỡ cài đặt app cũng tự xoá tác vụ.":
        "Turning this off removes the task. Uninstalling the app removes it too.",
    "App tạo file <code>~/.config/autostart/blackcyclone.desktop</code> để môi trường desktop (GNOME, KDE, XFCE…) chạy app khi bạn đăng nhập, khoảng 10 giây sau để thanh tác vụ kịp hiện. Không cần nhập mật khẩu vì quyền mạng đã được cấp sẵn cho file app.":
        "The app creates <code>~/.config/autostart/blackcyclone.desktop</code> so your desktop environment (GNOME, KDE, XFCE…) runs the app when you log in, about 10 seconds later so the panel is ready. No password is needed because the app binary already holds the network permissions.",
    "Tắt mục này sẽ xoá file đó.":
        "Turning this off removes that file.",
    "Ẩn xuống khay khi khởi động cùng <span class=\"os-win\">Windows</span><span class=\"os-linux\">máy</span><small>Chỉ hiện icon, không mở cửa sổ</small>":
        "Start hidden in the tray when starting with <span class=\"os-win\">Windows</span><span class=\"os-linux\">the computer</span><small>Only the icon, no window</small>",
    "Khi app tự chạy cùng Windows, nó chỉ hiện icon ở khay hệ thống (góc phải thanh taskbar, có thể nằm trong mũi tên ^) mà không mở cửa sổ. Bấm vào icon để mở. Không ảnh hưởng khi bạn tự mở app.":
        "When the app starts with Windows, it only shows its icon in the system tray (right end of the taskbar, possibly behind the ^ arrow) without opening a window. Click the icon to open it. No effect when you open the app yourself.",
    "Khi app tự chạy lúc đăng nhập, nó chỉ hiện icon ở khay hệ thống mà không mở cửa sổ. Bấm vào icon để mở. Không ảnh hưởng khi bạn tự mở app. Trên GNOME cần cài extension “AppIndicator and KStatusNotifierItem Support” thì mới thấy icon khay; nếu không có khay, đừng bật mục này.":
        "When the app starts at login, it only shows its icon in the system tray without opening a window. Click the icon to open it. No effect when you open the app yourself. GNOME needs the “AppIndicator and KStatusNotifierItem Support” extension to show tray icons; without a tray, leave this off.",
    "Bấm X thì thu nhỏ xuống khay<small>VPN vẫn chạy khi đóng cửa sổ</small>":
        "Closing the window minimises to the tray<small>The VPN keeps running when the window is closed</small>",
    "<b>Bật</b>: nút X chỉ ẩn cửa sổ, VPN vẫn chạy; mở lại bằng icon ở khay. Muốn tắt hẳn thì dùng nút <b>Thoát hẳn</b> bên dưới hoặc chuột phải icon → Thoát.":
        "<b>On</b>: the X button only hides the window, the VPN keeps running; reopen it from the tray icon. To really quit, use the <b>Quit</b> button below or right-click the icon → Quit.",
    "<b>Tắt</b>: nút X ngắt VPN và thoát app.":
        "<b>Off</b>: the X button disconnects the VPN and quits the app.",
    "Chứa nhật ký (app.log) và cài đặt (đã mã hoá)":
        "Holds the log (app.log) and the encrypted settings",
    "Mở thư mục dữ liệu":
        "Open data folder",
    "Ngắt VPN và đóng app hoàn toàn":
        "Disconnects the VPN and closes the app completely",
    "Thoát hẳn":
        "Quit",
    "Kiểm tra VPN có hoạt động đúng không, và xem nhật ký khi gặp lỗi.":
        "Check that the VPN works properly, and read the log when something fails.",
    "Kiểm tra kết nối":
        "Check connection",
    "Hiện IP công khai mà website nhìn thấy, quốc gia, trung tâm dữ liệu Cloudflare trả lời và trạng thái WARP. Bật VPN mà IP / quốc gia đổi nghĩa là VPN đang hoạt động.":
        "Shows the public IP that websites see, the country, the Cloudflare data centre that answered and the WARP state. If the IP / country changes when the VPN is on, the VPN is working.",
    "Sau đó thử mở các trang hay bị chặn hoặc bóp ở Việt Nam. ✗ kèm “bị ngắt kết nối”, “chứng chỉ sai” hoặc “không phân giải được tên miền” thường là bị nhà mạng chặn. Chạy thử cả khi tắt và bật VPN để so sánh.":
        "It then tries to open sites that are often blocked or throttled in Vietnam. ✗ with “connection reset”, “wrong certificate” or “could not resolve the domain” usually means your ISP is blocking it. Run it with the VPN off and on to compare.",
    "Nhật ký":
        "Log",
    "xoá":
        "clear",
    "Hoạt động của app và sing-box. Khi báo lỗi, gửi kèm file <code>app.log</code> trong thư mục dữ liệu.":
        "What the app and sing-box are doing. When reporting a problem, attach the <code>app.log</code> file from the data folder.",
    "Black Cyclone":
        "Black Cyclone",
    "Ngôn ngữ":
        "Language",
    "Ngôn ngữ giao diện":
        "Interface language",
    "Tự động (theo hệ điều hành)":
        "Auto (follow the system)",
    "English":
        "English",
    "Tiếng Việt":
        "Tiếng Việt",
    "Tên miền đi thẳng":
        "Direct domains",
    "Tên miền luôn qua VPN":
        "Domains always through the VPN",
    "Ứng dụng đi thẳng":
        "Direct apps",
};
