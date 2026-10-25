package com.blackcyclone.vpn

import android.app.PendingIntent
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.service.quicksettings.Tile
import android.service.quicksettings.TileService
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch

/** Quick Settings tile that toggles WARP. */
class WarpTileService : TileService() {
    private var scope: CoroutineScope? = null

    override fun onStartListening() {
        scope = CoroutineScope(Job() + Dispatchers.Main).also { s ->
            s.launch { TunnelState.state.collect { update(it) } }
        }
    }

    override fun onStopListening() {
        scope?.cancel()
        scope = null
    }

    override fun onClick() {
        val st = TunnelState.state.value
        when {
            st.active -> WarpVpnService.stop(this)
            // No VPN permission or no account yet: open the app, which handles both.
            VpnService.prepare(this) != null || Prefs.account == null -> openApp()
            else -> WarpVpnService.start(this)
        }
    }

    private fun openApp() {
        val intent = Intent(this, MainActivity::class.java)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            .putExtra(MainActivity.EXTRA_CONNECT, true)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            startActivityAndCollapse(PendingIntent.getActivity(this, 2, intent, PendingIntent.FLAG_IMMUTABLE))
        } else {
            @Suppress("DEPRECATION")
            startActivityAndCollapse(intent)
        }
    }

    private fun update(st: TunnelInfo) {
        val tile = qsTile ?: return
        tile.state = if (st.active) Tile.STATE_ACTIVE else Tile.STATE_INACTIVE
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            tile.subtitle = when (st.status) {
                Status.CONNECTED -> "Đã kết nối"
                Status.CONNECTING -> "Đang kết nối…"
                Status.ERROR -> "Lỗi"
                Status.DISCONNECTED -> "Đã ngắt"
            }
        }
        tile.updateTile()
    }
}
