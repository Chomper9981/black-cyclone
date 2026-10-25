package com.blackcyclone.vpn

import android.app.Notification
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.net.ConnectivityManager
import android.net.IpPrefix
import android.net.Network
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.os.SystemClock
import androidx.core.app.NotificationCompat
import androidx.core.app.ServiceCompat
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import org.json.JSONObject
import rcvcore.Rcvcore
import java.net.InetAddress

class WarpVpnService : VpnService() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var connectJob: Job? = null
    @Volatile private var running = false
    /** Kill switch: a VPN interface nobody reads, so all routed traffic is dropped. */
    @Volatile private var blockPfd: ParcelFileDescriptor? = null
    private var networkCallback: ConnectivityManager.NetworkCallback? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_DISCONNECT) {
            disconnect()
            return START_NOT_STICKY
        }
        // ACTION_CONNECT from the app/tile, or android.net.VpnService / null from always-on VPN.
        startForegroundCompat(buildNotification(TunnelState.state.value.copy(status = Status.CONNECTING)))
        connect()
        return START_NOT_STICKY
    }

    private fun connect() {
        connectJob?.cancel()
        connectJob = scope.launch {
            // Block before tearing the old tunnel down, so a reconnect never leaks.
            if (Prefs.killSwitch) holdBlock()
            stopTunnel()
            TunnelState.set(TunnelInfo(status = Status.CONNECTING, blocking = blockPfd != null, lockdown = lockdownNow()))
            val delays = listOf(2_000L, 4_000L, 8_000L, 16_000L, 30_000L)
            var attempt = 0
            while (isActive) {
                val err = try {
                    startTunnel()
                    null
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    e.message ?: e.toString()
                }
                if (err == null) {
                    monitor()
                    return@launch
                }
                Rcvcore.log("Kết nối thất bại: $err")
                // A failed attempt may have replaced the blocking interface and then closed its
                // own: block afresh.
                if (Prefs.killSwitch) {
                    releaseBlock()
                    holdBlock()
                }
                val blocking = blockPfd != null
                // With the kill switch on, keep trying every 30s instead of giving up and unblocking.
                if (Prefs.account == null || (attempt >= delays.size && !blocking)) {
                    fail(err)
                    return@launch
                }
                val wait = delays[attempt.coerceAtMost(delays.lastIndex)]
                attempt++
                val note = if (blocking) " Kill switch đang chặn mạng để không lộ IP thật." else ""
                TunnelState.update {
                    it.copy(status = Status.CONNECTING, blocking = blocking, message = "Lỗi: $err. Thử lại sau ${wait / 1000}s…$note")
                }
                notify(buildNotification(TunnelState.state.value))
                delay(wait)
            }
        }
    }

    private fun startTunnel() {
        val account = Prefs.account ?: throw IllegalStateException("Chưa đăng ký WARP")
        val pfd = newBuilder(account).establish() ?: throw IllegalStateException("Không tạo được VPN (chưa cấp quyền VPN?)")
        // The Go core owns the fd from here on, also on error.
        Rcvcore.start(pfd.detachFd().toLong(), account.json)
        // The tunnel's interface has replaced the blocking one, so dropping it opens no gap.
        releaseBlock()
        running = true
        registerNetworkCallback()
        TunnelState.set(
            TunnelInfo(
                status = Status.CONNECTING, endpoint = account.endpointText,
                startedAt = SystemClock.elapsedRealtime(), lockdown = lockdownNow(),
            ),
        )
    }

    /** Builder with the tunnel's addresses, routes, DNS and split tunnelling. */
    private fun newBuilder(account: Account?): Builder {
        val b = Builder()
            .setSession("BlackCyclone")
            .setMtu(1280)
            .setBlocking(false)
            .setConfigureIntent(mainActivityIntent(this))
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) b.setMetered(false)

        var v4 = false
        var v6 = false
        for (a in account?.addresses.orEmpty()) {
            val (ip, len) = a.split('/').let { it[0] to (it.getOrNull(1)?.toIntOrNull() ?: if (it[0].contains(':')) 128 else 32) }
            if (ip.isEmpty()) continue
            b.addAddress(ip, len)
            if (ip.contains(':')) v6 = true else v4 = true
        }
        // Both families need an address for their default route to be installed (and so
        // blocked); without an account (kill switch only) any address will do.
        if (!v4) b.addAddress("172.16.0.2", 32)
        if (!v6) b.addAddress("fd00:7263::2", 128)
        DNS.forEach { b.addDnsServer(it) }
        addRoutes(b)
        applySplitTunnel(b)
        return b
    }

    /** Kill switch: route everything into an interface nobody reads, until the tunnel is back. */
    private fun holdBlock() {
        if (blockPfd != null) return
        val pfd = try {
            newBuilder(Prefs.account).establish()
        } catch (e: Exception) {
            Rcvcore.log("Kill switch: không tạo được giao diện chặn: ${e.message}")
            null
        } ?: return
        blockPfd = pfd
        Rcvcore.log("Kill switch: đang chặn mọi kết nối trong lúc đường hầm chưa sẵn sàng")
    }

    private fun releaseBlock() {
        blockPfd?.let { runCatching { it.close() } }
        blockPfd = null
    }

    /** System "Block connections without VPN"; null before Android 10. */
    private fun lockdownNow(): Boolean? =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) isLockdownEnabled else null

    private fun addRoutes(b: Builder) {
        if (!Prefs.bypassLan) {
            b.addRoute("0.0.0.0", 0)
            b.addRoute("::", 0)
            return
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            b.addRoute("0.0.0.0", 0)
            b.addRoute("::", 0)
            for (cidr in LAN) {
                val (ip, len) = cidr.split('/')
                b.excludeRoute(IpPrefix(InetAddress.getByName(ip), len.toInt()))
            }
        } else {
            for (cidr in Rcvcore.routesExcluding(LAN.joinToString(",")).split(',')) {
                val (ip, len) = cidr.split('/')
                b.addRoute(ip, len.toInt())
            }
        }
    }

    private fun applySplitTunnel(b: Builder) {
        val apps = Prefs.splitApps - packageName
        if (Prefs.splitMode == SplitMode.INCLUDE && apps.isNotEmpty()) {
            // Only these apps use the tunnel; this app (and so wireguard-go's socket) stays outside.
            var added = 0
            for (p in apps) {
                try {
                    b.addAllowedApplication(p)
                    added++
                } catch (_: PackageManager.NameNotFoundException) {
                }
            }
            if (added > 0) return
        }
        // Load-bearing: keep this app's own sockets (WireGuard UDP, WARP API) off the tunnel.
        b.addDisallowedApplication(packageName)
        if (Prefs.splitMode == SplitMode.EXCLUDE) {
            for (p in apps) {
                try {
                    b.addDisallowedApplication(p)
                } catch (_: PackageManager.NameNotFoundException) {
                }
            }
        }
    }

    /** Polls traffic counters every second until the tunnel stops. */
    private suspend fun monitor() {
        var lastRx = 0L
        var lastTx = 0L
        var tick = 0
        while (running) {
            val s = runCatching { JSONObject(Rcvcore.stats()) }.getOrNull()
            if (s != null && s.optBoolean("running")) {
                val rx = s.optLong("rx")
                val tx = s.optLong("tx")
                val hs = s.optLong("handshake_ago_ms", -1)
                TunnelState.update { st ->
                    val elapsed = SystemClock.elapsedRealtime() - st.startedAt
                    val status = if (hs >= 0) Status.CONNECTED else Status.CONNECTING
                    val warn = if (hs < 0 && elapsed > NO_HANDSHAKE_WARN_MS) {
                        "Chưa bắt tay được với server sau ${elapsed / 1000}s: endpoint có thể bị chặn, hãy quét endpoint."
                    } else if (hs > STALE_HANDSHAKE_MS) {
                        "Không bắt tay lại được ${hs / 1000}s: endpoint có thể đã bị chặn, hãy quét endpoint."
                    } else null
                    st.copy(
                        status = status, message = warn, handshakeAgoMs = hs, lockdown = lockdownNow(),
                        rx = rx, tx = tx,
                        rxRate = (rx - lastRx).coerceAtLeast(0), txRate = (tx - lastTx).coerceAtLeast(0),
                    )
                }
                lastRx = rx
                lastTx = tx
            }
            if (tick++ % 2 == 0) notify(buildNotification(TunnelState.state.value))
            delay(1_000)
        }
    }

    private fun fail(err: String) {
        stopTunnel()
        if (blockPfd != null) {
            // Stay up holding the block until the user disconnects.
            TunnelState.set(
                TunnelInfo(
                    status = Status.ERROR, blocking = true, lockdown = lockdownNow(),
                    message = "$err. Kill switch đang chặn mọi kết nối để không lộ IP thật: bấm Ngắt để mở lại mạng.",
                ),
            )
            notify(buildNotification(TunnelState.state.value))
            return
        }
        TunnelState.set(TunnelInfo(status = Status.ERROR, message = err))
        ServiceCompat.stopForeground(this, ServiceCompat.STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    private fun disconnect() {
        connectJob?.cancel()
        connectJob = null
        stopTunnel()
        releaseBlock()
        TunnelState.set(TunnelInfo(status = Status.DISCONNECTED))
        ServiceCompat.stopForeground(this, ServiceCompat.STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    private fun stopTunnel() {
        unregisterNetworkCallback()
        if (running) {
            running = false
            Rcvcore.stop()
        }
    }

    private fun registerNetworkCallback() {
        if (networkCallback != null) return
        val cm = getSystemService(ConnectivityManager::class.java)
        val cb = object : ConnectivityManager.NetworkCallback() {
            private var current: Network? = null

            override fun onAvailable(network: Network) {
                setUnderlyingNetworks(arrayOf(network))
                // Switched between Wi-Fi and mobile data: reopen WireGuard's sockets.
                if (current != null && current != network && running) {
                    Rcvcore.log("Mạng thay đổi, kết nối lại socket")
                    Rcvcore.rebind()
                }
                current = network
            }

            override fun onLost(network: Network) {
                if (network == current) setUnderlyingNetworks(null)
            }
        }
        cm.registerDefaultNetworkCallback(cb)
        networkCallback = cb
    }

    private fun unregisterNetworkCallback() {
        networkCallback?.let { runCatching { getSystemService(ConnectivityManager::class.java).unregisterNetworkCallback(it) } }
        networkCallback = null
    }

    override fun onRevoke() {
        // Another VPN took over or the user revoked permission.
        Rcvcore.log("Quyền VPN bị thu hồi")
        disconnect()
    }

    override fun onDestroy() {
        connectJob?.cancel()
        stopTunnel()
        releaseBlock()
        if (TunnelState.state.value.active) TunnelState.set(TunnelInfo(status = Status.DISCONNECTED))
        scope.cancel()
        super.onDestroy()
    }

    private fun startForegroundCompat(n: Notification) {
        val type = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE
        } else 0
        ServiceCompat.startForeground(this, NOTIFICATION_ID, n, type)
    }

    private fun notify(n: Notification) {
        if (running || blockPfd != null) getSystemService(NotificationManager::class.java).notify(NOTIFICATION_ID, n)
    }

    private fun buildNotification(st: TunnelInfo): Notification {
        val title = when {
            st.status == Status.CONNECTED -> "WARP đã kết nối"
            st.blocking -> "Kill switch: đang chặn mạng"
            else -> "Đang kết nối WARP…"
        }
        val text = if (st.status == Status.CONNECTED) {
            "↓ ${formatRate(st.rxRate)}  ↑ ${formatRate(st.txRate)}  ·  ${st.endpoint ?: ""}"
        } else st.message ?: st.endpoint ?: ""
        val stop = PendingIntent.getService(
            this, 1, Intent(this, WarpVpnService::class.java).setAction(ACTION_DISCONNECT),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        return NotificationCompat.Builder(this, App.CHANNEL_VPN)
            .setSmallIcon(R.drawable.ic_tile)
            .setContentTitle(title)
            .setContentText(text)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setSilent(true)
            .setCategory(NotificationCompat.CATEGORY_SERVICE)
            .setContentIntent(mainActivityIntent(this))
            .addAction(0, "Ngắt", stop)
            .build()
    }

    companion object {
        const val ACTION_CONNECT = "com.blackcyclone.vpn.CONNECT"
        const val ACTION_DISCONNECT = "com.blackcyclone.vpn.DISCONNECT"
        private const val NOTIFICATION_ID = 1
        private const val NO_HANDSHAKE_WARN_MS = 10_000L
        private const val STALE_HANDSHAKE_MS = 180_000L

        private val DNS = listOf("1.1.1.1", "1.0.0.1", "2606:4700:4700::1111", "2606:4700:4700::1001")
        private val LAN = listOf(
            "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16", "100.64.0.0/10",
            "fc00::/7", "fe80::/10",
        )

        /** Starts (or restarts) the tunnel. VpnService.prepare() must have been granted. */
        fun start(ctx: Context) {
            ctx.startForegroundService(Intent(ctx, WarpVpnService::class.java).setAction(ACTION_CONNECT))
        }

        fun stop(ctx: Context) {
            if (!TunnelState.state.value.active) return
            ctx.startService(Intent(ctx, WarpVpnService::class.java).setAction(ACTION_DISCONNECT))
        }

        fun mainActivityIntent(ctx: Context): PendingIntent = PendingIntent.getActivity(
            ctx, 0, Intent(ctx, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
    }
}
