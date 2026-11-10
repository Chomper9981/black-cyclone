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
import android.net.LinkProperties
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
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
import org.json.JSONArray
import org.json.JSONObject
import rcvcore.Platform
import rcvcore.Rcvcore
import java.io.File
import java.net.Inet6Address
import java.net.InetAddress
import java.net.InetSocketAddress
import java.net.NetworkInterface

/**
 * The VPN: establishes the TUN interface (addresses, routes, DNS, per-app rules) and runs
 * sing-box on it through the Go core, for both WARP and Server riêng. (The class keeps its
 * WARP-only name so an existing "Always-on VPN" setting keeps pointing at it.)
 */
class WarpVpnService : VpnService() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var connectJob: Job? = null
    @Volatile private var running = false
    /** Kill switch: a VPN interface nobody reads, so all routed traffic is dropped. */
    @Volatile private var blockPfd: ParcelFileDescriptor? = null
    private var networkCallback: ConnectivityManager.NetworkCallback? = null
    private var mode = Prefs.mode

    private val platform = object : Platform {
        override fun protect(fd: Int): Boolean = this@WarpVpnService.protect(fd)
        override fun interfaces(): String = networkInterfacesJson()
        override fun connectionOwner(protocol: Int, srcIP: String, srcPort: Int, dstIP: String, dstPort: Int): Int =
            connectionOwnerUid(protocol, srcIP, srcPort, dstIP, dstPort)
    }

    override fun onCreate() {
        super.onCreate()
        Rcvcore.setPlatform(platform)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_DISCONNECT) {
            disconnect()
            return START_NOT_STICKY
        }
        // ACTION_CONNECT from the app/tile, or android.net.VpnService / null from always-on VPN.
        mode = Prefs.mode
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
            TunnelState.set(TunnelInfo(status = Status.CONNECTING, mode = mode, blocking = blockPfd != null, lockdown = lockdownNow()))
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
                Rcvcore.log(tr("Kết nối thất bại: $err", "Connection failed: $err"))
                // A failed attempt may have replaced the blocking interface and then closed its
                // own: block afresh.
                if (Prefs.killSwitch) {
                    releaseBlock()
                    holdBlock()
                }
                val blocking = blockPfd != null
                // With the kill switch on, keep trying every 30s instead of giving up and unblocking.
                if (Prefs.connectProblem() != null || (attempt >= delays.size && !blocking)) {
                    fail(err)
                    return@launch
                }
                val wait = delays[attempt.coerceAtMost(delays.lastIndex)]
                attempt++
                val note = if (blocking) tr(" Kill switch đang chặn mạng để không lộ IP thật.", " The kill switch is blocking the network so your real IP doesn't leak.") else ""
                TunnelState.update {
                    it.copy(status = Status.CONNECTING, blocking = blocking, message = tr("Lỗi: $err. Thử lại sau ${wait / 1000}s…$note", "Error: $err. Retrying in ${wait / 1000}s…$note"))
                }
                notify(buildNotification(TunnelState.state.value))
                delay(wait)
            }
        }
    }

    private fun startTunnel() {
        Prefs.connectProblem()?.let { throw IllegalStateException(it) }
        // sing-box needs the current default network before it dials out.
        registerNetworkCallback()
        reportDefaultNetwork()
        val dataDir = prepareDataDir()
        val pfd = newBuilder().establish() ?: throw IllegalStateException(tr("Không tạo được VPN (chưa cấp quyền VPN?)", "Could not create the VPN (VPN permission not granted?)"))
        // The Go core owns the fd from here on, also on error.
        Rcvcore.start(pfd.detachFd().toLong(), Prefs.coreSettingsJson(), dataDir)
        // The tunnel's interface has replaced the blocking one, so dropping it opens no gap.
        releaseBlock()
        running = true
        val endpoint = if (mode == Mode.WARP) Prefs.account?.endpointText else null
        TunnelState.set(
            TunnelInfo(
                status = Status.CONNECTING, mode = mode, endpoint = endpoint,
                startedAt = SystemClock.elapsedRealtime(), lockdown = lockdownNow(),
            ),
        )
    }

    /** Folder for sing-box's cache; also gets the bundled geoip-vn.srs rule set. */
    private fun prepareDataDir(): String {
        val dir = File(filesDir, "core").apply { mkdirs() }
        val srs = File(dir, "geoip-vn.srs")
        runCatching {
            assets.open("geoip-vn.srs").use { input ->
                val bytes = input.readBytes()
                if (!srs.exists() || srs.length() != bytes.size.toLong()) srs.writeBytes(bytes)
            }
        } // not bundled: sing-box downloads it instead
        return dir.absolutePath
    }

    /** Builder with sing-box's TUN addresses, routes, DNS and split tunnelling. */
    private fun newBuilder(): Builder {
        val b = Builder()
            .setSession("BlackCyclone")
            .setMtu(TUN_MTU)
            .setBlocking(false)
            .setConfigureIntent(mainActivityIntent(this))
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) b.setMetered(false)
        // Must match the TUN inbound of internal/singbox (172.19.0.1/30, fdfe:dcba:9876::1/126).
        b.addAddress("172.19.0.1", 30)
        b.addAddress("fdfe:dcba:9876::1", 126)
        // Any DNS server routed into the TUN is answered by sing-box (hijack-dns).
        b.addDnsServer("172.19.0.2")
        addRoutes(b)
        applySplitTunnel(b)
        return b
    }

    /** Kill switch: route everything into an interface nobody reads, until the tunnel is back. */
    private fun holdBlock() {
        if (blockPfd != null) return
        val pfd = try {
            newBuilder().establish()
        } catch (e: Exception) {
            Rcvcore.log(tr("Kill switch: không tạo được giao diện chặn: ${e.message}", "Kill switch: could not create the blocking interface: ${e.message}"))
            null
        } ?: return
        blockPfd = pfd
        Rcvcore.log(tr("Kill switch: đang chặn mọi kết nối trong lúc đường hầm chưa sẵn sàng", "Kill switch: blocking all connections until the tunnel is ready"))
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
            // Only these apps use the tunnel; this app (and so sing-box's sockets) stays outside.
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
        // Load-bearing: keep this app's own sockets (sing-box's outbound, WARP API) off the
        // tunnel, on top of the per-socket protect() sing-box does.
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

    /** Polls traffic counters and the core's health check every second until the tunnel stops. */
    private suspend fun monitor() {
        var lastRx = 0L
        var lastTx = 0L
        var tick = 0
        while (running) {
            val s = runCatching { JSONObject(Rcvcore.stats()) }.getOrNull()
            if (s != null && s.optBoolean("running")) {
                val rx = s.optLong("down")
                val tx = s.optLong("up")
                val okAgo = s.optLong("ok_ago_ms", -1)
                val server = s.optString("server").ifEmpty { null }
                TunnelState.update { st ->
                    val elapsed = SystemClock.elapsedRealtime() - st.startedAt
                    val status = if (okAgo >= 0) Status.CONNECTED else Status.CONNECTING
                    val hint = if (mode == Mode.WARP) tr("endpoint có thể bị chặn, hãy quét endpoint", "the endpoint may be blocked, try scanning endpoints") else tr("kiểm tra link, tường lửa của VPS, hoặc thử server khác", "check the link and the VPS firewall, or try another server")
                    val warn = if (okAgo < 0 && elapsed > NO_TRAFFIC_WARN_MS) {
                        tr("Chưa thông được sau ${elapsed / 1000}s: $hint.", "No traffic after ${elapsed / 1000}s: $hint.")
                    } else if (okAgo > STALE_MS) {
                        tr("Mất kết nối ${okAgo / 1000}s: $hint.", "Connection lost for ${okAgo / 1000}s: $hint.")
                    } else null
                    st.copy(
                        status = status, message = warn, delayMs = s.optLong("delay_ms"), okAgoMs = okAgo,
                        lockdown = lockdownNow(), endpoint = server ?: st.endpoint,
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
                    status = Status.ERROR, mode = mode, blocking = true, lockdown = lockdownNow(),
                    message = tr("$err. Kill switch đang chặn mọi kết nối để không lộ IP thật: bấm Ngắt để mở lại mạng.", "$err. The kill switch is blocking all connections so your real IP doesn't leak: disconnect to restore the network."),
                ),
            )
            notify(buildNotification(TunnelState.state.value))
            return
        }
        TunnelState.set(TunnelInfo(status = Status.ERROR, mode = mode, message = err))
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
        if (running) {
            running = false
            Rcvcore.stop()
        }
        unregisterNetworkCallback()
    }

    /* ---------------- Network for sing-box ---------------- */

    /**
     * Follows the best physical network (Wi-Fi, mobile data, …), never a VPN: sing-box sends
     * its traffic out through it. The plain default-network callback can report our own TUN.
     */
    private fun registerNetworkCallback() {
        if (networkCallback != null) return
        val cm = getSystemService(ConnectivityManager::class.java)
        val cb = object : ConnectivityManager.NetworkCallback() {
            private var current: Network? = null

            override fun onAvailable(network: Network) {
                current = network
                setUnderlyingNetworks(arrayOf(network))
                reportNetwork(cm, network)
            }

            override fun onLinkPropertiesChanged(network: Network, lp: LinkProperties) {
                if (network == current) reportNetwork(cm, network)
            }

            override fun onCapabilitiesChanged(network: Network, caps: NetworkCapabilities) {
                if (network == current) reportNetwork(cm, network)
            }

            override fun onLost(network: Network) {
                if (network != current) return
                current = null
                setUnderlyingNetworks(null)
                Rcvcore.setDefaultInterface("", -1, false)
            }
        }
        val request = NetworkRequest.Builder()
            .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            .addCapability(NetworkCapabilities.NET_CAPABILITY_NOT_RESTRICTED)
            .addCapability(NetworkCapabilities.NET_CAPABILITY_NOT_VPN)
            .build()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            cm.registerBestMatchingNetworkCallback(request, cb, android.os.Handler(mainLooper))
        } else {
            // Matches the network that satisfies the default request (usually Wi-Fi); does not
            // bring up mobile data while Wi-Fi is connected.
            cm.requestNetwork(request, cb)
        }
        networkCallback = cb
    }

    private fun unregisterNetworkCallback() {
        networkCallback?.let { runCatching { getSystemService(ConnectivityManager::class.java).unregisterNetworkCallback(it) } }
        networkCallback = null
    }

    /** Reports the current physical network right away, before the callback's first event. */
    private fun reportDefaultNetwork() {
        val cm = getSystemService(ConnectivityManager::class.java)
        val n = cm.activeNetwork?.takeIf { !isVpn(cm, it) } ?: physicalNetwork(cm)
        if (n == null) Rcvcore.setDefaultInterface("", -1, false) else reportNetwork(cm, n)
    }

    private fun isVpn(cm: ConnectivityManager, n: Network): Boolean =
        cm.getNetworkCapabilities(n)?.hasTransport(NetworkCapabilities.TRANSPORT_VPN) != false

    /** A connected non-VPN network with Internet, validated ones first. */
    @Suppress("DEPRECATION") // allNetworks: only for this one-off lookup
    private fun physicalNetwork(cm: ConnectivityManager): Network? =
        cm.allNetworks
            .mapNotNull { n -> cm.getNetworkCapabilities(n)?.let { n to it } }
            .filter { (_, c) -> !c.hasTransport(NetworkCapabilities.TRANSPORT_VPN) && c.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET) }
            .sortedByDescending { (_, c) -> c.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED) }
            .firstOrNull()?.first

    private fun reportNetwork(cm: ConnectivityManager, network: Network) {
        if (isVpn(cm, network)) return
        val name = cm.getLinkProperties(network)?.interfaceName ?: return
        val index = runCatching { NetworkInterface.getByName(name)?.index }.getOrNull() ?: return
        val caps = cm.getNetworkCapabilities(network)
        val metered = caps?.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_METERED) == false
        Rcvcore.setDefaultInterface(name, index, metered)
    }

    /** UID of the app owning a connection, -1 if unknown (Android 10+). */
    private fun connectionOwnerUid(protocol: Int, srcIP: String, srcPort: Int, dstIP: String, dstPort: Int): Int {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.Q) return -1
        return runCatching {
            getSystemService(ConnectivityManager::class.java).getConnectionOwnerUid(
                protocol,
                InetSocketAddress(InetAddress.getByName(srcIP), srcPort),
                InetSocketAddress(InetAddress.getByName(dstIP), dstPort),
            )
        }.getOrDefault(-1)
    }

    /** The device's networks as JSON for the core (see rcvcore.netInterface). */
    private fun networkInterfacesJson(): String {
        val cm = getSystemService(ConnectivityManager::class.java)
        val out = JSONArray()
        @Suppress("DEPRECATION") // allNetworks: the callback-based replacement can't answer synchronously.
        for (network in cm.allNetworks) {
            val lp = cm.getLinkProperties(network) ?: continue
            val caps = cm.getNetworkCapabilities(network) ?: continue
            if (caps.hasTransport(NetworkCapabilities.TRANSPORT_VPN)) continue
            val name = lp.interfaceName ?: continue
            val ni = runCatching { NetworkInterface.getByName(name) }.getOrNull() ?: continue
            val type = when {
                caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) -> "wifi"
                caps.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR) -> "cellular"
                caps.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET) -> "ethernet"
                else -> "other"
            }
            val addrs = JSONArray()
            lp.linkAddresses.forEach { la ->
                val a = la.address
                val host = if (a is Inet6Address) a.hostAddress!!.substringBefore('%') else a.hostAddress!!
                addrs.put("$host/${la.prefixLength}")
            }
            val dns = JSONArray()
            lp.dnsServers.forEach { dns.put(it.hostAddress!!.substringBefore('%')) }
            out.put(
                JSONObject()
                    .put("name", name)
                    .put("index", ni.index)
                    .put("mtu", lp.mtu.takeIf { it > 0 } ?: runCatching { ni.mtu }.getOrDefault(1500))
                    .put("type", type)
                    .put("addresses", addrs)
                    .put("dns", dns)
                    .put("metered", !caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_METERED))
                    .put("up", runCatching { ni.isUp }.getOrDefault(true))
                    .put("loopback", false)
                    .put("multicast", runCatching { ni.supportsMulticast() }.getOrDefault(false)),
            )
        }
        return out.toString()
    }

    override fun onRevoke() {
        // Another VPN took over or the user revoked permission.
        Rcvcore.log(tr("Quyền VPN bị thu hồi", "VPN permission revoked"))
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
            st.status == Status.CONNECTED -> tr("${mode.label} đã kết nối", "${mode.label} connected")
            st.blocking -> tr("Kill switch: đang chặn mạng", "Kill switch: blocking the network")
            else -> tr("Đang kết nối ${mode.label}…", "Connecting ${mode.label}…")
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
            .addAction(0, tr("Ngắt", "Disconnect"), stop)
            .build()
    }

    companion object {
        const val ACTION_CONNECT = "com.blackcyclone.vpn.CONNECT"
        const val ACTION_DISCONNECT = "com.blackcyclone.vpn.DISCONNECT"
        private const val NOTIFICATION_ID = 1
        private const val TUN_MTU = 9000
        private const val NO_TRAFFIC_WARN_MS = 12_000L
        private const val STALE_MS = 65_000L

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
