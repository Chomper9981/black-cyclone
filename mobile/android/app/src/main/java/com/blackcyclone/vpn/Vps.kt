package com.blackcyclone.vpn

import org.json.JSONArray
import org.json.JSONObject

/**
 * A VPS managed over SSH, kept as the vps.Host JSON the Go core reads
 * (addr, port, user, password, key_pem, host_key, ...). Holds secrets: only
 * stored encrypted (Prefs.vpsHosts), never shown.
 */
class VpsHost(val json: JSONObject) {
    val id: String get() = json.optString("id")
    val name: String get() = json.optString("name").ifEmpty { addr }
    val addr: String get() = json.optString("addr")
    val port: Int get() = json.optInt("port", 22)
    val user: String get() = json.optString("user", "root")
    val usesKey: Boolean get() = json.optString("key_pem").isNotEmpty()
    val hasPassword: Boolean get() = json.optString("password").isNotEmpty()
    val hostKey: String get() = json.optString("host_key")

    val address: String get() = (if (addr.contains(':')) "[$addr]" else addr) + ":$port"

    /** Copy with the fields of the edit form; blank password / key keep the saved ones. */
    fun edited(name: String, addr: String, port: Int, user: String, password: String, keyPem: String?): VpsHost {
        val o = JSONObject(json.toString())
            .put("name", name.trim())
            .put("addr", addr.trim())
            .put("port", port)
            .put("user", user.trim().ifEmpty { "root" })
        if (password.isNotEmpty()) o.put("password", password)
        when {
            keyPem == null -> o.remove("key_pem") // password login
            keyPem.isNotEmpty() -> o.put("key_pem", keyPem)
        }
        if (addr.trim() != this.addr || port != this.port) o.remove("host_key") // another machine: pin again
        return VpsHost(o)
    }

    companion object {
        fun new(id: String, name: String, addr: String, port: Int, user: String, password: String, keyPem: String?): VpsHost =
            VpsHost(JSONObject().put("id", id)).edited(name, addr, port, user, password, keyPem)

        fun listFromJson(json: String?): List<VpsHost> = runCatching {
            val a = JSONArray(json ?: return emptyList())
            (0 until a.length()).map { VpsHost(a.getJSONObject(it)) }
        }.getOrDefault(emptyList())

        fun listToJson(list: List<VpsHost>): String = JSONArray().apply { list.forEach { put(it.json) } }.toString()
    }
}

/** One user of the server (vps.User). */
data class VpsUser(
    val name: String,
    val link: String,
    val up: Long,
    val down: Long,
    val conns: Int,
    val ips: List<String>,
)

/** Server state on a VPS (vps.Overview). */
data class VpsOverview(
    val installed: Boolean,
    val active: String,
    val host: String,
    val port: Int,
    val sni: String,
    val version: String,
    val scriptOutdated: Boolean,
    val stats: Boolean,
    val online: Boolean,
    val users: List<VpsUser>,
) {
    val address: String get() = (if (host.contains(':')) "[$host]" else host) + ":$port"

    companion object {
        fun parse(json: String): VpsOverview {
            val o = JSONObject(json)
            val users = o.optJSONArray("users")?.let { a ->
                (0 until a.length()).map { i ->
                    val u = a.getJSONObject(i)
                    VpsUser(
                        u.optString("name"), u.optString("link"), u.optLong("up"), u.optLong("down"), u.optInt("conns"),
                        u.optJSONArray("ips")?.let { ips -> (0 until ips.length()).map { ips.getString(it) } }.orEmpty(),
                    )
                }
            }.orEmpty()
            return VpsOverview(
                o.optBoolean("installed"), o.optString("active"), o.optString("host"), o.optInt("port"),
                o.optString("sni"), o.optString("version"), o.optBoolean("script_outdated"),
                o.optBoolean("stats"), o.optBoolean("online"), users,
            )
        }
    }
}
