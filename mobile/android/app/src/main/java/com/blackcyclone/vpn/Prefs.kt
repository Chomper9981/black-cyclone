package com.blackcyclone.vpn

import android.content.Context
import android.content.SharedPreferences
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import androidx.core.content.edit
import org.json.JSONArray
import org.json.JSONObject
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

enum class SplitMode { OFF, EXCLUDE, INCLUDE }

/**
 * App settings. The WARP account (keys, API token) and the server links (which carry
 * credentials) are encrypted with an Android Keystore key.
 */
object Prefs {
    private lateinit var sp: SharedPreferences

    fun init(ctx: Context) {
        sp = ctx.getSharedPreferences("settings", Context.MODE_PRIVATE)
    }

    var account: Account?
        get() = sp.getString("account", null)?.let { Crypto.decrypt(it) }?.let { Account.parse(it) }
        set(v) = sp.edit {
            if (v == null) remove("account") else putString("account", Crypto.encrypt(v.json))
        }

    var mode: Mode
        get() = Mode.of(sp.getString("mode", null))
        set(v) = sp.edit { putString("mode", v.id) }

    var servers: List<ServerEntry>
        get() = ServerEntry.listFromJson(sp.getString("servers", null)?.let { Crypto.decrypt(it) })
        set(v) = sp.edit { putString("servers", Crypto.encrypt(ServerEntry.listToJson(v))) }

    /** VPSes managed in the VPS tab (SSH login details included), as vps.Host JSON objects. */
    var vpsHosts: List<VpsHost>
        get() = VpsHost.listFromJson(sp.getString("vps_hosts", null)?.let { Crypto.decrypt(it) })
        set(v) = sp.edit { putString("vps_hosts", Crypto.encrypt(VpsHost.listToJson(v))) }

    var selectedVps: String?
        get() = sp.getString("selected_vps", null)
        set(v) = sp.edit { putString("selected_vps", v) }

    /** ServerEntry.id, or ServerEntry.AUTO for the lowest-latency server. */
    var selectedServer: String
        get() = sp.getString("selected_server", ServerEntry.AUTO)!!
        set(v) = sp.edit { putString("selected_server", v) }

    /** Vietnamese IPs and .vn domains skip the tunnel. */
    var directVn: Boolean
        get() = sp.getBoolean("direct_vn", true)
        set(v) = sp.edit { putBoolean("direct_vn", v) }

    /** Why the current mode cannot connect yet, or null when it can. */
    fun connectProblem(): String? = when (mode) {
        Mode.WARP -> if (account == null) "Hãy đăng ký WARP trước" else null
        Mode.CUSTOM -> if (servers.isEmpty()) "Hãy thêm server trước (dán link vless://…)" else null
    }

    /** Settings for the Go core, in the JSON layout of the desktop app's settings.Settings. */
    fun coreSettingsJson(): String {
        val o = JSONObject()
            .put("mode", mode.id)
            .put("bypass_lan", bypassLan)
            .put("servers", JSONArray().apply { servers.forEach { put(it.toJson()) } })
            .put("selected_server", selectedServer)
            .put("routing", JSONObject().put("direct_vn", directVn))
            .put("interrupt_on_switch", true)
        account?.let { o.put("warp", JSONObject(it.json)) }
        return o.toString()
    }

    var bypassLan: Boolean
        get() = sp.getBoolean("bypass_lan", true)
        set(v) = sp.edit { putBoolean("bypass_lan", v) }

    /** Block all traffic while the tunnel is down (reconnecting / failed) instead of leaking it. */
    var killSwitch: Boolean
        get() = sp.getBoolean("kill_switch", true)
        set(v) = sp.edit { putBoolean("kill_switch", v) }

    var autoConnect: Boolean
        get() = sp.getBoolean("auto_connect", false)
        set(v) = sp.edit { putBoolean("auto_connect", v) }

    var autoPickFastest: Boolean
        get() = sp.getBoolean("auto_pick_fastest", true)
        set(v) = sp.edit { putBoolean("auto_pick_fastest", v) }

    var splitMode: SplitMode
        get() = runCatching { SplitMode.valueOf(sp.getString("split_mode", null)!!) }.getOrDefault(SplitMode.OFF)
        set(v) = sp.edit { putString("split_mode", v.name) }

    var splitApps: Set<String>
        get() = sp.getStringSet("split_apps", emptySet())!!.toSet()
        set(v) = sp.edit { putStringSet("split_apps", v) }
}

private object Crypto {
    private const val ALIAS = "rcv_account"
    private const val GCM_TAG_BITS = 128

    private fun key(): SecretKey {
        val ks = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (ks.getKey(ALIAS, null) as? SecretKey)?.let { return it }
        val gen = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
        gen.init(
            KeyGenParameterSpec.Builder(ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build(),
        )
        return gen.generateKey()
    }

    fun encrypt(plain: String): String {
        val c = Cipher.getInstance("AES/GCM/NoPadding").apply { init(Cipher.ENCRYPT_MODE, key()) }
        val ct = c.doFinal(plain.toByteArray())
        return Base64.encodeToString(c.iv + ct, Base64.NO_WRAP)
    }

    /** Returns null if the data cannot be decrypted (e.g. the key was wiped). */
    fun decrypt(enc: String): String? = runCatching {
        val raw = Base64.decode(enc, Base64.NO_WRAP)
        val c = Cipher.getInstance("AES/GCM/NoPadding")
        c.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(GCM_TAG_BITS, raw, 0, 12))
        String(c.doFinal(raw, 12, raw.size - 12))
    }.getOrNull()
}
