package com.blackcyclone.vpn

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update

enum class Status { DISCONNECTED, CONNECTING, CONNECTED, ERROR }

data class TunnelInfo(
    val status: Status = Status.DISCONNECTED,
    /** Error text, or a warning while connecting. */
    val message: String? = null,
    val endpoint: String? = null,
    val rx: Long = 0,
    val tx: Long = 0,
    val rxRate: Long = 0,
    val txRate: Long = 0,
    /** Milliseconds since the last WireGuard handshake, -1 if none yet. */
    val handshakeAgoMs: Long = -1,
    /** elapsedRealtime when the tunnel came up, 0 if not running. */
    val startedAt: Long = 0,
    /** Kill switch holds a VPN interface that drops all traffic (tunnel down, reconnecting). */
    val blocking: Boolean = false,
    /** System "Block connections without VPN" (Android 10+); null if unknown. */
    val lockdown: Boolean? = null,
) {
    val active: Boolean get() = status == Status.CONNECTING || status == Status.CONNECTED || blocking
}

/** Process-wide tunnel state, written by [WarpVpnService] and read by the UI and the tile. */
object TunnelState {
    private val _state = MutableStateFlow(TunnelInfo())
    val state: StateFlow<TunnelInfo> = _state

    fun set(info: TunnelInfo) {
        _state.value = info
    }

    fun update(f: (TunnelInfo) -> TunnelInfo) = _state.update(f)
}

fun formatBytes(b: Long): String = when {
    b < 1024 -> "$b B"
    b < 1024 * 1024 -> "%.1f KB".format(b / 1024.0)
    b < 1024L * 1024 * 1024 -> "%.1f MB".format(b / 1024.0 / 1024)
    else -> "%.2f GB".format(b / 1024.0 / 1024 / 1024)
}

fun formatRate(b: Long): String = formatBytes(b) + "/s"
