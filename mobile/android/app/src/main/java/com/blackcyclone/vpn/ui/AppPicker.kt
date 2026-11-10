package com.blackcyclone.vpn.ui

import android.Manifest
import android.content.pm.ApplicationInfo
import android.content.pm.PackageManager
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.Image
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilterChip
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.core.graphics.drawable.toBitmap
import com.blackcyclone.vpn.tr
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

private data class AppEntry(val pkg: String, val label: String, val system: Boolean)

/** Full-screen picker for split-tunnel apps. Back or "Xong" saves the selection. */
@Composable
fun AppPicker(initial: Set<String>, onDone: (Set<String>) -> Unit) {
    val ctx = LocalContext.current
    val pm = ctx.packageManager
    var selected by remember { mutableStateOf(initial) }
    var query by remember { mutableStateOf("") }
    var showSystem by remember { mutableStateOf(false) }
    BackHandler { onDone(selected) }

    val apps by produceState<List<AppEntry>?>(null) {
        value = withContext(Dispatchers.IO) {
            pm.getInstalledApplications(PackageManager.GET_META_DATA)
                .filter { it.packageName != ctx.packageName && usesInternet(pm, it.packageName) }
                .map {
                    AppEntry(it.packageName, it.loadLabel(pm).toString(), it.flags and ApplicationInfo.FLAG_SYSTEM != 0)
                }
                .sortedBy { it.label.lowercase() }
        }
    }

    Column(Modifier.fillMaxSize().padding(horizontal = 16.dp)) {
        Row(Modifier.fillMaxWidth().padding(vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            Text(tr("Đã chọn ${selected.size}", "${selected.size} selected"), Modifier.weight(1f))
            Button(onClick = { onDone(selected) }) { Text(tr("Xong", "Done")) }
        }
        OutlinedTextField(
            value = query, onValueChange = { query = it }, singleLine = true,
            label = { Text(tr("Tìm ứng dụng", "Search apps")) }, modifier = Modifier.fillMaxWidth(),
        )
        FilterChip(
            selected = showSystem, onClick = { showSystem = !showSystem },
            label = { Text(tr("Hiện ứng dụng hệ thống", "Show system apps")) }, modifier = Modifier.padding(vertical = 4.dp),
        )
        val list = apps
        if (list == null) {
            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
            return@Column
        }
        val q = query.trim().lowercase()
        val shown = list.filter {
            (showSystem || !it.system || it.pkg in selected) &&
                (q.isEmpty() || it.label.lowercase().contains(q) || it.pkg.contains(q))
        }
        LazyColumn(Modifier.fillMaxSize()) {
            items(shown, key = { it.pkg }) { app ->
                val checked = app.pkg in selected
                val toggle = { selected = if (checked) selected - app.pkg else selected + app.pkg }
                Row(
                    Modifier.fillMaxWidth().clickable(onClick = toggle).padding(vertical = 6.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    AppIcon(pm, app.pkg)
                    Spacer(Modifier.width(12.dp))
                    Column(Modifier.weight(1f)) {
                        Text(app.label, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        Hint(app.pkg, Modifier)
                    }
                    Checkbox(checked = checked, onCheckedChange = { toggle() })
                }
            }
        }
    }
}

@Composable
private fun AppIcon(pm: PackageManager, pkg: String) {
    val icon by produceState<ImageBitmap?>(null, pkg) {
        value = withContext(Dispatchers.IO) {
            runCatching { pm.getApplicationIcon(pkg).toBitmap(96, 96).asImageBitmap() }.getOrNull()
        }
    }
    Box(Modifier.size(40.dp)) {
        icon?.let { Image(it, contentDescription = null, modifier = Modifier.size(40.dp)) }
    }
}

private fun usesInternet(pm: PackageManager, pkg: String): Boolean =
    pm.checkPermission(Manifest.permission.INTERNET, pkg) == PackageManager.PERMISSION_GRANTED
