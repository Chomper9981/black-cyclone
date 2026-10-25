package com.blackcyclone.vpn.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext
import rcvcore.Rcvcore

@Composable
fun LogsScreen(onCopied: () -> Unit) {
    val ctx = LocalContext.current
    var text by remember { mutableStateOf("") }
    val listState = rememberLazyListState()
    val lines = remember(text) { if (text.isEmpty()) emptyList() else text.lines() }

    LaunchedEffect(Unit) {
        while (true) {
            text = withContext(Dispatchers.IO) { Rcvcore.logs() }
            delay(1_000)
        }
    }
    LaunchedEffect(lines.size) {
        if (lines.isNotEmpty()) listState.scrollToItem(lines.size - 1)
    }

    Column(Modifier.fillMaxSize().padding(16.dp)) {
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            OutlinedButton(onClick = {
                ctx.copyText("BlackCyclone log", text)
                onCopied()
            }) { Text("Sao chép") }
            OutlinedButton(onClick = {
                Rcvcore.clearLogs()
                text = ""
            }) { Text("Xoá") }
        }
        if (lines.isEmpty()) {
            Hint("Chưa có nhật ký.", Modifier.padding(top = 16.dp))
        } else {
            SelectionContainer(Modifier.padding(top = 12.dp)) {
                LazyColumn(state = listState) {
                    items(lines) {
                        Text(it, fontFamily = FontFamily.Monospace, fontSize = 11.sp, lineHeight = 15.sp)
                    }
                }
            }
        }
    }
}
