package com.blackcyclone.vpn.ui

import androidx.annotation.DrawableRes
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.blackcyclone.vpn.MainViewModel
import com.blackcyclone.vpn.VpsViewModel
import com.blackcyclone.vpn.R

private data class Tab(val label: String, @param:DrawableRes val icon: Int)

private val tabs = listOf(
    Tab("Kết nối", R.drawable.ic_nav_power),
    Tab("VPS", R.drawable.ic_nav_server),
    Tab("Cài đặt", R.drawable.ic_nav_settings),
    Tab("Nhật ký", R.drawable.ic_nav_logs),
)

@Composable
fun AppRoot(vm: MainViewModel, onConnect: () -> Unit, onDisconnect: () -> Unit) {
    var tab by rememberSaveable { mutableIntStateOf(0) }
    val snackbar = remember { SnackbarHostState() }
    val toast by vm.toast.collectAsStateWithLifecycle()
    val vpsVm: VpsViewModel = viewModel()

    LaunchedEffect(toast) {
        toast?.let {
            vm.toastShown()
            snackbar.showSnackbar(it)
        }
    }

    Scaffold(
        snackbarHost = { SnackbarHost(snackbar) },
        bottomBar = {
            NavigationBar {
                tabs.forEachIndexed { i, t ->
                    NavigationBarItem(
                        selected = tab == i,
                        onClick = { tab = i },
                        icon = { Icon(painterResource(t.icon), contentDescription = null) },
                        label = { Text(t.label) },
                    )
                }
            }
        },
    ) { pad ->
        Box(Modifier.fillMaxSize().padding(pad)) {
            when (tab) {
                0 -> ConnectScreen(vm, onConnect, onDisconnect)
                1 -> VpsScreen(vpsVm, vm)
                2 -> SettingsScreen()
                else -> LogsScreen(onCopied = { vm.toast("Đã sao chép nhật ký") })
            }
        }
    }
}
