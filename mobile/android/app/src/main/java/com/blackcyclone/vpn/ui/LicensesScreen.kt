package com.blackcyclone.vpn.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/** License texts bundled in assets/licenses/ by the copyLicenses Gradle task. */
private val documents = listOf(
    "GPL-3.0.txt" to "GNU GPL 3.0 (app Android, sing-box)",
    "LICENSE" to "Mã nguồn Black Cyclone (MIT)",
    "THIRD_PARTY_NOTICES.md" to "Phần mềm bên thứ ba, nguồn mã",
    "go-android.txt" to "Thư viện Go (sing-box và các thư viện đi kèm)",
    "Apache-2.0.txt" to "Apache License 2.0 (AndroidX, Kotlin)",
)

@Composable
fun LicensesScreen(onBack: () -> Unit) {
    var open by remember { mutableStateOf<Pair<String, String>?>(null) }
    BackHandler { if (open != null) open = null else onBack() }

    open?.let { (file, title) ->
        LicenseText(file, title, onBack = { open = null })
        return
    }

    Column(Modifier.fillMaxSize().padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        OutlinedButton(onClick = onBack) { Text("Quay lại") }
        SectionCard("Giấy phép") {
            Hint(
                "App Android BlackCyclone chứa sing-box nên được phát hành theo giấy phép GNU GPL 3.0. " +
                    "Mã nguồn của Black Cyclone (© 2026 Red Cyclone) theo giấy phép MIT, " +
                    "tại github.com/Chomper9981/black-cyclone. Phần mềm bên thứ ba giữ giấy phép riêng.",
                Modifier.padding(top = 4.dp, bottom = 4.dp),
            )
            documents.forEach { doc ->
                Text(
                    doc.second,
                    Modifier.fillMaxWidth().clickable { open = doc }.padding(vertical = 12.dp),
                    style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.primary,
                )
            }
        }
    }
}

@Composable
private fun LicenseText(file: String, title: String, onBack: () -> Unit) {
    val ctx = LocalContext.current
    val lines by produceState<List<String>?>(null, file) {
        value = withContext(Dispatchers.IO) {
            runCatching { ctx.assets.open("licenses/$file").bufferedReader().use { it.readLines() } }
                .getOrElse { listOf("Không đọc được $file: ${it.message}") }
        }
    }
    Column(Modifier.fillMaxSize().padding(16.dp)) {
        OutlinedButton(onClick = onBack) { Text("Quay lại") }
        Text(title, Modifier.padding(top = 12.dp, bottom = 8.dp), style = MaterialTheme.typography.titleMedium)
        lines?.let { all ->
            SelectionContainer {
                LazyColumn {
                    items(all) {
                        Text(it, fontFamily = FontFamily.Monospace, fontSize = 11.sp, lineHeight = 15.sp)
                    }
                }
            }
        }
    }
}
