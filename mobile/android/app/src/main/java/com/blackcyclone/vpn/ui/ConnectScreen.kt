package com.blackcyclone.vpn.ui

import android.content.Intent
import android.provider.Settings
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.RadioButton
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.blackcyclone.vpn.Account
import com.blackcyclone.vpn.MainViewModel
import com.blackcyclone.vpn.Mode
import com.blackcyclone.vpn.ServerEntry
import com.blackcyclone.vpn.ScanState
import com.blackcyclone.vpn.Status
import com.blackcyclone.vpn.TunnelInfo
import com.blackcyclone.vpn.TunnelState
import com.blackcyclone.vpn.formatBytes
import com.blackcyclone.vpn.formatRate
import com.blackcyclone.vpn.parseEndpoint
import com.blackcyclone.vpn.tr

@Composable
fun ConnectScreen(vm: MainViewModel, onConnect: () -> Unit, onDisconnect: () -> Unit) {
    val st by TunnelState.state.collectAsStateWithLifecycle()
    val account by vm.account.collectAsStateWithLifecycle()
    val busy by vm.busy.collectAsStateWithLifecycle()
    val scan by vm.scan.collectAsStateWithLifecycle()
    val mode by vm.mode.collectAsStateWithLifecycle()
    val servers by vm.servers.collectAsStateWithLifecycle()
    val selected by vm.selectedServer.collectAsStateWithLifecycle()
    val canConnect = if (mode == Mode.WARP) account != null else servers.isNotEmpty()

    Column(
        Modifier.verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        StatusPanel(st, enabled = canConnect, onConnect, onDisconnect)
        if (st.active && st.lockdown == false) LockdownWarning()
        ModeSelector(mode, vm::setMode)
        if (mode == Mode.WARP) {
            AccountCard(vm, account, busy)
            if (account != null) EndpointCard(vm, account!!, scan)
        } else {
            ServersCard(vm, servers, selected)
        }
    }
}

@Composable
private fun ModeSelector(mode: Mode, onChange: (Mode) -> Unit) {
    SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
        Mode.entries.forEachIndexed { i, m ->
            SegmentedButton(
                selected = mode == m,
                onClick = { onChange(m) },
                shape = SegmentedButtonDefaults.itemShape(i, Mode.entries.size),
            ) { Text(m.label) }
        }
    }
}

@Composable
private fun ServersCard(vm: MainViewModel, servers: List<ServerEntry>, selected: String) {
    val ctx = LocalContext.current
    var adding by remember { mutableStateOf(false) }
    var menuFor by remember { mutableStateOf<ServerEntry?>(null) }
    var renaming by remember { mutableStateOf<ServerEntry?>(null) }

    SectionCard(tr("Server riêng", "Own server")) {
        if (servers.isEmpty()) {
            Hint(
                tr("Chưa có server. Dán link vless:// (hoặc vmess, trojan, ss, hysteria2, tuic) lấy từ VPS của bạn: " +
                    "trên VPS cài bằng thư mục server/ của Black Cyclone, lệnh bcserver link in ra link này.",
                    "No servers yet. Paste a vless:// link (or vmess, trojan, ss, hysteria2, tuic) from your VPS: " +
                    "on a VPS set up with Black Cyclone's server/ folder, the bcserver link command prints it."),
                Modifier.padding(vertical = 6.dp),
            )
        } else {
            Hint(tr("Chạm để chọn, giữ để đổi tên / xoá.", "Tap to select, long-press to rename / delete."), Modifier.padding(top = 4.dp, bottom = 4.dp))
            if (servers.size > 1) {
                ServerRow(tr("Tự động", "Auto"), tr("chọn server có độ trễ thấp nhất", "picks the server with the lowest latency"), selected == ServerEntry.AUTO, onClick = { vm.selectServer(ServerEntry.AUTO) })
            }
            servers.forEach { srv ->
                ServerRow(
                    srv.name, "${srv.scheme} · ${srv.address}", selected == srv.id || servers.size == 1,
                    onClick = { vm.selectServer(srv.id) },
                    onLongClick = { menuFor = srv },
                )
            }
        }
        Row(Modifier.padding(top = 10.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = { adding = true }) { Text(tr("Thêm server", "Add server")) }
        }
    }

    if (adding) {
        TextInputDialog(
            title = tr("Thêm server", "Add server"),
            label = "Link",
            initial = ctx.pasteText().takeIf { it.contains("://") }.orEmpty(),
            hint = tr("Dán một hoặc nhiều link. Link chứa khoá truy cập server: đừng chia sẻ công khai.", "Paste one or more links. A link contains the server's access key: don't share it publicly."),
            singleLine = false,
            onDismiss = { adding = false },
            onOk = { vm.addServers(it) },
        )
    }
    menuFor?.let { srv ->
        AlertDialog(
            onDismissRequest = { menuFor = null },
            title = { Text(srv.name) },
            text = { Text(srv.address, fontFamily = FontFamily.Monospace) },
            confirmButton = {
                TextButton(onClick = { menuFor = null; renaming = srv }) { Text(tr("Đổi tên", "Rename")) }
            },
            dismissButton = {
                Row {
                    TextButton(onClick = {
                        ctx.copyText("Server link", srv.link)
                        vm.toast(tr("Đã sao chép link", "Link copied"))
                        menuFor = null
                    }) { Text(tr("Sao chép link", "Copy link")) }
                    TextButton(onClick = { menuFor = null; vm.removeServer(srv.id) }) {
                        Text(tr("Xoá", "Delete"), color = MaterialTheme.colorScheme.error)
                    }
                }
            },
        )
    }
    renaming?.let { srv ->
        TextInputDialog(
            title = tr("Đổi tên", "Rename"),
            label = tr("Tên", "Name"),
            initial = srv.name,
            onDismiss = { renaming = null },
            onOk = { vm.renameServer(srv.id, it); it.isNotBlank() },
        )
    }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun ServerRow(title: String, sub: String, selected: Boolean, onClick: () -> Unit, onLongClick: (() -> Unit)? = null) {
    Row(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(8.dp))
            .background(if (selected) BrandRed.copy(alpha = 0.18f) else Color.Transparent)
            .combinedClickable(onClick = onClick, onLongClick = onLongClick)
            .padding(horizontal = 10.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        RadioButton(selected = selected, onClick = null)
        Column(Modifier.padding(start = 8.dp)) {
            Text(title, style = MaterialTheme.typography.bodyLarge)
            Hint(sub)
        }
    }
}

@Composable
private fun StatusPanel(st: TunnelInfo, enabled: Boolean, onConnect: () -> Unit, onDisconnect: () -> Unit) {
    val (label, color) = when {
        st.status == Status.CONNECTED -> tr("Đã kết nối", "Connected") to OkGreen
        st.blocking -> tr("Đang chặn mạng (kill switch)", "Blocking the network (kill switch)") to MaterialTheme.colorScheme.error
        st.status == Status.CONNECTING -> tr("Đang kết nối…", "Connecting…") to WarnAmber
        st.status == Status.ERROR -> tr("Lỗi", "Error") to MaterialTheme.colorScheme.error
        else -> tr("Đã ngắt", "Disconnected") to MaterialTheme.colorScheme.onSurfaceVariant
    }
    val pulse = rememberInfiniteTransition(label = "pulse").animateFloat(
        initialValue = 0.45f, targetValue = 1f,
        animationSpec = infiniteRepeatable(tween(900), RepeatMode.Reverse), label = "alpha",
    )
    Column(Modifier.fillMaxWidth().padding(top = 8.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        val ringColor = if (st.status == Status.CONNECTED) OkGreen else if (st.active) WarnAmber else MaterialTheme.colorScheme.outline
        Box(
            Modifier
                .size(176.dp)
                .alpha(if (st.status == Status.CONNECTING) pulse.value else 1f)
                .border(BorderStroke(4.dp, ringColor), CircleShape)
                .padding(10.dp)
                .clip(CircleShape)
                .background(if (st.active) BrandRed else MaterialTheme.colorScheme.surfaceContainerHighest)
                .clickable(enabled = enabled || st.active) { if (st.active) onDisconnect() else onConnect() },
            contentAlignment = Alignment.Center,
        ) {
            Text(
                if (st.active) tr("NGẮT", "STOP") else tr("KẾT NỐI", "CONNECT"),
                color = if (enabled || st.active) Color.White else MaterialTheme.colorScheme.onSurfaceVariant,
                fontSize = 22.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.5.sp,
            )
        }
        Spacer(Modifier.height(14.dp))
        Text(label, color = color, style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)
        st.endpoint?.takeIf { st.active }?.let { Hint(tr("qua $it", "via $it")) }
        st.message?.let {
            Text(
                it, modifier = Modifier.padding(top = 6.dp),
                color = if (st.status == Status.ERROR) MaterialTheme.colorScheme.error else WarnAmber,
                style = MaterialTheme.typography.bodySmall,
            )
        }
        if (st.active) {
            Spacer(Modifier.height(12.dp))
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceEvenly) {
                Stat(tr("↓ Tải xuống", "↓ Download"), formatRate(st.rxRate), formatBytes(st.rx))
                Stat(tr("↑ Tải lên", "↑ Upload"), formatRate(st.txRate), formatBytes(st.tx))
                Stat(
                    tr("Độ trễ", "Latency"),
                    if (st.okAgoMs < 0) "—" else "${st.delayMs} ms",
                    if (st.okAgoMs < 0) tr("chưa thông", "no traffic yet") else tr("đo ${st.okAgoMs / 1000}s trước", "measured ${st.okAgoMs / 1000}s ago"),
                )
            }
        }
    }
}

@Composable
private fun Stat(title: String, value: String, sub: String) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Hint(title)
        Text(value, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
        Hint(sub)
    }
}

@Composable
private fun AccountCard(vm: MainViewModel, account: Account?, busy: String?) {
    val ctx = LocalContext.current
    var dialog by remember { mutableStateOf<String?>(null) }

    SectionCard(tr("Tài khoản WARP", "WARP account")) {
        if (account == null) {
            Hint(tr("Chưa có tài khoản. Đăng ký WARP miễn phí ngay trong app (không cần email), hoặc nhập tài khoản đã xuất từ app khác.", "No account yet. Register a free WARP account right in the app (no email needed), or import one exported from another app."), Modifier.padding(vertical = 6.dp))
        } else {
            Row(Modifier.padding(top = 6.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(account.typeLabel, color = if (account.typeLabel == "WARP+") OkGreen else MaterialTheme.colorScheme.onSurface, fontWeight = FontWeight.SemiBold)
                account.addresses.firstOrNull()?.let { Hint("  ·  $it") }
            }
            if (account.license.isNotEmpty()) Hint("Key: " + account.license.take(8) + "…")
        }
        if (busy != null) {
            Row(Modifier.padding(top = 10.dp), verticalAlignment = Alignment.CenterVertically) {
                CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                Spacer(Modifier.width(10.dp))
                Text(busy, style = MaterialTheme.typography.bodyMedium)
            }
        }
        Row(Modifier.padding(top = 10.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            if (account == null) {
                Button(onClick = vm::register, enabled = busy == null) { Text(tr("Đăng ký WARP", "Register WARP")) }
            } else {
                Button(onClick = { dialog = "license" }, enabled = busy == null) { Text(tr("Key WARP+", "WARP+ key")) }
                OutlinedButton(onClick = { dialog = "rereg" }, enabled = busy == null) { Text(tr("Đăng ký lại", "Re-register")) }
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            if (account != null) {
                TextButton(onClick = {
                    ctx.copyText("WARP account", account.json)
                    vm.toast(tr("Đã sao chép tài khoản (chứa khoá bí mật, đừng chia sẻ)", "Account copied (it contains secret keys, don't share it)"))
                }) { Text(tr("Xuất", "Export")) }
            }
            TextButton(onClick = { dialog = "import" }, enabled = busy == null) { Text(tr("Nhập", "Import")) }
        }
    }

    when (dialog) {
        "license" -> TextInputDialog(
            title = tr("Key WARP+", "WARP+ key"),
            label = "xxxxxxxx-xxxxxxxx-xxxxxxxx",
            hint = tr("Gắn key WARP+ vào thiết bị này. Mỗi key dùng được trên tối đa 5 thiết bị.", "Bind a WARP+ key to this device. Each key works on up to 5 devices."),
            onDismiss = { dialog = null },
            onOk = { if (it.isNotBlank()) vm.applyLicense(it); it.isNotBlank() },
        )
        "import" -> TextInputDialog(
            title = tr("Nhập tài khoản", "Import account"),
            label = tr("JSON tài khoản", "Account JSON"),
            initial = ctx.pasteText().takeIf { it.trimStart().startsWith("{") }.orEmpty(),
            hint = tr("Dán JSON đã xuất từ BlackCyclone (Android) hoặc Black Cyclone bản máy tính. Tài khoản hiện tại sẽ bị thay thế.", "Paste the JSON exported from BlackCyclone (Android) or Black Cyclone for desktop. It replaces the current account."),
            singleLine = false,
            onDismiss = { dialog = null },
            onOk = { vm.import(it) },
        )
        "rereg" -> AlertDialog(
            onDismissRequest = { dialog = null },
            title = { Text(tr("Đăng ký lại?", "Re-register?")) },
            text = { Text(tr("Tạo tài khoản WARP mới với khoá mới. Tài khoản hiện tại (và key WARP+ nếu có) sẽ bị bỏ, hãy Xuất trước nếu cần giữ.", "Creates a new WARP account with new keys. The current account (and its WARP+ key, if any) is discarded: Export it first if you want to keep it.")) },
            confirmButton = { TextButton(onClick = { dialog = null; vm.register() }) { Text(tr("Đăng ký lại", "Re-register")) } },
            dismissButton = { TextButton(onClick = { dialog = null }) { Text(tr("Huỷ", "Cancel")) } },
        )
    }
}

@Composable
private fun EndpointCard(vm: MainViewModel, account: Account, scan: ScanState) {
    var editing by remember { mutableStateOf(false) }

    SectionCard("Endpoint") {
        Row(Modifier.fillMaxWidth().padding(top = 6.dp), verticalAlignment = Alignment.CenterVertically) {
            Text(account.endpointText, fontFamily = FontFamily.Monospace, modifier = Modifier.weight(1f))
            TextButton(onClick = { editing = true }) { Text(tr("Sửa", "Edit")) }
        }
        Hint(tr("Nếu kết nối được mà không vào mạng, hoặc mãi không bắt tay, hãy quét để tìm IP:port còn sống và nhanh nhất trên mạng của bạn.", "If it connects but nothing loads, or the handshake never completes, scan to find the fastest working IP:port on your network."))
        Spacer(Modifier.height(10.dp))
        if (scan.running) {
            LinearProgressIndicator(progress = { scan.progress }, modifier = Modifier.fillMaxWidth())
            Row(Modifier.fillMaxWidth().padding(top = 6.dp), verticalAlignment = Alignment.CenterVertically) {
                Hint(tr("Đang quét bằng handshake WireGuard thật…", "Scanning with real WireGuard handshakes…"), Modifier.weight(1f))
                TextButton(onClick = vm::cancelScan) { Text(tr("Dừng", "Stop")) }
            }
        } else {
            Button(onClick = vm::startScan) { Text(tr("Quét endpoint", "Scan endpoints")) }
        }
        if (scan.done && !scan.running) {
            Hint(tr("Đã quét ${scan.scanned}, ${scan.hits.size} phản hồi. Chạm để chọn:", "Scanned ${scan.scanned}, ${scan.hits.size} responded. Tap to select:"), Modifier.padding(top = 10.dp, bottom = 4.dp))
            scan.hits.take(10).forEach { h ->
                val selected = h.ip == account.endpoint && h.port == account.port
                Row(
                    Modifier
                        .fillMaxWidth()
                        .clip(RoundedCornerShape(8.dp))
                        .background(if (selected) BrandRed.copy(alpha = 0.18f) else Color.Transparent)
                        .clickable { vm.setEndpoint(h.ip, h.port) }
                        .padding(horizontal = 10.dp, vertical = 8.dp),
                ) {
                    Text(h.text, fontFamily = FontFamily.Monospace, modifier = Modifier.weight(1f))
                    Text("${h.rttMs} ms", color = if (h.rttMs < 150) OkGreen else WarnAmber)
                }
            }
        }
    }

    if (editing) {
        TextInputDialog(
            title = "Endpoint",
            label = "IP:port",
            initial = account.endpointText,
            hint = tr("Ví dụ 162.159.192.1:2408 hoặc engage.cloudflareclient.com:2408", "For example 162.159.192.1:2408 or engage.cloudflareclient.com:2408"),
            onDismiss = { editing = false },
            onOk = { text ->
                val ep = parseEndpoint(text)
                if (ep == null) vm.toast(tr("Endpoint không hợp lệ", "Invalid endpoint")) else vm.setEndpoint(ep.first, ep.second)
                ep != null
            },
        )
    }
}

/** Shown while connected if Android's "Block connections without VPN" is off (Android 10+). */
@Composable
private fun LockdownWarning() {
    val ctx = LocalContext.current
    SectionCard(tr("Chưa chặn hoàn toàn", "Not fully blocked")) {
        Text(
            tr("\"Chặn kết nối không qua VPN\" của Android đang tắt: nếu app bị hệ thống tắt hẳn hoặc lúc khởi động máy, " +
                "ứng dụng có thể đi thẳng và lộ IP thật. Bật \"VPN luôn bật\" và \"Chặn kết nối không qua VPN\" cho BlackCyclone.",
                "Android's \"Block connections without VPN\" is off: if the system kills the app, or while the device boots, " +
                "apps can go direct and expose your real IP. Turn on \"Always-on VPN\" and \"Block connections without VPN\" for BlackCyclone."),
            color = WarnAmber,
            style = MaterialTheme.typography.bodySmall,
            modifier = Modifier.padding(top = 4.dp, bottom = 8.dp),
        )
        OutlinedButton(onClick = {
            ctx.startActivity(Intent(Settings.ACTION_VPN_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
        }) { Text(tr("Mở cài đặt VPN", "Open VPN settings")) }
    }
}
