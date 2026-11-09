package com.blackcyclone.vpn

import android.app.Application
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONObject
import rcvcore.Rcvcore
import rcvcore.VpsOutput

/** What the VPS tab knows about one VPS. */
sealed interface VpsState {
    data object Loading : VpsState
    data class Ok(val o: VpsOverview) : VpsState
    data class Failed(val error: String) : VpsState {
        /** The VPS answered with another SSH key than the pinned one. */
        val hostKeyChanged: Boolean get() = error.contains("khoá SSH của VPS đã thay đổi")
    }
}

/**
 * The "VPS" tab: manage your own Black Cyclone servers over SSH (same Go code
 * as the desktop app). Whoever holds the SSH login is the admin; users of the
 * server only get a vless:// link.
 */
class VpsViewModel(app: Application) : AndroidViewModel(app) {
    private val _hosts = MutableStateFlow(Prefs.vpsHosts)
    val hosts: StateFlow<List<VpsHost>> = _hosts.asStateFlow()

    private val _selected = MutableStateFlow(Prefs.selectedVps?.takeIf { id -> _hosts.value.any { it.id == id } } ?: _hosts.value.firstOrNull()?.id)
    val selected: StateFlow<String?> = _selected.asStateFlow()

    private val _states = MutableStateFlow<Map<String, VpsState>>(emptyMap())
    val states: StateFlow<Map<String, VpsState>> = _states.asStateFlow()

    /** Label of the operation in progress on the selected VPS, or null. */
    private val _op = MutableStateFlow<String?>(null)
    val op: StateFlow<String?> = _op.asStateFlow()

    /** Install output, or the server log. */
    private val _output = MutableStateFlow<List<String>>(emptyList())
    val output: StateFlow<List<String>> = _output.asStateFlow()

    private val _message = MutableStateFlow<String?>(null)
    val message: StateFlow<String?> = _message.asStateFlow()

    fun messageShown() {
        _message.value = null
    }

    fun messageFor(msg: String) {
        _message.value = msg
    }

    private fun host(id: String?) = _hosts.value.firstOrNull { it.id == id }

    private fun saveHosts(list: List<VpsHost>) {
        Prefs.vpsHosts = list
        _hosts.value = list
    }

    fun select(id: String) {
        if (_selected.value == id) return
        _selected.value = id
        Prefs.selectedVps = id
        _output.value = emptyList()
        if (_states.value[id] !is VpsState.Ok) refresh()
    }

    /**
     * Checks the login (and root rights) and saves the VPS: a new one when
     * [editing] is null. Returns true when saved.
     */
    suspend fun save(editing: VpsHost?, h: VpsHost): Boolean {
        if (_op.value != null) return false
        val unchanged = editing != null && editing.addr == h.addr && editing.port == h.port &&
            editing.user == h.user && editing.json.optString("password") == h.json.optString("password") &&
            editing.json.optString("key_pem") == h.json.optString("key_pem")
        _op.value = "Đang kiểm tra đăng nhập…"
        val checked = try {
            if (unchanged) h else VpsHost(JSONObject(withContext(Dispatchers.IO) { Rcvcore.vpsCheck(h.json.toString(), false) }))
        } catch (e: Exception) {
            _message.value = e.message ?: e.toString()
            null
        } finally {
            _op.value = null
        }
        checked ?: return false
        saveHosts(if (editing == null) _hosts.value + checked else _hosts.value.map { if (it.id == checked.id) checked else it })
        _message.value = if (editing == null) "Đã thêm ${checked.name}. Khoá SSH: ${fingerprint(checked)}" else "Đã lưu ${checked.name}"
        _states.update { it - checked.id }
        if (editing == null) select(checked.id) else if (_selected.value == checked.id) refresh()
        return true
    }

    /** Forgets the VPS in this app; the server on it keeps running. */
    fun remove(id: String) {
        saveHosts(_hosts.value.filter { it.id != id })
        _states.update { it - id }
        if (_selected.value == id) {
            _selected.value = _hosts.value.firstOrNull()?.id
            Prefs.selectedVps = _selected.value
            _selected.value?.let { if (_states.value[it] == null) refresh() }
        }
        _message.value = "Đã xoá VPS khỏi app. Server trên VPS vẫn chạy, link của mọi người vẫn dùng được."
    }

    /** Runs an operation on the selected VPS that returns a new overview JSON. */
    private fun run(label: String, done: String? = null, call: (hostJson: String) -> String?) {
        val h = host(_selected.value) ?: return
        if (_op.value != null) return
        _op.value = label
        viewModelScope.launch {
            try {
                val json = withContext(Dispatchers.IO) { call(h.json.toString()) }
                if (json != null) _states.update { it + (h.id to VpsState.Ok(VpsOverview.parse(json))) }
                done?.let { _message.value = it }
            } catch (e: Exception) {
                val msg = e.message ?: e.toString()
                if (label == LOADING) _states.update { it + (h.id to VpsState.Failed(msg)) } else _message.value = msg
            } finally {
                _op.value = null
            }
        }
    }

    fun refresh() {
        val id = _selected.value ?: return
        if (_states.value[id] !is VpsState.Ok) _states.update { it + (id to VpsState.Loading) }
        run(LOADING) { Rcvcore.vpsOverview(it) }
    }

    /** Quiet refresh for the periodic update of who is online. */
    fun poll() {
        val h = host(_selected.value) ?: return
        val st = _states.value[h.id]
        if (_op.value != null || st !is VpsState.Ok || !st.o.installed) return
        viewModelScope.launch {
            runCatching { withContext(Dispatchers.IO) { Rcvcore.vpsOverview(h.json.toString()) } }
                .onSuccess { json -> if (_op.value == null) _states.update { it + (h.id to VpsState.Ok(VpsOverview.parse(json))) } }
        }
    }

    fun install(port: Int, sni: String, address: String, firstUser: String, update: Boolean) {
        val opts = JSONObject().put("port", port).put("sni", sni.trim()).put("host", address.trim()).put("name", firstUser.trim())
        _output.value = emptyList()
        val out = object : VpsOutput {
            override fun line(line: String) {
                _output.update { (it + line).takeLast(OUTPUT_MAX) }
            }
        }
        run(if (update) "Đang cập nhật server…" else "Đang cài server… (1–2 phút)",
            if (update) "Đã cập nhật server" else "Đã cài server. Dùng nút “+ app” để thêm link vào máy này.") {
            Rcvcore.vpsInstall(it, opts.toString(), out)
        }
    }

    fun addUser(name: String) = run("Đang thêm $name…", "Đã thêm $name") { Rcvcore.vpsAddUser(it, name.trim()) }

    fun removeUser(name: String) = run("Đang xoá $name…", "Đã xoá $name: link của người này hết dùng được") { Rcvcore.vpsRemoveUser(it, name) }

    fun restart() = run("Đang khởi động lại…", "Đã khởi động lại server") { Rcvcore.vpsRestart(it) }

    fun resetTraffic() = run("Đang đặt lại thống kê…", "Đã đặt lại thống kê lưu lượng") { Rcvcore.vpsResetTraffic(it) }

    fun uninstall() = run("Đang gỡ server…", "Đã gỡ server khỏi VPS") { Rcvcore.vpsUninstall(it) }

    fun logs() = run("Đang tải log…") {
        val text = Rcvcore.vpsLogs(it)
        _output.value = text.trimEnd().lines().takeLast(OUTPUT_MAX)
        null
    }

    /** Accepts the VPS's new SSH key after the user confirmed the VPS was reinstalled. */
    fun trustNewKey() {
        val h = host(_selected.value) ?: return
        if (_op.value != null) return
        _op.value = "Đang kết nối…"
        viewModelScope.launch {
            try {
                val checked = VpsHost(JSONObject(withContext(Dispatchers.IO) { Rcvcore.vpsCheck(h.json.toString(), true) }))
                saveHosts(_hosts.value.map { if (it.id == checked.id) checked else it })
                _message.value = "Đã chấp nhận khoá SSH mới: ${Rcvcore.vpsFingerprint(checked.hostKey)}"
                _states.update { it - h.id }
            } catch (e: Exception) {
                _message.value = e.message ?: e.toString()
            } finally {
                _op.value = null
            }
            refresh()
        }
    }

    suspend fun qr(link: String): Bitmap? = withContext(Dispatchers.IO) {
        runCatching { Rcvcore.linkQR(link, 640).let { BitmapFactory.decodeByteArray(it, 0, it.size) } }.getOrNull()
    }

    fun fingerprint(h: VpsHost): String = runCatching { Rcvcore.vpsFingerprint(h.hostKey) }.getOrDefault("")

    fun newId(): String = Rcvcore.newID()

    companion object {
        private const val LOADING = "Đang kết nối tới VPS…"
        private const val OUTPUT_MAX = 400
    }
}
