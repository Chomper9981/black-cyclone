package com.blackcyclone.vpn

import org.json.JSONArray
import org.json.JSONObject

/** Connection mode, same values as settings.Mode of the desktop app. */
enum class Mode(val id: String) {
    WARP("warp"),
    CUSTOM("custom");

    /** Display name, in the current language. */
    val label: String
        get() = when (this) {
            WARP -> "WARP"
            CUSTOM -> tr("Server riêng", "Own server")
        }

    companion object {
        fun of(id: String?): Mode = entries.firstOrNull { it.id == id } ?: WARP
    }
}

/** One server of the "Server riêng" mode, given as a share link (vless://, trojan://, …). */
data class ServerEntry(val id: String, val name: String, val link: String) {
    fun toJson(): JSONObject = JSONObject().put("id", id).put("name", name).put("link", link)

    /** "host:port" from the link, for display. */
    val address: String
        get() = runCatching {
            val rest = link.substringAfter("://").substringAfter('@').substringBefore('?').substringBefore('#').substringBefore('/')
            rest
        }.getOrDefault("")

    val scheme: String get() = link.substringBefore("://").lowercase()

    companion object {
        const val AUTO = "auto"

        fun listFromJson(json: String?): List<ServerEntry> = runCatching {
            val a = JSONArray(json ?: return emptyList())
            (0 until a.length()).map {
                val o = a.getJSONObject(it)
                ServerEntry(o.getString("id"), o.optString("name"), o.getString("link"))
            }
        }.getOrDefault(emptyList())

        fun listToJson(list: List<ServerEntry>): String = JSONArray().apply { list.forEach { put(it.toJson()) } }.toString()
    }
}
