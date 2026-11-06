package com.blackcyclone.vpn

import android.app.Application
import android.os.SystemClock
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import rcvcore.Rcvcore

data class ScanHit(val ip: String, val port: Int, val rttMs: Long) {
    val text: String get() = if (ip.contains(':')) "[$ip]:$port" else "$ip:$port"
}

data class ScanState(
    val running: Boolean = false,
    /** 0..1 progress of the time budget. */
    val progress: Float = 0f,
    val scanned: Int = 0,
    val hits: List<ScanHit> = emptyList(),
    val done: Boolean = false,
)

class MainViewModel(app: Application) : AndroidViewModel(app) {
    private val _account = MutableStateFlow(Prefs.account)
    val account: StateFlow<Account?> = _account.asStateFlow()

    private val _mode = MutableStateFlow(Prefs.mode)
    val mode: StateFlow<Mode> = _mode.asStateFlow()

    private val _servers = MutableStateFlow(Prefs.servers)
    val servers: StateFlow<List<ServerEntry>> = _servers.asStateFlow()

    private val _selected = MutableStateFlow(Prefs.selectedServer)
    val selectedServer: StateFlow<String> = _selected.asStateFlow()

    /** Label of the account operation in progress, or null. */
    private val _busy = MutableStateFlow<String?>(null)
    val busy: StateFlow<String?> = _busy.asStateFlow()

    private val _scan = MutableStateFlow(ScanState())
    val scan: StateFlow<ScanState> = _scan.asStateFlow()

    /** One-shot messages for a snackbar. */
    private val _toast = MutableStateFlow<String?>(null)
    val toast: StateFlow<String?> = _toast.asStateFlow()

    fun toastShown() {
        _toast.value = null
    }

    fun toast(msg: String) {
        _toast.value = msg
    }

    private fun save(a: Account?) {
        Prefs.account = a
        _account.value = a
    }

    private fun op(label: String, block: suspend () -> Unit) {
        if (_busy.value != null) return
        _busy.value = label
        viewModelScope.launch {
            try {
                block()
            } catch (e: Exception) {
                _toast.value = e.message ?: e.toString()
            } finally {
                _busy.value = null
            }
        }
    }

    fun register() = op("Đang đăng ký WARP…") {
        val json = withContext(Dispatchers.IO) { Rcvcore.register() }
        save(Account.parse(json) ?: error("Phản hồi đăng ký không hợp lệ"))
        _scan.value = ScanState()
        _toast.value = "Đã đăng ký WARP"
        restartIfActive(Mode.WARP)
    }

    fun applyLicense(key: String) = op("Đang áp dụng key WARP+…") {
        val acc = _account.value ?: error("Chưa đăng ký WARP")
        val json = withContext(Dispatchers.IO) { Rcvcore.applyLicense(acc.json, key.trim()) }
        val updated = Account.parse(json) ?: error("Phản hồi không hợp lệ")
        save(updated)
        _toast.value = "Đã áp dụng key: ${updated.typeLabel}"
        restartIfActive(Mode.WARP)
    }

    /** Imports account JSON (e.g. exported from this app or the desktop app). */
    fun import(json: String): Boolean {
        val a = Account.parse(json) ?: run {
            _toast.value = "JSON tài khoản WARP không hợp lệ"
            return false
        }
        save(a)
        _toast.value = "Đã nhập tài khoản"
        restartIfActive(Mode.WARP)
        return true
    }

    fun setEndpoint(ip: String, port: Int) {
        val acc = _account.value ?: return
        save(acc.withEndpoint(ip, port))
        restartIfActive(Mode.WARP)
    }

    /* ---------------- Mode & servers ---------------- */

    fun setMode(m: Mode) {
        if (m == _mode.value) return
        Prefs.mode = m
        _mode.value = m
        restartIfActive()
    }

    private fun saveServers(list: List<ServerEntry>) {
        Prefs.servers = list
        _servers.value = list
    }

    /** Imports every share link found in text (one or many, e.g. the output of bcserver). */
    fun addServers(text: String): Boolean {
        val found = JSONArray(Rcvcore.parseLinks(text))
        if (found.length() == 0) {
            _toast.value = "Không tìm thấy link hợp lệ (vless, vmess, trojan, ss, hysteria2, tuic)"
            return false
        }
        val list = _servers.value.toMutableList()
        var added = 0
        var firstError: String? = null
        for (i in 0 until found.length()) {
            val o = found.getJSONObject(i)
            val err = o.optString("error")
            if (err.isNotEmpty()) {
                firstError = firstError ?: "${o.optString("name")}: $err"
                Rcvcore.log("Bỏ qua link ${o.optString("name")}: $err")
                continue
            }
            val link = o.getString("link")
            if (list.any { it.link == link }) continue
            list += ServerEntry(Rcvcore.newID(), o.optString("name").ifEmpty { "server" }, link)
            added++
        }
        if (added == 0) {
            _toast.value = firstError?.let { "Link không hợp lệ: $it" } ?: "Các server này đã có trong danh sách"
            return firstError == null
        }
        val wasEmpty = _servers.value.isEmpty()
        saveServers(list)
        if (wasEmpty && list.size == 1) selectServer(list[0].id)
        _toast.value = "Đã thêm $added server" + (firstError?.let { ", bỏ qua link lỗi" } ?: "")
        restartIfActive(Mode.CUSTOM)
        return true
    }

    fun removeServer(id: String) {
        saveServers(_servers.value.filter { it.id != id })
        if (_selected.value == id) selectServer(ServerEntry.AUTO)
        restartIfActive(Mode.CUSTOM)
    }

    fun renameServer(id: String, name: String) {
        val n = name.trim().ifEmpty { return }
        saveServers(_servers.value.map { if (it.id == id) it.copy(name = n) else it })
        restartIfActive(Mode.CUSTOM)
    }

    /** Selects a server (or ServerEntry.AUTO); switches a running tunnel without reconnecting. */
    fun selectServer(id: String) {
        Prefs.selectedServer = id
        _selected.value = id
        val st = TunnelState.state.value
        if (st.active && st.mode == Mode.CUSTOM && _mode.value == Mode.CUSTOM) {
            viewModelScope.launch(Dispatchers.IO) {
                // Fails if the tunnel is only reconnecting / blocking: restart instead.
                if (runCatching { Rcvcore.selectServer(id) }.isFailure) restartIfActive()
            }
        }
    }

    private var scanJob: Job? = null

    fun startScan() {
        val acc = _account.value ?: return
        if (_scan.value.running) return
        _scan.value = ScanState(running = true)
        val started = SystemClock.elapsedRealtime()
        val ticker = viewModelScope.launch {
            while (true) {
                val p = (SystemClock.elapsedRealtime() - started) / (SCAN_TIMEOUT_SEC * 1000f)
                _scan.update { it.copy(progress = p.coerceAtMost(1f)) }
                delay(200)
            }
        }
        scanJob = viewModelScope.launch {
            try {
                val json = withContext(Dispatchers.IO) { Rcvcore.scan(acc.json, SCAN_PARALLEL, SCAN_TIMEOUT_SEC) }
                val o = JSONObject(json)
                val arr = o.optJSONArray("ok")
                val hits = (0 until (arr?.length() ?: 0)).map {
                    val h = arr!!.getJSONObject(it)
                    ScanHit(h.getString("ip"), h.getInt("port"), h.getLong("rtt_ms"))
                }
                _scan.value = ScanState(scanned = o.optInt("scanned"), hits = hits, done = true)
                when {
                    hits.isEmpty() -> _toast.value = "Không endpoint nào phản hồi. Mạng có thể chặn WireGuard/UDP."
                    Prefs.autoPickFastest -> {
                        setEndpoint(hits[0].ip, hits[0].port)
                        _toast.value = "Đã chọn endpoint nhanh nhất ${hits[0].text} (${hits[0].rttMs} ms)"
                    }
                }
            } catch (e: Exception) {
                _scan.value = ScanState(done = true)
                _toast.value = "Quét lỗi: ${e.message}"
            } finally {
                ticker.cancel()
            }
        }
    }

    /** Stops the scan early; it still reports the endpoints found so far. */
    fun cancelScan() {
        Rcvcore.cancelScan()
    }

    /** Reconnects a running tunnel so it picks up changed settings (only if they affect [forMode]). */
    private fun restartIfActive(forMode: Mode? = null) {
        if (forMode != null && forMode != _mode.value) return
        if (TunnelState.state.value.active) WarpVpnService.start(getApplication())
    }

    override fun onCleared() {
        if (_scan.value.running) Rcvcore.cancelScan()
    }

    companion object {
        const val SCAN_PARALLEL = 50L
        const val SCAN_TIMEOUT_SEC = 20L
    }
}
