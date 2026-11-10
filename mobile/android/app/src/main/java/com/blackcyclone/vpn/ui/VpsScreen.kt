package com.blackcyclone.vpn.ui

import android.graphics.Bitmap
import android.net.Uri
import android.provider.OpenableColumns
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.RadioButton
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.FilterQuality
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.blackcyclone.vpn.MainViewModel
import com.blackcyclone.vpn.VpsHost
import com.blackcyclone.vpn.VpsOverview
import com.blackcyclone.vpn.VpsState
import com.blackcyclone.vpn.VpsUser
import com.blackcyclone.vpn.VpsViewModel
import com.blackcyclone.vpn.formatBytes
import com.blackcyclone.vpn.tr
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private const val POLL_MS = 20_000L

/** A link used from this many addresses at once has probably been shared. */
private const val MANY_IPS = 3

private val NAME_RE = Regex("^[A-Za-z0-9._-]{1,32}$")

@Composable
fun VpsScreen(vm: VpsViewModel, main: MainViewModel) {
    val hosts by vm.hosts.collectAsStateWithLifecycle()
    val selected by vm.selected.collectAsStateWithLifecycle()
    val states by vm.states.collectAsStateWithLifecycle()
    val op by vm.op.collectAsStateWithLifecycle()
    val output by vm.output.collectAsStateWithLifecycle()
    val message by vm.message.collectAsStateWithLifecycle()
    val servers by main.servers.collectAsStateWithLifecycle()

    var editing by remember { mutableStateOf<VpsHost?>(null) }
    var adding by remember { mutableStateOf(false) }
    var menuFor by remember { mutableStateOf<VpsHost?>(null) }

    LaunchedEffect(message) {
        message?.let { main.toast(it); vm.messageShown() }
    }
    // Load the selected VPS, then keep "who is online" fresh while the tab is open.
    LaunchedEffect(selected) {
        if (selected != null && states[selected] == null) vm.refresh()
        while (true) {
            delay(POLL_MS)
            vm.poll()
        }
    }

    Column(
        Modifier.verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        SectionCard(tr("VPS của bạn", "Your VPS")) {
            Hint(
                tr("Cài và quản lý server riêng trên VPS qua SSH. Ai có đăng nhập SSH là người quản lý; " +
                    "người dùng chỉ nhận link vless:// của họ nên không quản lý được gì. " +
                    "Thông tin SSH được mã hoá và chỉ lưu trên máy này.",
                    "Install and manage your own server on a VPS over SSH. Whoever has the SSH login is the admin; " +
                    "users only get their vless:// link, so they can't manage anything. " +
                    "SSH details are encrypted and stored only on this device."),
                Modifier.padding(top = 4.dp, bottom = 6.dp),
            )
            if (hosts.isEmpty()) {
                Hint(tr("Chưa có VPS.", "No VPS yet."), Modifier.padding(vertical = 4.dp))
            } else {
                Hint(tr("Chạm để chọn, giữ để sửa / xoá.", "Tap to select, long-press to edit / remove."), Modifier.padding(bottom = 4.dp))
            }
            hosts.forEach { h ->
                HostRow(h, h.id == selected, onClick = { vm.select(h.id) }, onLongClick = { menuFor = h })
            }
            Button(onClick = { adding = true }, enabled = op == null, modifier = Modifier.padding(top = 8.dp)) {
                Text(tr("Thêm VPS", "Add VPS"))
            }
        }

        hosts.firstOrNull { it.id == selected }?.let { h ->
            DetailCard(vm, h, states[h.id], op, output, servers.map { it.link }.toSet(), onImport = { main.addServers(it) })
        }
    }

    if (adding || editing != null) {
        HostDialog(vm, editing, op, onDismiss = { adding = false; editing = null })
    }
    menuFor?.let { h ->
        var confirm by remember { mutableStateOf(false) }
        AlertDialog(
            onDismissRequest = { menuFor = null },
            title = { Text(h.name) },
            text = {
                Text(
                    if (confirm) tr("Xoá ${h.name} khỏi app? Server trên VPS vẫn chạy, link của mọi người vẫn dùng được.", "Remove ${h.name} from the app? The server on the VPS keeps running and everyone's links still work.")
                    else "${h.user}@${h.address}",
                )
            },
            confirmButton = {
                TextButton(onClick = { menuFor = null; editing = h }) { Text(tr("Sửa", "Edit")) }
            },
            dismissButton = {
                TextButton(onClick = {
                    if (!confirm) confirm = true else { vm.remove(h.id); menuFor = null }
                }) { Text(if (confirm) tr("Xoá thật", "Yes, remove") else tr("Xoá khỏi app", "Remove from app"), color = MaterialTheme.colorScheme.error) }
            },
        )
    }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun HostRow(h: VpsHost, selected: Boolean, onClick: () -> Unit, onLongClick: () -> Unit) {
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
            Text(h.name, style = MaterialTheme.typography.bodyLarge)
            Hint("${h.user}@${h.address} · ${if (h.usesKey) tr("khoá SSH", "SSH key") else tr("mật khẩu", "password")}")
        }
    }
}

/** Add (editing == null) or edit a VPS. The login is checked before saving. */
@Composable
private fun HostDialog(vm: VpsViewModel, editing: VpsHost?, op: String?, onDismiss: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var name by remember { mutableStateOf(editing?.json?.optString("name").orEmpty()) }
    var addr by remember { mutableStateOf(editing?.addr.orEmpty()) }
    var port by remember { mutableStateOf((editing?.port ?: 22).toString()) }
    var user by remember { mutableStateOf(editing?.user ?: "root") }
    var useKey by remember { mutableStateOf(editing?.usesKey ?: false) }
    var password by remember { mutableStateOf("") }
    var keyPem by remember { mutableStateOf("") }
    var keyName by remember { mutableStateOf(if (editing?.usesKey == true) tr("(giữ khoá đã lưu)", "(keep the saved key)") else "") }
    var error by remember { mutableStateOf<String?>(null) }

    val pickKey = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri: Uri? ->
        uri ?: return@rememberLauncherForActivityResult
        runCatching {
            val text = ctx.contentResolver.openInputStream(uri)!!.use { s -> s.readBytes().also { require(it.size < 64 * 1024) } }
                .toString(Charsets.UTF_8)
            require(text.contains("PRIVATE KEY")) { tr("không phải file khoá riêng (private key)", "not a private key file") }
            keyPem = text
            keyName = ctx.contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { c ->
                if (c.moveToFirst()) c.getString(0) else null
            } ?: tr("khoá SSH", "SSH key")
            error = null
        }.onFailure { error = tr("Không đọc được file khoá: ${it.message}", "Could not read the key file: ${it.message}") }
    }

    AlertDialog(
        onDismissRequest = { if (op == null) onDismiss() },
        title = { Text(if (editing == null) tr("Thêm VPS", "Add VPS") else tr("Sửa ${editing.name}", "Edit ${editing.name}")) },
        text = {
            Column(Modifier.verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Hint(tr("Nhập thông tin nhà cung cấp VPS gửi cho bạn. App sẽ thử đăng nhập trước khi lưu.", "Enter the details your VPS provider sent you. The app tries to log in before saving."))
                OutlinedTextField(name, { name = it }, label = { Text(tr("Tên gợi nhớ (tuỳ chọn)", "Display name (optional)")) }, singleLine = true, modifier = Modifier.fillMaxWidth())
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    OutlinedTextField(addr, { addr = it.trim() }, label = { Text(tr("IP hoặc tên miền", "IP or domain name")) }, singleLine = true, modifier = Modifier.weight(1f))
                    OutlinedTextField(
                        port, { port = it.filter(Char::isDigit).take(5) }, label = { Text(tr("Cổng", "Port")) }, singleLine = true,
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number), modifier = Modifier.width(84.dp),
                    )
                }
                OutlinedTextField(user, { user = it.trim() }, label = { Text("User") }, singleLine = true, modifier = Modifier.fillMaxWidth())
                SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
                    SegmentedButton(!useKey, { useKey = false }, SegmentedButtonDefaults.itemShape(0, 2)) { Text(tr("Mật khẩu", "Password")) }
                    SegmentedButton(useKey, { useKey = true }, SegmentedButtonDefaults.itemShape(1, 2)) { Text(tr("File khoá", "Key file")) }
                }
                if (useKey) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        OutlinedButton(onClick = { pickKey.launch(arrayOf("*/*")) }) { Text(tr("Chọn file khoá", "Choose key file")) }
                        Hint(keyName, Modifier.padding(start = 8.dp))
                    }
                }
                OutlinedTextField(
                    password, { password = it },
                    label = { Text(if (useKey) tr("Mật khẩu của khoá / sudo (nếu có)", "Key passphrase / sudo password (if any)") else tr("Mật khẩu", "Password")) },
                    placeholder = { if (editing != null) Text(tr("để trống: giữ mật khẩu đã lưu", "leave empty to keep the saved password")) },
                    singleLine = true, visualTransformation = PasswordVisualTransformation(),
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password), modifier = Modifier.fillMaxWidth(),
                )
                error?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall) }
                op?.let {
                    LinearProgressIndicator(Modifier.fillMaxWidth().padding(top = 4.dp))
                    Hint(it)
                }
            }
        },
        confirmButton = {
            TextButton(enabled = op == null, onClick = {
                val p = port.toIntOrNull() ?: 0
                error = when {
                    addr.isBlank() -> tr("Nhập IP hoặc tên miền của VPS", "Enter the VPS's IP or domain name")
                    p !in 1..65535 -> tr("Cổng SSH phải từ 1 đến 65535", "The SSH port must be between 1 and 65535")
                    useKey && keyPem.isEmpty() && editing?.usesKey != true -> tr("Chọn file khoá SSH", "Choose an SSH key file")
                    !useKey && password.isEmpty() && !(editing != null && !editing.usesKey && editing.hasPassword) -> tr("Nhập mật khẩu SSH", "Enter the SSH password")
                    else -> null
                }
                if (error != null) return@TextButton
                val key: String? = if (useKey) keyPem else null
                val h = editing?.edited(name, addr, p, user, password, key)
                    ?: VpsHost.new(vm.newId(), name, addr, p, user, password, key)
                scope.launch { if (vm.save(editing, h)) onDismiss() }
            }) { Text(if (editing == null) tr("Kiểm tra & lưu", "Check & save") else tr("Lưu", "Save")) }
        },
        dismissButton = { TextButton(enabled = op == null, onClick = onDismiss) { Text(tr("Huỷ", "Cancel")) } },
    )
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun DetailCard(
    vm: VpsViewModel, h: VpsHost, state: VpsState?, op: String?, output: List<String>,
    inApp: Set<String>, onImport: (String) -> Boolean,
) {
    var confirm by remember { mutableStateOf<Pair<String, () -> Unit>?>(null) }
    var updating by remember(h.id) { mutableStateOf(false) }

    SectionCard(h.name) {
        op?.let {
            LinearProgressIndicator(Modifier.fillMaxWidth().padding(top = 8.dp))
            Hint(it, Modifier.padding(top = 4.dp))
        }
        when (state) {
            null, VpsState.Loading -> Hint(tr("Đang kết nối tới VPS…", "Connecting to the VPS…"), Modifier.padding(vertical = 8.dp))
            is VpsState.Failed -> {
                Text(state.error, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium, modifier = Modifier.padding(top = 8.dp))
                Row(Modifier.padding(top = 8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    OutlinedButton(onClick = vm::refresh, enabled = op == null) { Text(tr("Thử lại", "Retry")) }
                    if (state.hostKeyChanged) {
                        OutlinedButton(enabled = op == null, onClick = {
                            confirm = tr("Chỉ chấp nhận khoá mới nếu chính bạn vừa cài lại hệ điều hành VPS hoặc nhà cung cấp vừa đổi máy. " +
                                "Nếu không, có thể có kẻ đang giả mạo VPS.",
                                "Only accept the new key if you have just reinstalled the VPS's operating system or your provider has just replaced the machine. " +
                                "Otherwise someone may be impersonating the VPS.") to vm::trustNewKey
                        }) { Text(tr("Chấp nhận khoá mới", "Accept new key"), color = MaterialTheme.colorScheme.error) }
                    }
                }
            }
            is VpsState.Ok -> if (!state.o.installed) {
                InstallForm(update = false, op = op, onInstall = { p, sni, addr, first -> vm.install(p, sni, addr, first, false) })
            } else {
                Installed(vm, state.o, op, inApp, onImport, onConfirm = { confirm = it })
                if (updating) {
                    HorizontalDivider(Modifier.padding(vertical = 10.dp))
                    InstallForm(update = true, op = op, onInstall = { p, sni, addr, _ ->
                        updating = false
                        vm.install(p, sni, addr, "", true)
                    }, onCancel = { updating = false })
                }
                HorizontalDivider(Modifier.padding(vertical = 10.dp))
                FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    OutlinedButton(onClick = vm::restart, enabled = op == null) { Text(tr("Khởi động lại", "Restart")) }
                    OutlinedButton(onClick = vm::logs, enabled = op == null) { Text(tr("Xem log", "View log")) }
                    if (!updating) OutlinedButton(onClick = { updating = true }, enabled = op == null) { Text(tr("Cập nhật / đổi cấu hình", "Update / reconfigure")) }
                    OutlinedButton(enabled = op == null, onClick = {
                        confirm = tr("Gỡ server khỏi VPS? Toàn bộ khoá và người dùng bị xoá, mọi link mất hiệu lực.", "Uninstall the server from the VPS? All keys and users are deleted and every link stops working.") to vm::uninstall
                    }) { Text(tr("Gỡ server", "Uninstall server"), color = MaterialTheme.colorScheme.error) }
                }
            }
        }
        Hint(tr("Khoá SSH: ${vm.fingerprint(h)}", "SSH key: ${vm.fingerprint(h)}"), Modifier.padding(top = 10.dp))
        if (output.isNotEmpty()) {
            Text(
                output.joinToString("\n"),
                fontFamily = FontFamily.Monospace, fontSize = 11.sp, lineHeight = 15.sp,
                modifier = Modifier
                    .padding(top = 10.dp)
                    .fillMaxWidth()
                    .heightIn(max = 260.dp)
                    .clip(RoundedCornerShape(8.dp))
                    .background(MaterialTheme.colorScheme.surfaceContainerLowest)
                    .verticalScroll(rememberScrollState(Int.MAX_VALUE))
                    .horizontalScroll(rememberScrollState())
                    .padding(8.dp),
            )
        }
    }

    confirm?.let { (text, action) ->
        AlertDialog(
            onDismissRequest = { confirm = null },
            text = { Text(text) },
            confirmButton = { TextButton(onClick = { confirm = null; action() }) { Text(tr("Đồng ý", "Confirm"), color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton(onClick = { confirm = null }) { Text(tr("Huỷ", "Cancel")) } },
        )
    }
}

@Composable
private fun Installed(
    vm: VpsViewModel, o: VpsOverview, op: String?, inApp: Set<String>,
    onImport: (String) -> Boolean, onConfirm: (Pair<String, () -> Unit>) -> Unit,
) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var qr by remember { mutableStateOf<Pair<VpsUser, Bitmap>?>(null) }
    var newUser by remember { mutableStateOf("") }

    val (stateText, stateColor) = when (o.active) {
        "active" -> tr("Đang chạy", "Running") to OkGreen
        "activating", "reloading" -> tr("Đang khởi động", "Starting") to WarnAmber
        "failed" -> tr("Lỗi", "Error") to MaterialTheme.colorScheme.error
        else -> tr("Đã dừng", "Stopped") to MaterialTheme.colorScheme.error
    }
    Row(Modifier.padding(top = 8.dp), verticalAlignment = Alignment.CenterVertically) {
        Text("● $stateText", color = stateColor, fontWeight = FontWeight.SemiBold)
        Hint("  sing-box ${o.version}")
    }
    Hint("${o.address} · SNI ${o.sni}")
    if (o.scriptOutdated) {
        Text(
            tr("Bản server trên VPS cũ hơn bản đi kèm app. Bấm “Cập nhật / đổi cấu hình” để cập nhật.", "The server on the VPS is older than the version bundled with the app. Tap “Update / reconfigure” to update it."),
            color = WarnAmber, style = MaterialTheme.typography.bodySmall, modifier = Modifier.padding(top = 6.dp),
        )
    }

    val online = o.users.count { it.conns > 0 }
    Row(Modifier.fillMaxWidth().padding(top = 12.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(
            tr("Người dùng (${o.users.size}${if (o.online) ", $online online" else ""})", "Users (${o.users.size}${if (o.online) ", $online online" else ""})"),
            style = MaterialTheme.typography.titleSmall, modifier = Modifier.weight(1f),
        )
        if (o.stats) {
            TextButton(enabled = op == null, onClick = {
                onConfirm(tr("Đặt lại thống kê lưu lượng của mọi người về 0?", "Reset everyone's traffic statistics to 0?") to vm::resetTraffic)
            }) { Text(tr("Đặt lại", "Reset")) }
        }
    }
    if (o.stats) {
        Hint(tr("Tổng ↓ ${formatBytes(o.users.sumOf { it.down })} ↑ ${formatBytes(o.users.sumOf { it.up })}", "Total ↓ ${formatBytes(o.users.sumOf { it.down })} ↑ ${formatBytes(o.users.sumOf { it.up })}"))
    }
    o.users.forEach { u ->
        val has = u.link in inApp
        Column(Modifier.fillMaxWidth().padding(top = 8.dp)) {
            Text(u.name, style = MaterialTheme.typography.bodyLarge)
            val meta = buildList {
                if (u.conns > 0) add("● online · ${u.ips.size} IP" + if (u.ips.size >= MANY_IPS) tr(" (link bị chia sẻ?)", " (link shared?)") else "")
                if (o.stats) add("↓ ${formatBytes(u.down)} ↑ ${formatBytes(u.up)}")
                if (has) add(tr("đã có trong app", "already in the app"))
            }
            if (meta.isNotEmpty()) {
                Text(
                    meta.joinToString(" · "), style = MaterialTheme.typography.bodySmall,
                    color = when {
                        u.ips.size >= MANY_IPS -> WarnAmber
                        u.conns > 0 -> OkGreen
                        else -> MaterialTheme.colorScheme.onSurfaceVariant
                    },
                )
            }
            Row {
                TextButton(onClick = { scope.launch { vm.qr(u.link)?.let { qr = u to it } } }) { Text("QR") }
                TextButton(onClick = { ctx.copyText("Link ${u.name}", u.link); vm.messageFor(tr("Đã sao chép link của ${u.name}. Gửi riêng cho người đó.", "Copied ${u.name}'s link. Send it to them privately.")) }) {
                    Text(tr("Sao chép", "Copy"))
                }
                if (!has) TextButton(onClick = { onImport(u.link) }) { Text("+ app") }
                if (o.users.size > 1) {
                    TextButton(enabled = op == null, onClick = {
                        onConfirm(tr("Xoá ${u.name}? Link của người này hết dùng được ngay.", "Remove ${u.name}? Their link stops working immediately.") to { vm.removeUser(u.name) })
                    }) { Text(tr("Xoá", "Delete"), color = MaterialTheme.colorScheme.error) }
                }
            }
        }
    }
    Row(Modifier.padding(top = 6.dp), verticalAlignment = Alignment.CenterVertically) {
        OutlinedTextField(
            newUser, { newUser = it.trim() }, label = { Text(tr("Tên người dùng mới", "New user name")) }, singleLine = true,
            isError = newUser.isNotEmpty() && !NAME_RE.matches(newUser), modifier = Modifier.weight(1f),
        )
        Button(
            onClick = { vm.addUser(newUser); newUser = "" },
            enabled = op == null && NAME_RE.matches(newUser), modifier = Modifier.padding(start = 8.dp),
        ) { Text(tr("Thêm", "Add")) }
    }
    Hint(tr("Mỗi người / thiết bị một tên (chữ không dấu, số, . _ -). Muốn cắt quyền ai thì xoá đúng tên đó.", "One name per person / device (unaccented letters, digits, . _ -). To revoke someone's access, delete their name."), Modifier.padding(top = 4.dp))

    qr?.let { (u, bmp) ->
        AlertDialog(
            onDismissRequest = { qr = null },
            title = { Text(u.name) },
            text = {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Image(
                        bmp.asImageBitmap(), contentDescription = tr("Mã QR link của ${u.name}", "QR code of ${u.name}'s link"),
                        filterQuality = FilterQuality.None,
                        modifier = Modifier.size(240.dp).clip(RoundedCornerShape(8.dp)).background(Color.White),
                    )
                    Hint(
                        tr("Quét bằng camera hoặc Google Lens trên máy của ${u.name}, sao chép link rồi dán vào Black Cyclone → Server riêng. " +
                            "Mã QR chứa link đầy đủ: ai chụp được nó cũng dùng được server.",
                            "Scan it with the camera or Google Lens on ${u.name}'s device, copy the link and paste it into Black Cyclone → Own server. " +
                            "The QR code contains the full link: anyone who photographs it can use the server."),
                        Modifier.padding(top = 10.dp),
                    )
                }
            },
            confirmButton = { TextButton(onClick = { qr = null }) { Text(tr("Đóng", "Close")) } },
        )
    }
}

@Composable
private fun InstallForm(
    update: Boolean, op: String?,
    onInstall: (port: Int, sni: String, address: String, firstUser: String) -> Unit,
    onCancel: (() -> Unit)? = null,
) {
    var first by remember { mutableStateOf("") }
    var port by remember { mutableStateOf("") }
    var sni by remember { mutableStateOf("") }
    var addr by remember { mutableStateOf("") }
    var advanced by remember { mutableStateOf(update) }
    val keep = if (update) tr("để trống: giữ nguyên", "leave empty to keep the current value") else null

    Column(Modifier.padding(top = 8.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        if (!update) {
            Hint(
                tr("Chưa cài server trên VPS này. App sẽ cài sing-box (VLESS + Reality), bật BBR, mở tường lửa và tạo người dùng đầu tiên. " +
                    "Mất khoảng 1–2 phút.",
                    "No server is installed on this VPS yet. The app will install sing-box (VLESS + Reality), enable BBR, open the firewall and create the first user. " +
                    "It takes about 1–2 minutes."),
            )
            OutlinedTextField(
                first, { first = it.trim() }, label = { Text(tr("Tên người dùng đầu tiên", "First user name")) }, placeholder = { Text(tr("mặc định: default", "default: default")) },
                singleLine = true, isError = first.isNotEmpty() && !NAME_RE.matches(first), modifier = Modifier.fillMaxWidth(),
            )
            TextButton(onClick = { advanced = !advanced }) { Text(if (advanced) tr("Ẩn tuỳ chọn nâng cao", "Hide advanced options") else tr("Tuỳ chọn nâng cao", "Advanced options")) }
        }
        if (advanced) {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(
                    port, { port = it.filter(Char::isDigit).take(5) }, label = { Text(tr("Cổng", "Port")) },
                    placeholder = { Text(keep ?: "443") }, singleLine = true,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number), modifier = Modifier.width(110.dp),
                )
                OutlinedTextField(
                    sni, { sni = it.trim() }, label = { Text(tr("Trang giả làm (SNI)", "Site to mimic (SNI)")) },
                    placeholder = { Text(keep ?: "www.microsoft.com") }, singleLine = true, modifier = Modifier.weight(1f),
                )
            }
            OutlinedTextField(
                addr, { addr = it.trim() }, label = { Text(tr("Địa chỉ ghi vào link", "Address written into links")) },
                placeholder = { Text(keep ?: tr("để trống: IP công khai của VPS", "leave empty for the VPS's public IP")) }, singleLine = true, modifier = Modifier.fillMaxWidth(),
            )
            Hint(tr("Đổi cổng, SNI hoặc địa chỉ thì link cũ của mọi người hết dùng được: gửi lại link mới cho họ.", "Changing the port, SNI or address breaks everyone's existing links: send them new ones."))
        }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
            onCancel?.let { TextButton(onClick = it) { Text(tr("Huỷ", "Cancel")) } }
            Button(
                enabled = op == null && (first.isEmpty() || NAME_RE.matches(first)) && (port.isEmpty() || port.toInt() in 1..65535),
                onClick = { onInstall(port.toIntOrNull() ?: 0, sni, addr, first) },
            ) { Text(if (update) tr("Cập nhật server", "Update server") else tr("Cài server", "Install server")) }
        }
    }
}
