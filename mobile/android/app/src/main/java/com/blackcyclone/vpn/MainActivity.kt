package com.blackcyclone.vpn

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.net.VpnService
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import com.blackcyclone.vpn.ui.AppRoot
import com.blackcyclone.vpn.ui.RcvTheme

class MainActivity : ComponentActivity() {
    private val vm: MainViewModel by viewModels()

    private val vpnPermission = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) {
        if (it.resultCode == RESULT_OK) WarpVpnService.start(this)
        else vm.toast("Cần cấp quyền VPN để kết nối")
    }

    private val notificationPermission = registerForActivityResult(ActivityResultContracts.RequestPermission()) {}

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            RcvTheme {
                AppRoot(vm, onConnect = ::connect, onDisconnect = { WarpVpnService.stop(this) })
            }
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            notificationPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
        }
        if (savedInstanceState == null) {
            val fromTile = intent.getBooleanExtra(EXTRA_CONNECT, false)
            if ((fromTile || Prefs.autoConnect) && !TunnelState.state.value.active) connect()
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        if (intent.getBooleanExtra(EXTRA_CONNECT, false) && !TunnelState.state.value.active) connect()
    }

    private fun connect() {
        if (vm.account.value == null) {
            vm.toast("Hãy đăng ký WARP trước")
            return
        }
        val prepare = VpnService.prepare(this)
        if (prepare != null) vpnPermission.launch(prepare) else WarpVpnService.start(this)
    }

    companion object {
        const val EXTRA_CONNECT = "connect"
    }
}
