package com.blackcyclone.vpn.ui

import android.content.Intent
import android.provider.Settings
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import com.blackcyclone.vpn.BuildConfig
import com.blackcyclone.vpn.Prefs
import com.blackcyclone.vpn.SplitMode

@Composable
fun SettingsScreen() {
    var pickingApps by remember { mutableStateOf(false) }
    var splitApps by remember { mutableStateOf(Prefs.splitApps) }
    var showLicenses by remember { mutableStateOf(false) }
    if (showLicenses) {
        LicensesScreen(onBack = { showLicenses = false })
        return
    }
    if (pickingApps) {
        AppPicker(
            initial = splitApps,
            onDone = {
                Prefs.splitApps = it
                splitApps = it
                pickingApps = false
            },
        )
        return
    }

    val ctx = LocalContext.current
    var bypassLan by remember { mutableStateOf(Prefs.bypassLan) }
    var killSwitch by remember { mutableStateOf(Prefs.killSwitch) }
    var autoConnect by remember { mutableStateOf(Prefs.autoConnect) }
    var autoPick by remember { mutableStateOf(Prefs.autoPickFastest) }
    var splitMode by remember { mutableStateOf(Prefs.splitMode) }

    Column(
        Modifier.verticalScroll(rememberScrollState()).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Hint("Thay đổi có hiệu lực từ lần kết nối sau.")
        SectionCard("Kết nối") {
            SwitchRow("Bỏ qua mạng LAN", "Mạng nội bộ (192.168.x.x, 10.x.x.x…) đi thẳng, không qua WARP: vẫn dùng được máy in, NAS, Chromecast.", bypassLan) {
                bypassLan = it; Prefs.bypassLan = it
            }
            SwitchRow("Tự kết nối khi mở app", "Mở app là tự bật WARP.", autoConnect) {
                autoConnect = it; Prefs.autoConnect = it
            }
            SwitchRow("Tự chọn endpoint nhanh nhất", "Sau khi quét, tự dùng endpoint có ping thấp nhất.", autoPick) {
                autoPick = it; Prefs.autoPickFastest = it
            }
        }

        SectionCard("Chia tuyến theo ứng dụng") {
            Hint("Chọn ứng dụng nào đi qua WARP. Ví dụ để app ngân hàng, game đi thẳng.", Modifier.padding(top = 4.dp, bottom = 4.dp))
            listOf(
                SplitMode.OFF to "Tắt: mọi ứng dụng qua WARP",
                SplitMode.EXCLUDE to "Ứng dụng đã chọn đi thẳng",
                SplitMode.INCLUDE to "Chỉ ứng dụng đã chọn qua WARP",
            ).forEach { (mode, label) ->
                Row(
                    Modifier.fillMaxWidth().selectable(splitMode == mode) { splitMode = mode; Prefs.splitMode = mode },
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    RadioButton(selected = splitMode == mode, onClick = null)
                    Text(label, Modifier.padding(start = 8.dp, top = 10.dp, bottom = 10.dp))
                }
            }
            OutlinedButton(onClick = { pickingApps = true }, enabled = splitMode != SplitMode.OFF) {
                Text("Chọn ứng dụng (${splitApps.size})")
            }
        }

        SectionCard("Kill switch") {
            SwitchRow(
                "Chặn mạng khi WARP rớt",
                "Trong lúc kết nối lại hoặc khi lỗi, chặn mọi kết nối thay vì để đi thẳng (lộ IP thật), đến khi WARP nối lại được hoặc bạn bấm Ngắt.",
                killSwitch,
            ) { killSwitch = it; Prefs.killSwitch = it }
            Hint(
                "Kill switch của app không bảo vệ được khi app bị tắt hẳn hoặc lúc khởi động máy. Để kín hoàn toàn, vào Cài đặt VPN, chạm bánh răng cạnh BlackCyclone, bật \"VPN luôn bật\" và \"Chặn kết nối không qua VPN\".",
                Modifier.padding(top = 4.dp, bottom = 8.dp),
            )
            OutlinedButton(onClick = {
                ctx.startActivity(Intent(Settings.ACTION_VPN_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            }) { Text("Mở cài đặt VPN") }
        }

        SectionCard("Ô cài đặt nhanh") {
            Hint("Kéo thanh thông báo xuống, chạm bút chỉnh sửa và thêm ô \"WARP\" để bật/tắt nhanh.", Modifier.padding(top = 4.dp))
        }

        SectionCard("Giới thiệu") {
            Hint("BlackCyclone ${BuildConfig.VERSION_NAME} · © 2026 Red Cyclone · MIT", Modifier.padding(top = 4.dp, bottom = 8.dp))
            OutlinedButton(onClick = { showLicenses = true }) { Text("Giấy phép mã nguồn mở") }
        }
    }
}

@Composable
private fun SwitchRow(title: String, desc: String, checked: Boolean, onChange: (Boolean) -> Unit) {
    Row(
        Modifier.fillMaxWidth().clickable { onChange(!checked) }.padding(vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f).padding(end = 12.dp)) {
            Text(title, style = MaterialTheme.typography.bodyLarge)
            Hint(desc)
        }
        Switch(checked = checked, onCheckedChange = onChange)
    }
}
